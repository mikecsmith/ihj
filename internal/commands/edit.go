package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/encoding"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// Edit fetches an existing work item, opens it in the editor, and applies
// changes through the provider. Fully provider-agnostic.
func Edit(ctx context.Context, ws *WorkspaceSession, issueKey string, overrides map[string]string) error {
	// Validate overrides against provider FieldDefs before opening editor.
	if err := core.ValidateFieldOverrides(overrides, ws.Workspace.AllFieldDefs()); err != nil {
		return err
	}

	workspace, _, _, _, origStatus, initialDoc, _, _, err := PrepareEdit(ctx, ws, issueKey, overrides)
	if err != nil {
		return err
	}

	edited, err := ws.Runtime.UI.EditDocument(initialDoc, "ihj_")
	if err != nil {
		return err
	}
	if strings.TrimSpace(edited) == strings.TrimSpace(initialDoc) {
		return &CancelledError{Operation: "edit"}
	}

	for {
		item, recoverableMsg, err := SubmitEdit(ctx, ws, workspace, issueKey, edited, origStatus)
		if err != nil {
			return err
		}
		if recoverableMsg != "" {
			retry, retryErr := offerRecovery(ws, edited, recoverableMsg)
			if retryErr != nil || retry == "" {
				return &CancelledError{Operation: "edit"}
			}
			edited = retry
			continue
		}
		if item == nil {
			ws.Runtime.UI.Notify("No Changes", "Nothing to update.")
			return nil
		}

		ws.Runtime.UI.Notify("Updated", issueKey)
		PostEditNotify(ws, item, issueKey, origStatus)
		return nil
	}
}

// PrepareEdit fetches the issue and builds the editor document.
// Used by the TUI for async edit flow.
func PrepareEdit(ctx context.Context, ws *WorkspaceSession, issueKey string, overrides map[string]string) (
	workspace *core.Workspace, schemaPath string,
	item *core.WorkItem, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	workspace = ws.Workspace

	schemaPath, err = writeEditorSchema(ws)
	if err != nil {
		return
	}

	item, err = ws.Provider.Get(ctx, issueKey)
	if err != nil {
		err = fmt.Errorf("fetching %s: %w", issueKey, err)
		return
	}

	applyItemOverrides(item, overrides)
	origStatus = item.Status
	bodyText = item.DescriptionMarkdown()

	// If the description is empty, pre-populate with the type's template.
	if tc := workspace.TypeByName(item.Type); tc != nil && tc.Template != "" && bodyText == "" {
		bodyText = strings.TrimSpace(tc.Template)
	}

	defs := ws.Workspace.AllFieldDefs()
	initialDoc = encoding.BuildFrontmatterDoc(schemaPath, item, defs, bodyText)
	cursorLine, searchPat = terminal.CalculateCursor(initialDoc, item.Summary)
	return
}

// SubmitEdit parses, validates, and submits an edited document.
// Returns the parsed work item, a recoverable error message (if any),
// or a hard error.
func SubmitEdit(ctx context.Context, ws *WorkspaceSession, workspace *core.Workspace, issueKey, edited, origStatus string) (
	item *core.WorkItem, recoverableMsg string, err error,
) {
	defs := ws.Workspace.AllFieldDefs()
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

	// Fetch current state to compute diff.
	current, fetchErr := ws.Provider.Get(ctx, issueKey)
	if fetchErr != nil {
		err = fmt.Errorf("fetching %s for diff: %w", issueKey, fetchErr)
		return
	}

	changes, diffErr := core.ComputeChanges(current, item, item.Presence, defs)
	if diffErr != nil {
		recoverableMsg = diffErr.Error()
		return
	}
	if changes == nil {
		item = nil // signal no changes
		return
	}

	if updateErr := ws.Provider.Update(ctx, issueKey, changes); updateErr != nil {
		recoverableMsg = fmt.Sprintf("API rejected update: %v", updateErr)
		return
	}

	return
}

// PostEditNotify handles post-edit notifications (sprint info).
// Status transitions are already handled by Provider.Update.
func PostEditNotify(ws *WorkspaceSession, item *core.WorkItem, issueKey, origStatus string) {
	if item.Status != "" && !strings.EqualFold(item.Status, origStatus) {
		ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", item.Status))
	}
}

// writeEditorSchema generates and caches the frontmatter JSON schema.
func writeEditorSchema(ws *WorkspaceSession) (string, error) {
	schemaDict := encoding.FrontmatterSchema(ws.Workspace, ws.Workspace.AllFieldDefs())
	schemaPath, err := writeSchema(ws.Runtime.CacheDir, ws.Workspace.Provider, ws.Workspace.Slug, encoding.Frontmatter, schemaDict)
	if err != nil {
		return "", fmt.Errorf("writing schema: %w", err)
	}
	return schemaPath, nil
}

// applyItemOverrides merges non-empty overrides into a WorkItem.
func applyItemOverrides(item *core.WorkItem, overrides map[string]string) {
	for k, v := range overrides {
		if v == "" {
			continue
		}
		switch k {
		case core.KeySummary:
			item.Summary = v
		case core.KeyType:
			item.Type = v
		case core.KeyStatus:
			item.Status = v
		case core.KeyParent:
			item.ParentID = v
		default:
			if item.Fields == nil {
				item.Fields = make(map[string]any)
			}
			item.Fields[k] = v
		}
	}
}
