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
func Apply(rt *Runtime, factory WorkspaceSessionFactory, inputFile string) error {
	rt.UI.Status("Reading import file...")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	// First pass: extract workspace slug from metadata to create session.
	var rawMeta struct {
		Metadata struct {
			Workspace string `yaml:"workspace"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(data, &rawMeta); err != nil {
		return fmt.Errorf("parsing import metadata: %w", err)
	}

	ws, err := rt.ResolveWorkspace(rawMeta.Metadata.Workspace)
	if err != nil {
		return fmt.Errorf("resolving workspace: %w", err)
	}

	wsSess, err := factory(ws.Slug)
	if err != nil {
		return fmt.Errorf("creating workspace session: %w", err)
	}

	defs := wsSess.Provider.FieldDefinitions()

	// Full decode with field-def routing.
	payload, err := core.DecodeManifest(data, defs)
	if err != nil {
		return fmt.Errorf("decoding manifest: %w", err)
	}

	// Dynamic Schema Validation
	rt.UI.Status("Validating payload against workspace schema...")

	schema := core.ManifestSchema(ws, defs)

	if _, err := writeSchema(rt.CacheDir, ws.Slug, "manifest", schema); err != nil {
		rt.UI.Notify("Warning", fmt.Sprintf("Could not cache manifest schema: %v", err))
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolving workspace schema: %w", err)
	}

	var rawData map[string]any
	if err := yaml.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("re-parsing for validation: %w", err)
	}

	if err := resolved.Validate(rawData); err != nil {
		return fmt.Errorf("validation failed (check types/statuses in your file):\n%w", err)
	}
	rt.UI.Notify("Validation", "Schema validation passed.")

	// Create Backup
	rt.UI.Status("Creating backup...")
	bakFile := inputFile + ".bak"
	if err := copyFile(inputFile, bakFile); err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", bakFile, err)
	}

	// Load Safety State from Cache Directory
	baseName := filepath.Base(inputFile)
	stateFileName := fmt.Sprintf("apply_%s_%s.state.json", ws.Slug, baseName)
	stateFile := filepath.Join(rt.CacheDir, stateFileName)
	state := loadApplyState(stateFile)

	// Process Changes
	processed := make(map[string]bool)
	rt.UI.Notify("Apply", fmt.Sprintf("Loaded %d top-level items for workspace '%s'", len(payload.Items), ws.Name))

	var processErr error
	for _, node := range payload.Items {
		if err := processNode(wsSess, node, "", state, stateFile, processed, defs); err != nil {
			if IsCancelled(err) {
				rt.UI.Notify("Cancelled", "Apply cancelled by user.")
			} else {
				processErr = err
			}
			break
		}
	}

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

	rt.UI.Notify("Apply Complete", "All changes have been processed.")

	if rmErr := os.Remove(stateFile); rmErr != nil && !os.IsNotExist(rmErr) {
		rt.UI.Notify("Warning", fmt.Sprintf("Failed to clean up state file: %v", rmErr))
	}
	return nil
}

func processNode(ws *WorkspaceSession, node *core.WorkItem, parentID string, state map[string]string, stateFile string, processed map[string]bool, defs []core.FieldDef) error {
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
			id, err := applyCreate(ws, node, parentID)
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
		current, err := ws.Provider.Get(context.TODO(), node.ID)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", node.ID, err)
		}

		diffs := computeDiff(current, node, parentID, defs)
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
				if err := applyUpdate(ws, node, parentID, diffs, defs); err != nil {
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
		if err := processNode(ws, child, effectiveID, state, stateFile, processed, defs); err != nil {
			return err
		}
	}

	return nil
}

func applyCreate(ws *WorkspaceSession, node *core.WorkItem, parentID string) (string, error) {
	// Shallow copy so we can set parent without mutating the manifest node.
	item := *node
	item.ParentID = parentID
	item.Children = nil // Don't send children to the provider.

	id, err := ws.Provider.Create(context.TODO(), &item)
	if err != nil {
		return "", err
	}

	// Transition to target status if needed (most providers create in a default status).
	if node.Status != "" {
		if tErr := ws.Provider.Update(context.TODO(), id, &core.Changes{Status: &node.Status}); tErr != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but failed to set status to %s: %v", id, node.Status, tErr))
		}
	}

	return id, nil
}

func applyUpdate(ws *WorkspaceSession, node *core.WorkItem, parentID string, diffs []FieldDiff, defs []core.FieldDef) error {
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
	return ws.Provider.Update(context.TODO(), node.ID, changes)
}

func computeDiff(current, target *core.WorkItem, parentID string, defs []core.FieldDef) []FieldDiff {
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

		if fieldValuesEqual(curVal, tgtVal, def.Type) {
			continue
		}
		diffs = append(diffs, FieldDiff{
			Field: def.Label,
			Old:   fmt.Sprintf("%v", curVal),
			New:   fmt.Sprintf("%v", tgtVal),
		})
	}

	return diffs
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
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
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
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		notifier.Notify("Warning", fmt.Sprintf("Failed to encode apply state: %v", err))
		return
	}

	if wErr := os.WriteFile(path, data, 0o600); wErr != nil {
		notifier.Notify("Warning", fmt.Sprintf("Failed to save apply state to disk: %v", wErr))
	}
}
