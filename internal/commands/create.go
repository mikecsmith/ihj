package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// Create opens an editor for a new work item, then persists it through
// the provider. Fully provider-agnostic.
func Create(ctx context.Context, ws *WorkspaceSession, overrides map[string]string) error {
	if err := core.ValidateFieldOverrides(overrides, ws.Provider.FieldDefinitions()); err != nil {
		return err
	}
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

	_, _, _, _, origStatus, initialDoc, _, _, err := PrepareCreate(ws, selectedType, overrides)
	if err != nil {
		return err
	}

	edited, err := ws.Runtime.UI.EditDocument(initialDoc, "ihj_")
	if err != nil {
		return err
	}
	if strings.TrimSpace(edited) == strings.TrimSpace(initialDoc) {
		return &CancelledError{Operation: "create"}
	}

	for {
		issueKey, fm, recoverableMsg, submitErr := SubmitCreate(ctx, ws, edited)
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
		PostCreateActions(ctx, ws, fm, issueKey, origStatus)
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

	metadata, bodyText, origStatus = buildCreateMetadata(workspace, selectedType, overrides, ws.Provider.FieldDefinitions())

	initialDoc = core.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat = terminal.CalculateCursor(initialDoc, metadata["summary"])
	return
}

// SubmitCreate parses, validates, and submits a new work item.
// Returns the created issue key, parsed frontmatter, a recoverable error
// message (if any), or a hard error.
func SubmitCreate(ctx context.Context, ws *WorkspaceSession, edited string) (
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

	item := core.FrontmatterToWorkItem(fm, ast)
	issueKey, createErr := ws.Provider.Create(ctx, item)
	if createErr != nil {
		recoverableMsg = fmt.Sprintf("API rejected create: %v", createErr)
		return
	}

	return
}

// PostCreateActions handles status transition and sprint after creation.
func PostCreateActions(ctx context.Context, ws *WorkspaceSession, fm map[string]string, issueKey, origStatus string) {
	// Transition to target status if it differs from the default.
	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		if err := ws.Provider.Update(ctx, issueKey, &core.Changes{Status: &newStatus}); err != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but could not transition to '%s': %v", issueKey, newStatus, err))
		} else {
			ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
		}
	}

	// Post-create field fixups: certain fields (e.g., sprint) require a
	// separate update call because providers may ignore them during creation.
	postFields := make(map[string]any)
	for k, v := range fm {
		if core.IsCoreKey(k) || v == "" {
			continue
		}
		postFields[k] = v
	}
	if len(postFields) > 0 {
		if err := ws.Provider.Update(ctx, issueKey, &core.Changes{
			Fields: postFields,
		}); err != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but post-create field update failed: %v", issueKey, err))
		}
	}
}

// buildCreateMetadata populates default metadata for a new issue.
func buildCreateMetadata(ws *core.Workspace, selectedType string, overrides map[string]string, defs core.FieldDefs) (
	metadata map[string]string, bodyText, origStatus string,
) {
	// Default to the first configured status (lowest order).
	origStatus = "To Do"
	if len(ws.Statuses) > 0 {
		origStatus = ws.Statuses[0].Name
	}
	metadata = map[string]string{
		"type":   selectedType,
		"status": first(override(overrides, "status"), origStatus),
	}

	// Default priority from the primary urgency field's enum (middle value).
	if urgency := defs.ByRole(core.RoleUrgency).Primary(); urgency != nil && len(urgency.Enum) > 0 {
		metadata[urgency.Key] = first(override(overrides, urgency.Key), urgency.Enum[len(urgency.Enum)/2])
	}

	// Forward all non-core overrides (parent, summary, sprint, etc.).
	for k, v := range overrides {
		if v != "" && k != "type" && k != "status" {
			metadata[k] = v
		}
	}

	// Include required custom fields for the selected type with defaults.
	for _, t := range ws.Types {
		if t.Name == selectedType {
			if t.Template != "" {
				bodyText = strings.TrimSpace(t.Template)
			}
			for _, def := range t.Fields.Required() {
				if _, exists := metadata[def.Key]; !exists {
					metadata[def.Key] = defaultForField(def)
				}
			}
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

// typeNames returns the display names of all configured types.
func typeNames(ws *core.Workspace) []string {
	names := make([]string, len(ws.Types))
	for i, t := range ws.Types {
		names[i] = t.Name
	}
	return names
}

// defaultForField returns a sensible default value for a required field.
// Enums default to the first value; other types default to empty.
func defaultForField(def core.FieldDef) string {
	if def.Type == core.FieldEnum && len(def.Enum) > 0 {
		return def.Enum[0]
	}
	return ""
}

// first returns the first non-empty string from the arguments.
func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
