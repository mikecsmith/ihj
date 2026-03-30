package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var branchSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateBranchCmd returns a git checkout command for the given issue.
// Used by both CLI and TUI.
func GenerateBranchCmd(issueKey, summary string) string {
	slug := strings.Trim(branchSlugRe.ReplaceAllString(strings.ToLower(summary), "-"), "-")
	return fmt.Sprintf("git checkout -b %s-%s", strings.ToLower(issueKey), slug)
}

// Branch copies a git-friendly branch name for the issue to the clipboard.
func Branch(ctx context.Context, ws *WorkspaceSession, issueKey string) error {
	item, err := ws.Provider.Get(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("issue %s not found: %w", issueKey, err)
	}

	branchCmd := GenerateBranchCmd(issueKey, item.Summary)

	if err := ws.Runtime.UI.CopyToClipboard(branchCmd); err != nil {
		ws.Runtime.UI.Notify("Branch (clipboard unavailable)", branchCmd)
		return nil
	}

	ws.Runtime.UI.Notify("Branch Copied!", branchCmd)
	return nil
}
