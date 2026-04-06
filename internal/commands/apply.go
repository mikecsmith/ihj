package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/encoding"
)

// Apply reads an exported file, validates it, and applies changes to the backend.
// If workspaceOverride is non-empty it takes precedence over the manifest's metadata.workspace.
func Apply(ctx context.Context, rt *Runtime, factory WorkspaceSessionFactory, inputFile, workspaceOverride string) error {
	rt.UI.Status("Reading import file...")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	wsSess, payload, defs, err := applyPrepare(rt, factory, data, workspaceOverride)
	if err != nil {
		return err
	}

	// Create Backup
	rt.UI.Status("Creating backup...")
	bakFile := inputFile + ".bak"
	if err := copyFile(inputFile, bakFile); err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", bakFile, err)
	}

	// Load Safety State from Cache Directory
	baseName := filepath.Base(inputFile)
	stateFileName := fmt.Sprintf("%s.state.apply.%s.%s.json", wsSess.Workspace.Provider, wsSess.Workspace.Slug, baseName)
	stateFile := filepath.Join(rt.CacheDir, stateFileName)
	state := loadApplyState(stateFile)

	processErr := applyProcess(ctx, rt, wsSess, payload, defs, state, stateFile)

	// In-Situ Write Back
	rt.UI.Status("Writing IDs back to original file...")
	if writeErr := writeInSitu(inputFile, payload, defs); writeErr != nil {
		rt.UI.Notify("Warning", fmt.Sprintf("Failed to write updated IDs back to %s: %v", inputFile, writeErr))
	} else {
		rt.UI.Notify("Success", fmt.Sprintf("Updated %s with new issue IDs.", inputFile))
	}

	if processErr != nil {
		return processErr
	}

	if rmErr := os.Remove(stateFile); rmErr != nil && !os.IsNotExist(rmErr) {
		rt.UI.Notify("Warning", fmt.Sprintf("Failed to clean up state file: %v", rmErr))
	}
	return nil
}

// ApplyContent applies manifest YAML from memory (desktop use case).
// It performs the same validation and per-item review loop as Apply but
// skips file backup, state tracking, and in-situ write-back.
func ApplyContent(ctx context.Context, rt *Runtime, factory WorkspaceSessionFactory, data []byte) error {
	wsSess, payload, defs, err := applyPrepare(rt, factory, data, "")
	if err != nil {
		return err
	}

	// No state file for in-memory applies — creates are not idempotent.
	state := make(map[string]string)
	return applyProcess(ctx, rt, wsSess, payload, defs, state, "")
}

