package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/encoding"
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
		selectedType = overrides[core.KeyType]
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
		issueKey, item, recoverableMsg, submitErr := SubmitCreate(ctx, ws, edited)
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
		PostCreateActions(ctx, ws, item, issueKey, origStatus)
		return nil
	}
}

// PrepareCreate builds a stub work item for create mode and returns an editor document.
// Used by the TUI for async create flow.
func PrepareCreate(ws *WorkspaceSession, selectedType string, overrides map[string]string) (
	workspace *core.Workspace, schemaPath string,
	item *core.WorkItem, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	workspace = ws.Workspace
	defs := ws.Provider.FieldDefinitions()

	schemaPath, err = writeEditorSchema(ws)
	if err != nil {
		return
	}

	item, bodyText, origStatus = buildCreateStub(workspace, selectedType, overrides, defs)

	initialDoc = encoding.BuildFrontmatterDoc(schemaPath, item, defs, bodyText)
	cursorLine, searchPat = terminal.CalculateCursor(initialDoc, item.Summary)
	return
}

// SubmitCreate parses, validates, and submits a new work item.
// Returns the created issue key, parsed work item, a recoverable error
// message (if any), or a hard error.
func SubmitCreate(ctx context.Context, ws *WorkspaceSession, edited string) (
	issueKey string, item *core.WorkItem, recoverableMsg string, err error,
) {
	defs := ws.Provider.FieldDefinitions()
	item, _, err = encoding.ParseFrontmatter(edited, defs)
	if err != nil {
		recoverableMsg = fmt.Sprintf("YAML error: %v", err)
		err = nil
		return
	}

	if errMsg := encoding.ValidateFrontmatter(item); errMsg != "" {
		recoverableMsg = errMsg
		return
	}

	issueKey, createErr := ws.Provider.Create(ctx, item)
	if createErr != nil {
		recoverableMsg = fmt.Sprintf("API rejected create: %v", createErr)
		return
	}

	return
}

// PostCreateActions handles status transition and sprint after creation.
func PostCreateActions(ctx context.Context, ws *WorkspaceSession, item *core.WorkItem, issueKey, origStatus string) {
	// Transition to target status if it differs from the default.
	if item.Status != "" && !strings.EqualFold(item.Status, origStatus) {
		newStatus := item.Status
		if err := ws.Provider.Update(ctx, issueKey, &core.Changes{Status: &newStatus}); err != nil {
			ws.Runtime.UI.Notify("Warning", fmt.Sprintf("Created %s, but could not transition to '%s': %v", issueKey, newStatus, err))
		} else {
			ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
		}
	}

	// Post-create field fixups: certain fields (e.g., sprint) require a
	// separate update call because providers may ignore them during creation.
	postFields := make(map[string]any)
	for k, v := range item.Fields {
		if core.IsZeroFieldValue(v) {
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

// buildCreateStub populates a stub WorkItem for a new issue.
func buildCreateStub(ws *core.Workspace, selectedType string, overrides map[string]string, defs core.FieldDefs) (
	item *core.WorkItem, bodyText, origStatus string,
) {
	// Default to the first configured status (lowest order).
	origStatus = "To Do"
	if len(ws.Statuses) > 0 {
		origStatus = ws.Statuses[0].Name
	}

	item = &core.WorkItem{
		Type:   selectedType,
		Status: first(override(overrides, core.KeyStatus), origStatus),
		Fields: make(map[string]any),
	}

	// Default priority from the primary urgency field's enum (middle value).
	if urgency := defs.ByRole(core.RoleUrgency).Primary(); urgency != nil && len(urgency.Enum) > 0 {
		item.Fields[urgency.Key] = first(override(overrides, urgency.Key), urgency.Enum[len(urgency.Enum)/2])
	}

	// Forward all non-core overrides (parent, summary, sprint, etc.).
	for k, v := range overrides {
		if v == "" || k == core.KeyType || k == core.KeyStatus {
			continue
		}
		switch k {
		case core.KeySummary:
			item.Summary = v
		case core.KeyParent:
			item.ParentID = v
		default:
			item.Fields[k] = v
		}
	}

	// Include required custom fields for the selected type with defaults.
	for _, t := range ws.Types {
		if t.Name == selectedType {
			if t.Template != "" {
				bodyText = strings.TrimSpace(t.Template)
			}
			for _, def := range t.Fields {
				if !def.SeedOnCreate() {
					continue
				}
				if _, hasField := item.Fields[def.Key]; !hasField {
					item.Fields[def.Key] = defaultForField(def)
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
