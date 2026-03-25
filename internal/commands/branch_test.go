package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

func TestBranch_FromCache(t *testing.T) {
	dir := t.TempDir()
	issues := []jira.Issue{
		{Key: "FOO-42", Fields: jira.IssueFields{
			Summary:   "Fix the Login Page",
			IssueType: jira.IssueType{ID: "1", Name: "Bug"},
			Status:    jira.Status{Name: "Open"},
			Priority:  jira.Priority{Name: "High"},
			Created:   "2024-01-01T00:00:00.000+0000",
			Updated:   "2024-01-01T00:00:00.000+0000",
		}},
	}
	data, _ := json.Marshal(issues)
	_ = os.WriteFile(filepath.Join(dir, "eng_active.json"), data, 0o644)

	ui := &MockUI{}
	app := NewTestApp(ui)
	app.CacheDir = dir

	err := Branch(app, "FOO-42", "eng")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ui.ClipboardContents, "git checkout -b foo-42-fix-the-login-page") {
		t.Errorf("ClipboardContents = %q; want containing \"git checkout -b foo-42-fix-the-login-page\"", ui.ClipboardContents)
	}
}

func TestBranch_NotInCache(t *testing.T) {
	ui := &MockUI{}
	app := NewTestApp(ui)
	app.CacheDir = t.TempDir()

	err := Branch(app, "MISSING-1", "eng")
	if err == nil {
		t.Errorf("Branch(\"MISSING-1\") = nil; want error for missing issue")
	}
}

func TestGenerateBranchCmd(t *testing.T) {
	tests := []struct {
		name     string
		issueKey string
		summary  string
		want     string
	}{
		{"basic", "ENG-42", "Fix the Login Page", "git checkout -b eng-42-fix-the-login-page"},
		{"special chars", "FOO-1", "Add OAuth 2.0 (Google)", "git checkout -b foo-1-add-oauth-2-0-google"},
		{"trailing hyphens trimmed", "BAR-5", "---urgent---", "git checkout -b bar-5-urgent"},
		{"empty summary", "X-1", "", "git checkout -b x-1-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBranchCmd(tt.issueKey, tt.summary)
			if got != tt.want {
				t.Errorf("GenerateBranchCmd(%q, %q) = %q; want %q", tt.issueKey, tt.summary, got, tt.want)
			}
		})
	}
}