// applyPrepare handles workspace resolution, manifest decoding, and schema
// validation — shared by Apply and ApplyContent. If workspaceOverride is
// non-empty it takes precedence over the manifest's metadata.workspace.
func applyPrepare(rt *Runtime, factory WorkspaceSessionFactory, data []byte, workspaceOverride string) (*WorkspaceSession, *encoding.Manifest, []core.FieldDef, error) {
	var rawMeta struct {
		Metadata struct {
			Workspace string `yaml:"workspace"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(data, &rawMeta); err != nil {
		return nil, nil, nil, fmt.Errorf("parsing import metadata: %w", err)
	}

	slug := rawMeta.Metadata.Workspace
	if workspaceOverride != "" {
		slug = workspaceOverride
	}
	ws, err := rt.ResolveWorkspace(slug)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving workspace: %w", err)
	}

	wsSess, err := factory(ws.Slug)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating workspace session: %w", err)
	}

	defs := wsSess.Provider.FieldDefinitions()

	payload, err := encoding.DecodeManifest(data, defs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decoding manifest: %w", err)
	}

	rt.UI.Status("Validating payload against workspace schema...")

	schema := encoding.ManifestSchema(ws, defs)
	if _, err := writeSchema(rt.CacheDir, ws.Provider, ws.Slug, "manifest", schema); err != nil {
		rt.UI.Notify("Warning", fmt.Sprintf("Could not cache manifest schema: %v", err))
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving workspace schema: %w", err)
	}

	var rawData map[string]any
	if err := yaml.Unmarshal(data, &rawData); err != nil {
		return nil, nil, nil, fmt.Errorf("re-parsing for validation: %w", err)
	}

	if err := resolved.Validate(rawData); err != nil {
		return nil, nil, nil, fmt.Errorf("validation failed (check types/statuses in your file):\n%w", err)
	}
	rt.UI.Notify("Validation", "Schema validation passed.")

	return wsSess, payload, defs, nil
}

// applyProcess runs the per-item review loop — shared by Apply and ApplyContent.
func applyProcess(ctx context.Context, rt *Runtime, wsSess *WorkspaceSession, payload *encoding.Manifest, defs []core.FieldDef, state map[string]string, stateFile string) error {
	processed := make(map[string]bool)
	rt.UI.Notify("Apply", fmt.Sprintf("Loaded %d top-level items for workspace '%s'", len(payload.Items), wsSess.Workspace.Name))

	for _, node := range payload.Items {
		if err := processNode(ctx, wsSess, node, "", state, stateFile, processed, defs); err != nil {
			if IsCancelled(err) {
				rt.UI.Notify("Cancelled", "Apply cancelled by user.")
				return nil
			}
			return err
		}
	}

	rt.UI.Notify("Apply Complete", "All changes have been processed.")
	return nil
}

func processNode(ctx context.Context, ws *WorkspaceSession, node *core.WorkItem, parentID string, state map[string]string, stateFile string, processed map[string]bool, defs []core.FieldDef) error {
	if node.ID != "" && processed[node.ID] {
		ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Skipping duplicate entry for %s (already processed in this run)", node.ID))
		return nil
	}

	nodeHash := core.ComputeStateHash(node, parentID, defs)
	if node.ID == "" && state[nodeHash] != "" {
		node.ID = state[nodeHash]
	}

	effectiveID := node.ID

	if node.ID == "" {
		title := fmt.Sprintf("[CREATE] %s: %s", node.Type, node.Summary)
		if parentID != "" {
			title += fmt.Sprintf("\n  ↳ Parent: %s", parentID)
		}

		choice, err := ws.Runtime.UI.Select(title, []string{"Create", "Skip", "Abort Apply"})
		if err != nil {
			return err
		}
		if choice < 0 || choice == 2 {
			return &CancelledError{Operation: "apply"}
		}

		if choice == 0 { // Create
			ws.Runtime.UI.Status(fmt.Sprintf("Creating %s...", node.Summary))
			id, err := ApplyCreate(ctx, ws, node, parentID)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}
			effectiveID = id
			node.ID = id
			ws.Runtime.UI.Notify("Created", effectiveID)

			state[nodeHash] = effectiveID
			saveApplyState(ws.Runtime.UI, stateFile, state)
		} else {
			ws.Runtime.UI.Status("Skipped creation.")
			return nil
		}

	} else {
		ws.Runtime.UI.Status(fmt.Sprintf("Fetching %s...", node.ID))
		current, err := ws.Provider.Get(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", node.ID, err)
		}

		changes, diffs, diffErr := diffItem(current, node, parentID, defs)
		if diffErr != nil {
			return fmt.Errorf("diffing %s: %w", node.ID, diffErr)
		}
		if changes == nil {
			ws.Runtime.UI.Status(fmt.Sprintf("Skipping %s (No changes)", node.ID))
		} else {
			title := fmt.Sprintf("[UPDATE] %s", node.ID)

			options := []string{"Apply Changes", "Accept Remote (Update Local)", "Skip", "Abort Apply"}
			choice, err := ws.Runtime.UI.ReviewDiff(title, diffs, options)
			if err != nil {
				return err
			}

			if choice < 0 || choice == 3 {
				return &CancelledError{Operation: "apply"}
			}

			switch choice {
			case 0: // Apply Changes
				ws.Runtime.UI.Status(fmt.Sprintf("Updating %s...", node.ID))
				if err := ws.Provider.Update(ctx, node.ID, changes); err != nil {
					return fmt.Errorf("updating %s: %w", node.ID, err)
				}
				ws.Runtime.UI.Notify("Updated", node.ID)

			case 1: // Accept Remote (Update Local)
				ws.Runtime.UI.Status(fmt.Sprintf("Accepting remote changes for %s...", node.ID))
				node.Summary = current.Summary
				node.Type = current.Type
				node.Status = current.Status
				node.Description = current.Description
				node.Fields = current.Fields
				ws.Runtime.UI.Notify("Updated Local YAML", node.ID)

			case 2: // Skip
				ws.Runtime.UI.Status(fmt.Sprintf("Skipped update for %s.", node.ID))
			}
		}
	}

	if node.ID != "" {
		processed[node.ID] = true
	}

	for _, child := range node.Children {
		if err := processNode(ctx, ws, child, effectiveID, state, stateFile, processed, defs); err != nil {
			return err
		}
	}

	return nil
}

// ApplyCreate creates a new work item from a manifest node, optionally
// linking it to a parent. It also transitions to the target status if set
// and assigns to the active sprint when sprint is true.
func ApplyCreate(ctx context.Context, ws *WorkspaceSession, node *core.WorkItem, parentID string) (string, error) {
	// Shallow copy so we can set parent without mutating the manifest node.
	item := *node
	item.ParentID = parentID
	item.Children = nil // Don't send children to the provider.

	id, err := ws.Provider.Create(ctx, &item)
	if err != nil {
		return "", err
	}

	// Post-create fixups: status transition and sprint assignment are
	// handled via Update because providers typically ignore these during
	// initial creation.
	postChanges := &core.Changes{}
	if node.Status != "" {
		postChanges.Status = &node.Status
	}
	// Forward all non-empty fields for post-create processing. Certain
	// fields (like sprint) are handled by providers as post-create operations
	// that don't participate in the initial Create payload.
	if len(node.Fields) > 0 {
		postChanges.Fields = make(map[string]any, len(node.Fields))
		for k, v := range node.Fields {
			postChanges.Fields[k] = v
		}
	}

	if postChanges.Status != nil || postChanges.Fields != nil {
		ws.Runtime.UI.Status(fmt.Sprintf("Applying post-create fields for %s...", id))
		if tErr := ws.Provider.Update(ctx, id, postChanges); tErr != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but post-create update failed: %v", id, tErr))
		}
	}

	return id, nil
}

// diffItem computes the change payload and the corresponding display diffs
// for an item against the remote current state. Returns (nil, nil, nil) when
// there are no changes. The cascade parentID is injected into the target
// before comparison; "parent" is only considered set when parentID is non-
// empty (matching the hierarchical manifest convention).
func diffItem(current, target *core.WorkItem, parentID string, defs []core.FieldDef) (*core.Changes, []FieldDiff, error) {
	edited := *target
	edited.ParentID = parentID

	var set core.SetKeys
	if target.DecodedKeys != nil {
		// Use real presence tracking from decoder.
		set = target.DecodedKeys
		if parentID != "" {
			set[core.KeyParent] = true
		}
	} else {
		set = deriveSetKeys(&edited, parentID, defs)
	}

	changes, err := core.ComputeChanges(current, &edited, set, defs)
	if err != nil || changes == nil {
		return nil, nil, err
	}
	return changes, changesToFieldDiffs(current, changes, defs), nil
}

// deriveSetKeys infers SetKeys from non-zero values when the decoder did not
// track presence (DecodedKeys is nil). This is the fallback path — decoders
// that populate DecodedKeys give callers true clear-intent semantics.
func deriveSetKeys(target *core.WorkItem, parentID string, defs []core.FieldDef) core.SetKeys {
	set := make(core.SetKeys, 8+len(target.Fields))
	if target.Summary != "" {
		set[core.KeySummary] = true
	}
	if target.Type != "" {
		set[core.KeyType] = true
	}
	if target.Status != "" {
		set[core.KeyStatus] = true
	}
	if parentID != "" {
		set[core.KeyParent] = true
	}
	if target.Description != nil {
		set[core.KeyDescription] = true
	}
	for _, def := range defs {
		if v, ok := target.Fields[def.Key]; ok && v != nil {
			set[def.Key] = true
		}
	}
	return set
}

// changesToFieldDiffs renders a Changes payload as display diffs, pairing each
// new value with the corresponding current value for the review UI.
func changesToFieldDiffs(current *core.WorkItem, ch *core.Changes, defs []core.FieldDef) []FieldDiff {
	var diffs []FieldDiff
	if ch.Summary != nil {
		diffs = append(diffs, FieldDiff{Field: "Summary", Old: current.Summary, New: *ch.Summary})
	}
	if ch.Type != nil {
		diffs = append(diffs, FieldDiff{Field: "Type", Old: current.Type, New: *ch.Type})
	}
	if ch.Status != nil {
		diffs = append(diffs, FieldDiff{Field: "Status", Old: current.Status, New: *ch.Status})
	}
	if ch.ParentID != nil {
		diffs = append(diffs, FieldDiff{Field: "Parent", Old: current.ParentID, New: *ch.ParentID})
	}
	if ch.Description != nil {
		diffs = append(diffs, FieldDiff{Field: "Description", Old: current.DescriptionMarkdown(), New: core.RenderRichText(ch.Description)})
	}
	defByKey := make(map[string]core.FieldDef, len(defs))
	for _, def := range defs {
		defByKey[def.Key] = def
	}
	for k, v := range ch.Fields {
		def := defByKey[k]
		old := ""
		if !def.WriteOnly {
			old = fieldToString(current.Fields[k])
		}
		label := def.Label
		if label == "" {
			label = k
		}
		diffs = append(diffs, FieldDiff{Field: label, Old: old, New: fieldToString(v)})
	}
	return diffs
}

// fieldToString converts a field value to its display string,
// returning "" for nil instead of "<nil>".
func fieldToString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// State and File Management Helpers
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := in.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}

func writeInSitu(path string, payload *encoding.Manifest, defs []core.FieldDef) (err error) {
	ext := strings.ToLower(filepath.Ext(path))
	format := "yaml"
	if ext == ".json" {
		format = "json"
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Write back with full=true to preserve all fields that were in the original file.
	return encoding.EncodeManifest(f, payload, defs, true, format)
}

func loadApplyState(path string) map[string]string {
	state := make(map[string]string)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &state) //nolint:errcheck
	}
	return state
}

func saveApplyState(notifier UI, path string, state map[string]string) {
	if path == "" {
		return // In-memory apply — no state file.
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		notifier.Notify("Warning", fmt.Sprintf("Failed to encode apply state: %v", err))
		return
	}

	if wErr := os.WriteFile(path, data, 0o600); wErr != nil {
		notifier.Notify("Warning", fmt.Sprintf("Failed to save apply state to disk: %v", wErr))
	}
}
