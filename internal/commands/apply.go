package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/storage"
	"github.com/mikecsmith/ihj/internal/ui"
)

// Apply reads an exported file, validates it, and applies changes to Jira.
func Apply(app *App, inputFile string) error {
	app.UI.Status("Reading import file...")
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var payload core.Manifest
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parsing import payload: %w", err)
	}

	ws, err := app.Config.ResolveWorkspace(payload.Metadata.Target)
	if err != nil {
		return fmt.Errorf("resolving workspace: %w", err)
	}

	jiraCfg := ws.ProviderConfig.(*jira.Config)

	// Dynamic Schema Validation
	app.UI.Status("Validating payload against workspace schema...")

	schema := core.ManifestSchema(ws)

	if _, err := storage.WriteSchema(app.CacheDir, ws.Slug, "manifest", schema); err != nil {
		app.UI.Notify("Warning", fmt.Sprintf("Could not cache manifest schema: %v", err))
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
	app.UI.Notify("Validation", "Schema validation passed.")

	// Create Backup
	app.UI.Status("Creating backup...")
	bakFile := inputFile + ".bak"
	if err := copyFile(inputFile, bakFile); err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", bakFile, err)
	}

	// Load Safety State from Cache Directory
	baseName := filepath.Base(inputFile)
	stateFileName := fmt.Sprintf("apply_%s_%s.state.json", ws.Slug, baseName)
	stateFile := filepath.Join(app.CacheDir, stateFileName)
	state := loadApplyState(stateFile)

	// Process Changes
	processed := make(map[string]bool)
	app.UI.Notify("Apply", fmt.Sprintf("Loaded %d top-level items for target '%s'", len(payload.Items), ws.Name))

	var processErr error
	for _, node := range payload.Items {
		if err := processNode(app, ws, jiraCfg, node, "", state, stateFile, processed); err != nil {
			if IsCancelled(err) {
				app.UI.Notify("Cancelled", "Apply cancelled by user.")
			} else {
				processErr = err
			}
			break
		}
	}

	// In-Situ Write Back
	app.UI.Status("Writing IDs back to original file...")
	if writeErr := writeInSitu(inputFile, &payload); writeErr != nil {
		app.UI.Notify("Warning", fmt.Sprintf("Failed to write updated IDs back to %s: %v", inputFile, writeErr))
	} else {
		app.UI.Notify("Success", fmt.Sprintf("Updated %s with new issue IDs.", inputFile))
	}

	if processErr != nil {
		return processErr
	}

	app.UI.Notify("Apply Complete", "All changes have been processed.")

	if rmErr := os.Remove(stateFile); rmErr != nil && !os.IsNotExist(rmErr) {
		app.UI.Notify("Warning", fmt.Sprintf("Failed to clean up state file: %v", rmErr))
	}
	return nil
}

func processNode(app *App, ws *core.Workspace, jiraCfg *jira.Config, node *core.WorkItem, parentID string, state map[string]string, stateFile string, processed map[string]bool) error {
	if node.ID != "" && processed[node.ID] {
		app.UI.Notify("Warning", fmt.Sprintf("Skipping duplicate entry for %s (already processed in this run)", node.ID))
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

		choice, err := app.UI.Select(title, []string{"Create", "Skip", "Abort Apply"})
		if err != nil {
			return err
		}
		if choice < 0 || choice == 2 {
			return &CancelledError{Operation: "apply"}
		}

		if choice == 0 { // Create
			app.UI.Status(fmt.Sprintf("Creating %s...", node.Summary))
			id, err := createIssue(app, ws, jiraCfg, node, parentID)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}
			effectiveID = id
			node.ID = id
			app.UI.Notify("Created", effectiveID)

			state[nodeHash] = effectiveID
			saveApplyState(app.UI, stateFile, state)
		} else {
			app.UI.Status("Skipped creation.")
			return nil
		}

	} else {
		app.UI.Status(fmt.Sprintf("Fetching %s...", node.ID))
		current, err := app.Client.FetchIssue(node.ID)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", node.ID, err)
		}

		diffs := computeDiff(current, node, parentID)
		if len(diffs) == 0 {
			app.UI.Status(fmt.Sprintf("Skipping %s (No changes)", node.ID))
		} else {
			title := fmt.Sprintf("[UPDATE] %s", node.ID)

			options := []string{"Apply to Jira", "Accept Remote (Update Local)", "Skip", "Abort Apply"}
			choice, err := app.UI.ReviewDiff(title, diffs, options)
			if err != nil {
				return err
			}

			if choice < 0 || choice == 3 {
				return &CancelledError{Operation: "apply"}
			}

			switch choice {
			case 0: // Apply Changes to Jira
				app.UI.Status(fmt.Sprintf("Updating %s...", node.ID))
				if err := updateIssue(app, ws, node, current, parentID, diffs); err != nil {
					return fmt.Errorf("updating %s: %w", node.ID, err)
				}
				app.UI.Notify("Updated", node.ID)

			case 1: // Accept Remote (Update Local)
				app.UI.Status(fmt.Sprintf("Accepting remote changes for %s...", node.ID))
				node.Summary = current.Fields.Summary
				node.Type = current.Fields.IssueType.Name
				node.Status = current.Fields.Status.Name

				if len(current.Fields.Description) > 0 && string(current.Fields.Description) != "null" {
					if ast, err := jira.ParseADF(current.Fields.Description); err == nil {
						node.Description = ast
					}
				} else {
					node.Description = nil
				}
				app.UI.Notify("Updated Local YAML", node.ID)

			case 2: // Skip
				app.UI.Status(fmt.Sprintf("Skipped update for %s.", node.ID))
			}
		}
	}

	if node.ID != "" {
		processed[node.ID] = true
	}

	for _, child := range node.Children {
		if err := processNode(app, ws, jiraCfg, child, effectiveID, state, stateFile, processed); err != nil {
			return err
		}
	}

	return nil
}

