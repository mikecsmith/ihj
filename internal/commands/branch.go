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
func Branch(s *Session, issueKey string) error {
	item, err := s.Provider.Get(context.TODO(), issueKey)
	if err != nil {
		return fmt.Errorf("issue %s not found: %w", issueKey, err)
	}

	branchCmd := GenerateBranchCmd(issueKey, item.Summary)

	if err := s.UI.CopyToClipboard(branchCmd); err != nil {
		s.UI.Notify("Branch (clipboard unavailable)", branchCmd)
		return nil
	}

	s.UI.Notify("Branch Copied!", branchCmd)
	return nil
}
