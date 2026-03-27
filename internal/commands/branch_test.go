package commands

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestBranch_Success(t *testing.T) {
	ui := &MockUI{}
	s := NewTestSession(ui)
	s.Provider = &core.MockProvider{
		GetReturn: &core.WorkItem{
			ID:      "FOO-42",
			Summary: "Fix the Login Page",
			Type:    "Bug",
			Status:  "Open",
		},
	}

	err := Branch(s, "FOO-42")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ui.ClipboardContents, "git checkout -b foo-42-fix-the-login-page") {
		t.Errorf("ClipboardContents = %q; want containing \"git checkout -b foo-42-fix-the-login-page\"", ui.ClipboardContents)
	}
}

func TestBranch_NotFound(t *testing.T) {
	ui := &MockUI{}
	s := NewTestSession(ui)
	s.Provider = &core.MockProvider{
		Registry: map[string]*core.WorkItem{}, // empty registry
	}

	err := Branch(s, "MISSING-1")
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
