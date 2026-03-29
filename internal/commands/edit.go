package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Edit fetches an existing work item, opens it in the editor, and applies
// changes through the provider. Fully provider-agnostic.
func Edit(ws *WorkspaceSession, issueKey string, overrides map[string]string) error {
	workspace, _, _, _, origStatus, initialDoc, _, _, err := PrepareEdit(ws, issueKey, overrides)
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
		fm, recoverableMsg, err := SubmitEdit(ws, workspace, issueKey, edited, origStatus)
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
		if fm == nil {
			ws.Runtime.UI.Notify("No Changes", "Nothing to update.")
			return nil
		}

		ws.Runtime.UI.Notify("Updated", issueKey)
		PostEditNotify(ws, fm, issueKey, origStatus)
		return nil
	}
}

// PrepareEdit fetches the issue and builds the editor document.
// Used by the TUI for async edit flow.
func PrepareEdit(ws *WorkspaceSession, issueKey string, overrides map[string]string) (
	workspace *core.Workspace, schemaPath string,
	metadata map[string]string, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	workspace = ws.Workspace

	schemaPath, err = writeEditorSchema(ws)
	if err != nil {
		return
	}

	var item *core.WorkItem
	item, err = ws.Provider.Get(context.TODO(), issueKey)
	if err != nil {
		err = fmt.Errorf("fetching %s: %w", issueKey, err)
		return
	}

	metadata = workItemToMetadata(item)
	applyOverrides(metadata, overrides)
	origStatus = item.Status
	bodyText = item.DescriptionMarkdown()

	initialDoc = core.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat = CalculateCursor(initialDoc, metadata["summary"])
	return
}

// SubmitEdit parses, validates, and submits an edited document.
// Returns the parsed frontmatter, a recoverable error message (if any),
// or a hard error.
func SubmitEdit(ws *WorkspaceSession, workspace *core.Workspace, issueKey, edited, origStatus string) (
	fm map[string]string, recoverableMsg string, err error,
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

	// Fetch current state to compute diff.
	current, fetchErr := ws.Provider.Get(context.TODO(), issueKey)
	if fetchErr != nil {
		err = fmt.Errorf("fetching %s for diff: %w", issueKey, fetchErr)
		return
	}

	changes := frontmatterToChanges(fm, ast, current)
	if changes == nil {
		// No actual changes — not an error, just nothing to do.
		return
	}

	if updateErr := ws.Provider.Update(context.TODO(), issueKey, changes); updateErr != nil {
		recoverableMsg = fmt.Sprintf("API rejected update: %v", updateErr)
		return
	}

	return
}

// PostEditNotify handles post-edit notifications (sprint info).
// Status transitions are already handled by Provider.Update.
func PostEditNotify(ws *WorkspaceSession, fm map[string]string, issueKey, origStatus string) {
	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
	}
}

// writeEditorSchema generates and caches the frontmatter JSON schema.
func writeEditorSchema(ws *WorkspaceSession) (string, error) {
	schemaDict := core.FrontmatterSchema(ws.Workspace)
	schemaPath, err := writeSchema(ws.Runtime.CacheDir, ws.Workspace.Slug, core.Frontmatter, schemaDict)
	if err != nil {
		return "", fmt.Errorf("writing schema: %w", err)
	}
	return schemaPath, nil
}

// applyOverrides merges non-empty overrides into metadata.
func applyOverrides(metadata, overrides map[string]string) {
	for k, v := range overrides {
		if v != "" {
			metadata[k] = v
		}
	}
}
