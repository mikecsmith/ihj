package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mikecsmith/ihj/internal/client"
)

var branchSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateBranchCmd returns a git checkout command for the given issue.
// Used by both CLI and TUI.
func GenerateBranchCmd(issueKey, summary string) string {
	slug := strings.Trim(branchSlugRe.ReplaceAllString(strings.ToLower(summary), "-"), "-")
	return fmt.Sprintf("git checkout -b %s-%s", strings.ToLower(issueKey), slug)
}

func Branch(app *App, issueKey, boardSlug string) error {
	summary := findCachedSummary(app.CacheDir, issueKey, boardSlug)
	if summary == "" {
		return fmt.Errorf("issue %s not found in local cache", issueKey)
	}

	branchCmd := GenerateBranchCmd(issueKey, summary)

	if err := app.UI.CopyToClipboard(branchCmd); err != nil {
		app.UI.Notify("Branch (clipboard unavailable)", branchCmd)
		return nil
	}

	app.UI.Notify("Branch Copied!", branchCmd)
	return nil
}

func findCachedSummary(cacheDir, issueKey, boardSlug string) string {
	pattern := "*.json"
	if boardSlug != "" {
		pattern = boardSlug + "_*.json"
	}

	files, _ := filepath.Glob(filepath.Join(cacheDir, pattern))
	for _, f := range files {
		if strings.HasSuffix(f, ".state.json") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var issues []client.Issue
		if json.Unmarshal(data, &issues) != nil {
			continue
		}
		for _, iss := range issues {
			if iss.Key == issueKey {
				return iss.Fields.Summary
			}
		}
	}
	return ""
}
