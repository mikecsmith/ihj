package commands_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestPrepareCreate(t *testing.T) {
	tests := []struct {
		name         string
		selectedType string
		overrides    map[string]string
		wantErr      bool
		checkDoc     string // substring expected in initialDoc
		checkStatus  string // expected origStatus
	}{
		{
			name:         "success - builds document for Story",
			selectedType: "Story",
			checkDoc:     "Story",
			checkStatus:  "Backlog",
		},
		{
			name:         "with overrides",
			selectedType: "Task",
			overrides:    map[string]string{"priority": "Critical", "summary": "Prefilled"},
			checkDoc:     "Prefilled",
			checkStatus:  "Backlog",
		},
		{
			name:         "with status override",
			selectedType: "Epic",
			overrides:    map[string]string{"status": "In Progress"},
			checkDoc:     "In Progress",
			checkStatus:  "Backlog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &testutil.MockUI{}
			ws := testutil.NewTestSession(ui)
			ws.Runtime.CacheDir = t.TempDir()
			ws.Provider = &testutil.MockProvider{}

			_, _, _, _, origStatus, initialDoc, _, _, err := commands.PrepareCreate(ws, tt.selectedType, tt.overrides)

			if (err != nil) != tt.wantErr {
				t.Fatalf("PrepareCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if origStatus != tt.checkStatus {
				t.Errorf("origStatus = %q; want %q", origStatus, tt.checkStatus)
			}
			if !strings.Contains(initialDoc, tt.checkDoc) {
				t.Errorf("initialDoc does not contain %q:\n%s", tt.checkDoc, initialDoc)
			}
		})
	}
}

func TestSubmitCreate(t *testing.T) {
	tests := []struct {
		name        string
		edited      string
		createErr   error
		wantKey     string // expected issue key prefix
		wantFM      bool   // expect non-nil frontmatter
		wantRecover string // substring in recoverable message
		wantErr     bool
	}{
		{
			name:    "success - creates issue",
			edited:  "---\ntype: Story\nstatus: To Do\nsummary: New Feature\npriority: Medium\n---\nDescription here",
			wantKey: "ENG-",
			wantFM:  true,
		},
		{
			name:        "invalid YAML - recoverable",
			edited:      "---\n: [broken\n---\n",
			wantRecover: "YAML error",
		},
		{
			name:        "missing summary - recoverable validation error",
			edited:      "---\ntype: Story\nstatus: To Do\nsummary: \"\"\n---\n",
			wantRecover: "Summary",
		},
		{
			name:        "API error - recoverable",
			edited:      "---\ntype: Story\nstatus: To Do\nsummary: Good Summary\npriority: High\n---\n",
			createErr:   fmt.Errorf("rate limited"),
			wantRecover: "API rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &testutil.MockUI{}
			ws := testutil.NewTestSession(ui)
			ws.Provider = &testutil.MockProvider{
				CreatePrefix: "ENG",
				CreateErr:    tt.createErr,
			}

			issueKey, fm, recoverableMsg, err := commands.SubmitCreate(ws, tt.edited)

			if (err != nil) != tt.wantErr {
				t.Fatalf("SubmitCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantRecover != "" {
				if !strings.Contains(recoverableMsg, tt.wantRecover) {
					t.Errorf("recoverableMsg = %q; want containing %q", recoverableMsg, tt.wantRecover)
				}
				return
			}
			if tt.wantKey != "" && !strings.HasPrefix(issueKey, tt.wantKey) {
				t.Errorf("issueKey = %q; want prefix %q", issueKey, tt.wantKey)
			}
			if tt.wantFM && fm == nil {
				t.Error("expected non-nil frontmatter")
			}
		})
	}
}