func createIssue(app *App, ws *core.Workspace, jiraCfg *jira.Config, node *core.WorkItem, parentID string) (string, error) {
	fm := map[string]string{
		"summary": node.Summary,
		"type":    node.Type,
	}
	if parentID != "" {
		fm["parent"] = parentID
	}

	var adfDesc map[string]any
	if node.Description != nil {
		adfDesc = jira.RenderADFValue(node.Description)
	}

	payload := jira.BuildUpsertPayload(fm, adfDesc, ws.Types, jiraCfg.CustomFields, jiraCfg.ProjectKey, jiraCfg.TeamUUID)

	created, err := app.Client.CreateIssue(payload)
	if err != nil {
		return "", err
	}

	if node.Status != "" {
		if tErr := jira.PerformTransition(app.Client, created.Key, node.Status); tErr != nil {
			app.UI.Notify("Warning", fmt.Sprintf("Created %s, but failed to transition status to %s: %v", created.Key, node.Status, tErr))
		}
	}

	return created.Key, nil
}

func updateIssue(app *App, ws *core.Workspace, node *core.WorkItem, current *jira.Issue, parentID string, diffs []ui.Change) error {
	fields := make(map[string]any)
	needsFieldUpdate := false

	for _, d := range diffs {
		if !strings.EqualFold(d.Field, "Status") {
			needsFieldUpdate = true
			break
		}
	}

	if needsFieldUpdate {
		if !strings.EqualFold(current.Fields.IssueType.Name, node.Type) {
			typeFields := make(map[string]any)
			for _, t := range ws.Types {
				if strings.EqualFold(t.Name, node.Type) {
					typeFields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
					break
				}
			}
			if len(typeFields) > 0 {
				if err := app.Client.UpdateIssue(node.ID, map[string]any{"fields": typeFields}); err != nil {
					return fmt.Errorf("updating issue type: %w", err)
				}
			}
		}

		if current.Fields.Summary != node.Summary {
			fields["summary"] = node.Summary
		}

		currentParent := ""
		if current.Fields.Parent != nil {
			currentParent = current.Fields.Parent.Key
		}
		if parentID != "" && currentParent != parentID {
			fields["parent"] = map[string]any{"key": parentID}
		}

		if node.Description != nil {
			fields["description"] = jira.RenderADFValue(node.Description)
		}

		if len(fields) > 0 {
			if err := app.Client.UpdateIssue(node.ID, map[string]any{"fields": fields}); err != nil {
				return fmt.Errorf("updating fields: %w", err)
			}
		}
	}

	if !strings.EqualFold(current.Fields.Status.Name, node.Status) {
		if err := jira.PerformTransition(app.Client, node.ID, node.Status); err != nil {
			return fmt.Errorf("transitioning status: %w", err)
		}
	}

	return nil
}

func computeDiff(current *jira.Issue, target *core.WorkItem, parentID string) []ui.Change {
	var diffs []ui.Change

	if current.Fields.Summary != target.Summary {
		diffs = append(diffs, ui.Change{Field: "Summary", Old: current.Fields.Summary, New: target.Summary})
	}
	if !strings.EqualFold(current.Fields.IssueType.Name, target.Type) {
		diffs = append(diffs, ui.Change{Field: "Type", Old: current.Fields.IssueType.Name, New: target.Type})
	}
	if !strings.EqualFold(current.Fields.Status.Name, target.Status) {
		diffs = append(diffs, ui.Change{Field: "Status", Old: current.Fields.Status.Name, New: target.Status})
	}

	currentParent := ""
	if current.Fields.Parent != nil {
		currentParent = current.Fields.Parent.Key
	}
	if parentID != "" && currentParent != parentID {
		diffs = append(diffs, ui.Change{Field: "Parent", Old: currentParent, New: parentID})
	}
	currentMD := ""
	if len(current.Fields.Description) > 0 && string(current.Fields.Description) != "null" {
		if ast, err := jira.ParseADF(current.Fields.Description); err == nil {
			currentMD = strings.TrimSpace(document.RenderMarkdown(ast))
		}
	}

	normTargetMD := target.DescriptionMarkdown()

	if currentMD != normTargetMD {
		diffs = append(diffs, ui.Change{Field: "Description", Old: currentMD, New: normTargetMD})
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

func saveApplyState(ui ui.UI, path string, state map[string]string) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		ui.Notify("Warning", fmt.Sprintf("Failed to encode apply state: %v", err))
		return
	}

	if wErr := os.WriteFile(path, data, 0o600); wErr != nil {
		ui.Notify("Warning", fmt.Sprintf("Failed to save apply state to disk: %v", wErr))
	}
}
