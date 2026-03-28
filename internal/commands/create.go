package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Create opens an editor for a new work item, then persists it through
// the provider. Fully provider-agnostic.
func Create(ws *WorkspaceSession, overrides map[string]string) error {
	typeNames := typeNames(ws.Workspace)
	selectedType := ""
	if overrides != nil {
		selectedType = overrides["type"]
	}
	if selectedType == "" {
		choice, err := ws.Runtime.UI.Select("Create New Issue", typeNames)
		if err != nil {
			return err
		}
		if choice < 0 {
			return &CancelledError{Operation: "create"}
		}
		selectedType = typeNames[choice]
	}

	_, _, _, _, origStatus, initialDoc, cursorLine, searchPat, err := PrepareCreate(ws, selectedType, overrides)
	if err != nil {
		return err
	}

	edited, err := ws.Runtime.UI.EditText(initialDoc, "ihj_", cursorLine, searchPat)
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}
	if strings.TrimSpace(edited) == strings.TrimSpace(initialDoc) {
		return &CancelledError{Operation: "create"}
	}

	for {
		issueKey, fm, recoverableMsg, submitErr := SubmitCreate(ws, edited)
		if recoverableMsg != "" {
			retry, err := offerRecovery(ws, edited, recoverableMsg)
			if err != nil || retry == "" {
				if submitErr != nil {
					return submitErr
				}
				return &CancelledError{Operation: "create"}
			}
			edited = retry
			continue
		}
		if submitErr != nil {
			return submitErr
		}

		ws.Runtime.UI.Notify("Created", issueKey)

		// Post-create: transition to target status if different from default.
		postCreateActions(ws, fm, issueKey, origStatus)
		return nil
	}
}

// PrepareCreate builds metadata for create mode and returns an editor document.
// Used by the TUI for async create flow.
func PrepareCreate(ws *WorkspaceSession, selectedType string, overrides map[string]string) (
	workspace *core.Workspace, schemaPath string,
	metadata map[string]string, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	workspace = ws.Workspace

	schemaPath, err = writeEditorSchema(ws)
	if err != nil {
		return
	}

	metadata, bodyText, origStatus = buildCreateMetadata(workspace, selectedType, overrides)

	initialDoc = core.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat = calculateCursor(initialDoc, metadata["summary"])
	return
}

// SubmitCreate parses, validates, and submits a new work item.
// Returns the created issue key, parsed frontmatter, a recoverable error
// message (if any), or a hard error.
func SubmitCreate(ws *WorkspaceSession, edited string) (
	issueKey string, fm map[string]string, recoverableMsg string, err error,
) {
	var mdBody string
	fm, mdBody, err = core.ParseFrontmatter(edited)
	if err != nil {
		recoverableMsg = fmt.Sprintf("YAML error: %v", err)
		err = nil
		return
	}

	if errMsg := core.ValidateFrontmatter(fm); errMsg != "" {
		recoverableMsg = errMsg
		return
	}

	ast, astErr := document.ParseMarkdownString(mdBody)
	if astErr != nil {
		err = fmt.Errorf("parsing description: %w", astErr)
		return
	}

	item := frontmatterToWorkItem(fm, ast)
	issueKey, createErr := ws.Provider.Create(context.TODO(), item)
	if createErr != nil {
		recoverableMsg = fmt.Sprintf("API rejected create: %v", createErr)
		return
	}

	return
}

// postCreateActions handles status transition and sprint after creation.
func postCreateActions(ws *WorkspaceSession, fm map[string]string, issueKey, origStatus string) {
	// Transition to target status if it differs from the default.
	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		if err := ws.Provider.Update(context.TODO(), issueKey, &core.Changes{Status: &newStatus}); err != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but could not transition to '%s': %v", issueKey, newStatus, err))
		} else {
			ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
		}
	}

	// Sprint assignment via provider.
	if strings.EqualFold(fm["sprint"], "true") {
		if err := ws.Provider.Update(context.TODO(), issueKey, &core.Changes{
			Fields: map[string]any{"sprint": true},
		}); err != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Could not assign %s to sprint: %v", issueKey, err))
		}
	}
}

// buildCreateMetadata populates default metadata for a new issue.
func buildCreateMetadata(ws *core.Workspace, selectedType string, overrides map[string]string) (
	metadata map[string]string, bodyText, origStatus string,
) {
	origStatus = "Backlog"
	metadata = map[string]string{
		"type":   selectedType,
		"status": first(override(overrides, "status"), origStatus),
	}
	if p := override(overrides, "priority"); p != "" {
		metadata["priority"] = p
	} else {
		metadata["priority"] = "Medium"
	}
	if p := override(overrides, "parent"); p != "" {
		metadata["parent"] = p
	}
	if s := override(overrides, "summary"); s != "" {
		metadata["summary"] = s
	}
	if strings.EqualFold(override(overrides, "sprint"), "true") {
		metadata["sprint"] = "true"
	}

	for _, t := range ws.Types {
		if t.Name == selectedType && t.Template != "" {
			bodyText = strings.TrimSpace(t.Template)
			break
		}
	}
	return
}

// override safely reads from a potentially nil overrides map.
func override(overrides map[string]string, key string) string {
	if overrides == nil {
		return ""
	}
	return overrides[key]
}
