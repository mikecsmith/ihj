package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
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
	stateFileName := fmt.Sprintf("apply_%s_%s.state.json", wsSess.Workspace.Slug, baseName)
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
func applyPrepare(rt *Runtime, factory WorkspaceSessionFactory, data []byte, workspaceOverride string) (*WorkspaceSession, *core.Manifest, []core.FieldDef, error) {
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

	payload, err := core.DecodeManifest(data, defs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decoding manifest: %w", err)
	}

	rt.UI.Status("Validating payload against workspace schema...")

	schema := core.ManifestSchema(ws, defs)
	if _, err := writeSchema(rt.CacheDir, ws.Slug, "manifest", schema); err != nil {
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
func applyProcess(ctx context.Context, rt *Runtime, wsSess *WorkspaceSession, payload *core.Manifest, defs []core.FieldDef, state map[string]string, stateFile string) error {
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

	nodeHash := node.StateHash(parentID)
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

		diffs := ComputeDiff(current, node, parentID, defs)
		if len(diffs) == 0 {
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
				if err := ApplyUpdate(ctx, ws, node, parentID, diffs, defs); err != nil {
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
		if tErr := ws.Provider.Update(ctx, id, postChanges); tErr != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but post-create update failed: %v", id, tErr))
		}
	}

	return id, nil
}

// ApplyUpdate sends only the changed fields for an existing work item.
func ApplyUpdate(ctx context.Context, ws *WorkspaceSession, node *core.WorkItem, parentID string, diffs []FieldDiff, defs []core.FieldDef) error {
	changes := &core.Changes{}

	// Build a label→key lookup for field defs so we can match diff labels.
	defByLabel := make(map[string]core.FieldDef, len(defs))
	for _, def := range defs {
		defByLabel[def.Label] = def
	}

	for _, d := range diffs {
		switch d.Field {
		case "Summary":
			changes.Summary = &node.Summary
		case "Type":
			changes.Type = &node.Type
		case "Status":
			changes.Status = &node.Status
		case "Parent":
			changes.ParentID = &parentID
		case "Description":
			changes.Description = node.Description
		default:
			// Field-def-driven fields go into Changes.Fields.
			if def, ok := defByLabel[d.Field]; ok {
				if changes.Fields == nil {
					changes.Fields = make(map[string]any)
				}
				changes.Fields[def.Key] = node.Fields[def.Key]
			}
		}
	}
	return ws.Provider.Update(ctx, node.ID, changes)
}

// ComputeDiff compares a current work item against a target (from a manifest)
// and returns the list of field-level differences.
func ComputeDiff(current, target *core.WorkItem, parentID string, defs []core.FieldDef) []FieldDiff {
	var diffs []FieldDiff

	if current.Summary != target.Summary {
		diffs = append(diffs, FieldDiff{Field: "Summary", Old: current.Summary, New: target.Summary})
	}
	if !strings.EqualFold(current.Type, target.Type) {
		diffs = append(diffs, FieldDiff{Field: "Type", Old: current.Type, New: target.Type})
	}
	if !strings.EqualFold(current.Status, target.Status) {
		diffs = append(diffs, FieldDiff{Field: "Status", Old: current.Status, New: target.Status})
	}

	if parentID != "" && current.ParentID != parentID {
		diffs = append(diffs, FieldDiff{Field: "Parent", Old: current.ParentID, New: parentID})
	}

	currentMD := current.DescriptionMarkdown()
	targetMD := target.DescriptionMarkdown()
	if currentMD != targetMD {
		diffs = append(diffs, FieldDiff{Field: "Description", Old: currentMD, New: targetMD})
	}

	// Diff editable fields driven by field defs.
	for _, def := range defs {
		if def.Visibility == core.FieldReadOnly {
			continue
		}
		curVal := current.Fields[def.Key]
		tgtVal := target.Fields[def.Key]

		// A nil/missing target value means the field wasn't in the manifest
		// (e.g. extended fields omitted without --full). Don't treat as a change.
		if tgtVal == nil {
			continue
		}

		// Normalise "unassigned" / "none" to empty string for user fields,
		// so `assignee: unassigned` in a manifest means "clear this field".
		tgtVal = normaliseUserField(def, tgtVal)

		if fieldValuesEqual(curVal, tgtVal, def.Type) {
			continue
		}
		diffs = append(diffs, FieldDiff{
			Field: def.Label,
			Old:   fieldToString(curVal),
			New:   fieldToString(tgtVal),
		})
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

// normaliseUserField converts "unassigned" or "none" (any casing) to "" for
// FieldAssignee fields, so `assignee: unassigned` in a manifest means "clear this".
func normaliseUserField(def core.FieldDef, val any) any {
	if def.Type != core.FieldAssignee {
		return val
	}
	if s, ok := val.(string); ok {
		lower := strings.ToLower(s)
		if lower == "unassigned" || lower == "none" {
			return ""
		}
	}
	return val
}

// fieldValuesEqual compares two field values based on FieldType.
func fieldValuesEqual(a, b any, ft core.FieldType) bool {
	switch ft {
	case core.FieldStringArray:
		as := toStringSlice(a)
		bs := toStringSlice(b)
		sort.Strings(as)
		sort.Strings(bs)
		return slices.Equal(as, bs)
	case core.FieldBool:
		ab, _ := a.(bool)
		bb, _ := b.(bool)
		return ab == bb
	default: // string, enum
		return fieldToString(a) == fieldToString(b)
	}
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		s := make([]string, len(val))
		for i, item := range val {
			s[i] = fmt.Sprintf("%v", item)
		}
		return s
	case nil:
		return nil
	default:
		return nil
	}
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

func writeInSitu(path string, payload *core.Manifest, defs []core.FieldDef) (err error) {
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
	return core.EncodeManifest(f, payload, defs, true, format)
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
