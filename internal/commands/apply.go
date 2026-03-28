package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
)

// Apply reads an exported file, validates it, and applies changes to the backend.
func Apply(s *Session, inputFile string) error {
	s.UI.Status("Reading import file...")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var payload core.Manifest
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parsing import payload: %w", err)
	}

	ws, err := s.ResolveWorkspace(payload.Metadata.Target)
	if err != nil {
		return fmt.Errorf("resolving workspace: %w", err)
	}

	// Dynamic Schema Validation
	s.UI.Status("Validating payload against workspace schema...")

	schema := core.ManifestSchema(ws)

	if _, err := writeSchema(s.CacheDir, ws.Slug, "manifest", schema); err != nil {
		s.UI.Notify("Warning", fmt.Sprintf("Could not cache manifest schema: %v", err))
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
	s.UI.Notify("Validation", "Schema validation passed.")

	// Create Backup
	s.UI.Status("Creating backup...")
	bakFile := inputFile + ".bak"
	if err := copyFile(inputFile, bakFile); err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", bakFile, err)
	}

	// Load Safety State from Cache Directory
	baseName := filepath.Base(inputFile)
	stateFileName := fmt.Sprintf("apply_%s_%s.state.json", ws.Slug, baseName)
	stateFile := filepath.Join(s.CacheDir, stateFileName)
	state := loadApplyState(stateFile)

	// Process Changes
	processed := make(map[string]bool)
	s.UI.Notify("Apply", fmt.Sprintf("Loaded %d top-level items for target '%s'", len(payload.Items), ws.Name))

	var processErr error
	for _, node := range payload.Items {
		if err := processNode(s, ws, node, "", state, stateFile, processed); err != nil {
			if IsCancelled(err) {
				s.UI.Notify("Cancelled", "Apply cancelled by user.")
			} else {
				processErr = err
			}
			break
		}
	}

	// In-Situ Write Back
	s.UI.Status("Writing IDs back to original file...")
	if writeErr := writeInSitu(inputFile, &payload); writeErr != nil {
		s.UI.Notify("Warning", fmt.Sprintf("Failed to write updated IDs back to %s: %v", inputFile, writeErr))
	} else {
		s.UI.Notify("Success", fmt.Sprintf("Updated %s with new issue IDs.", inputFile))
	}

	if processErr != nil {
		return processErr
	}

	s.UI.Notify("Apply Complete", "All changes have been processed.")

	if rmErr := os.Remove(stateFile); rmErr != nil && !os.IsNotExist(rmErr) {
		s.UI.Notify("Warning", fmt.Sprintf("Failed to clean up state file: %v", rmErr))
	}
	return nil
}

func processNode(s *Session, ws *core.Workspace, node *core.WorkItem, parentID string, state map[string]string, stateFile string, processed map[string]bool) error {
	if node.ID != "" && processed[node.ID] {
		s.UI.Notify("Warning", fmt.Sprintf("Skipping duplicate entry for %s (already processed in this run)", node.ID))
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

		choice, err := s.UI.Select(title, []string{"Create", "Skip", "Abort Apply"})
		if err != nil {
			return err
		}
		if choice < 0 || choice == 2 {
			return &CancelledError{Operation: "apply"}
		}

		if choice == 0 { // Create
			s.UI.Status(fmt.Sprintf("Creating %s...", node.Summary))
			id, err := applyCreate(s, node, parentID)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}
			effectiveID = id
			node.ID = id
			s.UI.Notify("Created", effectiveID)

			state[nodeHash] = effectiveID
			saveApplyState(s.UI, stateFile, state)
		} else {
			s.UI.Status("Skipped creation.")
			return nil
		}

	} else {
		s.UI.Status(fmt.Sprintf("Fetching %s...", node.ID))
		current, err := s.Provider.Get(context.TODO(), node.ID)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", node.ID, err)
		}

		diffs := computeDiff(current, node, parentID)
		if len(diffs) == 0 {
			s.UI.Status(fmt.Sprintf("Skipping %s (No changes)", node.ID))
		} else {
			title := fmt.Sprintf("[UPDATE] %s", node.ID)

			options := []string{"Apply Changes", "Accept Remote (Update Local)", "Skip", "Abort Apply"}
			choice, err := s.UI.ReviewDiff(title, diffs, options)
			if err != nil {
				return err
			}

			if choice < 0 || choice == 3 {
				return &CancelledError{Operation: "apply"}
			}

			switch choice {
			case 0: // Apply Changes
				s.UI.Status(fmt.Sprintf("Updating %s...", node.ID))
				if err := applyUpdate(s, node, parentID, diffs); err != nil {
					return fmt.Errorf("updating %s: %w", node.ID, err)
				}
				s.UI.Notify("Updated", node.ID)

			case 1: // Accept Remote (Update Local)
				s.UI.Status(fmt.Sprintf("Accepting remote changes for %s...", node.ID))
				node.Summary = current.Summary
				node.Type = current.Type
				node.Status = current.Status
				node.Description = current.Description
				s.UI.Notify("Updated Local YAML", node.ID)

			case 2: // Skip
				s.UI.Status(fmt.Sprintf("Skipped update for %s.", node.ID))
			}
		}
	}

	if node.ID != "" {
		processed[node.ID] = true
	}

	for _, child := range node.Children {
		if err := processNode(s, ws, child, effectiveID, state, stateFile, processed); err != nil {
			return err
		}
	}

	return nil
}

func applyCreate(s *Session, node *core.WorkItem, parentID string) (string, error) {
	// Shallow copy so we can set parent without mutating the manifest node.
	item := *node
	item.ParentID = parentID
	item.Children = nil // Don't send children to the provider.

	id, err := s.Provider.Create(context.TODO(), &item)
	if err != nil {
		return "", err
	}

	// Transition to target status if needed (most providers create in a default status).
	if node.Status != "" {
		if tErr := s.Provider.Update(context.TODO(), id, &core.Changes{Status: &node.Status}); tErr != nil {
			s.UI.Notify("Warning", fmt.Sprintf("Created %s, but failed to set status to %s: %v", id, node.Status, tErr))
		}
	}

	return id, nil
}

func applyUpdate(s *Session, node *core.WorkItem, parentID string, diffs []FieldDiff) error {
	changes := &core.Changes{}
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
		}
	}
	return s.Provider.Update(context.TODO(), node.ID, changes)
}

func computeDiff(current, target *core.WorkItem, parentID string) []FieldDiff {
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

	return diffs
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

func writeInSitu(path string, payload *core.Manifest) error {
	var data []byte
	var err error

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf, yaml.UseLiteralStyleIfMultiline(true))
		if err = enc.Encode(payload); err == nil {
			data = buf.Bytes()
		}
	} else {
		data, err = json.MarshalIndent(payload, "", "  ")
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
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
