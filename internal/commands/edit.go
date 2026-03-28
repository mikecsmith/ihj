package commands

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Edit fetches an existing work item, opens it in the editor, and applies
// changes through the provider. Fully provider-agnostic.
func Edit(s *Session, workspaceSlug, issueKey string, overrides map[string]string) error {
	ws, err := s.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return err
	}

	schemaPath, err := writeEditorSchema(s, ws)
	if err != nil {
		return err
	}

	item, err := s.Provider.Get(nil, issueKey)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", issueKey, err)
	}

	metadata := workItemToMetadata(item)
	applyOverrides(metadata, overrides)
	origStatus := item.Status
	bodyText := item.DescriptionMarkdown()

	initialDoc := core.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat := CalculateCursor(initialDoc, metadata["summary"])

	edited, err := s.UI.EditText(initialDoc, "ihj_", cursorLine, searchPat)
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}
	if strings.TrimSpace(edited) == strings.TrimSpace(initialDoc) {
		return &CancelledError{Operation: "edit"}
	}

	for {
		fm, mdBody, parseErr := core.ParseFrontmatter(edited)
		if parseErr != nil {
			retry, err := offerRecovery(s, edited, fmt.Sprintf("YAML error: %v", parseErr))
			if err != nil || retry == "" {
				return &CancelledError{Operation: "edit"}
			}
			edited = retry
			continue
		}

		if errMsg := core.ValidateFrontmatter(fm); errMsg != "" {
			retry, err := offerRecovery(s, edited, errMsg)
			if err != nil || retry == "" {
				return &CancelledError{Operation: "edit"}
			}
			edited = retry
			continue
		}

		ast, err := document.ParseMarkdownString(mdBody)
		if err != nil {
			return fmt.Errorf("parsing description: %w", err)
		}

		changes := frontmatterToChanges(fm, ast, item)
		if changes == nil {
			s.UI.Notify("No Changes", "Nothing to update.")
			return nil
		}

		if err := s.Provider.Update(nil, issueKey, changes); err != nil {
			retry, retryErr := offerRecovery(s, edited, fmt.Sprintf("API rejected update: %v", err))
			if retryErr != nil || retry == "" {
				return err
			}
			edited = retry
			continue
		}

		s.UI.Notify("Updated", issueKey)

		// Sprint assignment (if status changed, provider already handled transition).
		postEditNotify(s, fm, issueKey, origStatus)
		return nil
	}
}

// PrepareEdit resolves the workspace, fetches the issue, and builds the
// editor document. Used by the TUI for async edit flow.
func PrepareEdit(s *Session, workspaceSlug, issueKey string, overrides map[string]string) (
	ws *core.Workspace, schemaPath string,
	metadata map[string]string, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	ws, err = s.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return
	}

	schemaPath, err = writeEditorSchema(s, ws)
	if err != nil {
		return
	}

	var item *core.WorkItem
	item, err = s.Provider.Get(nil, issueKey)
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
func SubmitEdit(s *Session, ws *core.Workspace, issueKey, edited, origStatus string) (
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
	current, fetchErr := s.Provider.Get(nil, issueKey)
	if fetchErr != nil {
		err = fmt.Errorf("fetching %s for diff: %w", issueKey, fetchErr)
		return
	}

	changes := frontmatterToChanges(fm, ast, current)
	if changes == nil {
		// No actual changes — not an error, just nothing to do.
		return
	}

	if err = s.Provider.Update(nil, issueKey, changes); err != nil {
		recoverableMsg = fmt.Sprintf("API rejected update: %v", err)
		return
	}

	return
}

// postEditNotify handles post-edit notifications (sprint info).
// Status transitions are already handled by Provider.Update.
func postEditNotify(s *Session, fm map[string]string, issueKey, origStatus string) {
	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		s.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
	}
}

// writeEditorSchema generates and caches the frontmatter JSON schema.
func writeEditorSchema(s *Session, ws *core.Workspace) (string, error) {
	schemaDict := core.FrontmatterSchema(ws)
	schemaPath, err := writeSchema(s.CacheDir, ws.Slug, core.Frontmatter, schemaDict)
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
