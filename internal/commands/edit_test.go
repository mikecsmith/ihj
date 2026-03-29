package commands_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestPrepareEdit(t *testing.T) {
	tests := []struct {
		name        string
		issueKey    string
		item        *core.WorkItem
		overrides   map[string]string
		wantErr     bool
		checkStatus string
		checkDoc    string // substring expected in initialDoc
	}{
		{
			name:     "success - fetches issue and builds document",
			issueKey: "ENG-1",
			item: &core.WorkItem{
				ID: "ENG-1", Summary: "Fix bug", Type: "Task",
				Status: "In Progress", Fields: map[string]any{"priority": "High"},
			},
			checkStatus: "In Progress",
			checkDoc:    "Fix bug",
		},
		{
			name:     "overrides are applied",
			issueKey: "ENG-2",
			item: &core.WorkItem{
				ID: "ENG-2", Summary: "Original", Type: "Story",
				Status: "To Do", Fields: map[string]any{"priority": "Low"},
			},
			overrides:   map[string]string{"priority": "Critical"},
			checkStatus: "To Do",
			checkDoc:    "Critical",
		},
		{
			name:     "issue not found",
			issueKey: "MISSING-1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &testutil.MockUI{}
			ws := testutil.NewTestSession(ui)
			ws.Runtime.CacheDir = t.TempDir()

			mp := &testutil.MockProvider{}
			if tt.item != nil {
				mp.Registry = map[string]*core.WorkItem{tt.item.ID: tt.item}
			} else {
				mp.Registry = map[string]*core.WorkItem{}
			}
			ws.Provider = mp

			_, _, _, _, origStatus, initialDoc, _, _, err := commands.PrepareEdit(ws, tt.issueKey, tt.overrides)

			if (err != nil) != tt.wantErr {
				t.Fatalf("PrepareEdit() error = %v, wantErr %v", err, tt.wantErr)
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

func TestSubmitEdit(t *testing.T) {
	tests := []struct {
		name           string
		issueKey       string
		current        *core.WorkItem
		edited         string
		origStatus     string
		updateErr      error
		wantFM         bool   // expect non-nil frontmatter
		wantRecover    string // substring in recoverable message
		wantErr        bool
		wantUpdateCall bool
	}{
		{
			name:     "success - applies changes",
			issueKey: "ENG-1",
			current: &core.WorkItem{
				ID: "ENG-1", Summary: "Old Summary", Type: "Task", Status: "To Do",
			},
			edited:         "---\nkey: ENG-1\ntype: Task\nstatus: To Do\nsummary: New Summary\n---\n",
			origStatus:     "To Do",
			wantFM:         true,
			wantUpdateCall: true,
		},
		{
			name:     "no changes - returns parsed fm but no update call",
			issueKey: "ENG-1",
			current: &core.WorkItem{
				ID: "ENG-1", Summary: "Same", Type: "Task", Status: "To Do",
			},
			edited:     "---\nkey: ENG-1\ntype: Task\nstatus: To Do\nsummary: Same\n---\n",
			origStatus: "To Do",
			wantFM:     true,
		},
		{
			name:        "invalid YAML - recoverable",
			issueKey:    "ENG-1",
			current:     &core.WorkItem{ID: "ENG-1", Summary: "X", Type: "Task", Status: "To Do"},
			edited:      "---\n: [broken\n---\n",
			wantRecover: "YAML error",
		},
		{
			name:        "missing summary - recoverable validation error",
			issueKey:    "ENG-1",
			current:     &core.WorkItem{ID: "ENG-1", Summary: "X", Type: "Task", Status: "To Do"},
			edited:      "---\nkey: ENG-1\ntype: Task\nstatus: To Do\nsummary:\n---\n",
			wantRecover: "Summary",
		},
		{
			name:     "API error - recoverable",
			issueKey: "ENG-1",
			current: &core.WorkItem{
				ID: "ENG-1", Summary: "Old", Type: "Task", Status: "To Do",
			},
			edited:      "---\nkey: ENG-1\ntype: Task\nstatus: To Do\nsummary: New\n---\n",
			updateErr:   fmt.Errorf("permission denied"),
			wantRecover: "API rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &testutil.MockUI{}
			ws := testutil.NewTestSession(ui)

			mp := &testutil.MockProvider{
				Registry:  map[string]*core.WorkItem{tt.issueKey: tt.current},
				UpdateErr: tt.updateErr,
			}
			ws.Provider = mp

			fm, recoverableMsg, err := commands.SubmitEdit(ws, testutil.TestWorkspace(), tt.issueKey, tt.edited, tt.origStatus)

			if (err != nil) != tt.wantErr {
				t.Fatalf("SubmitEdit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantRecover != "" {
				if !strings.Contains(recoverableMsg, tt.wantRecover) {
					t.Errorf("recoverableMsg = %q; want containing %q", recoverableMsg, tt.wantRecover)
				}
				return
			}
			if tt.wantFM && fm == nil {
				t.Error("expected non-nil frontmatter")
			}
			if !tt.wantFM && fm != nil {
				t.Errorf("expected nil frontmatter, got %v", fm)
			}
			if tt.wantUpdateCall && len(mp.UpdateCalls) == 0 {
				t.Error("expected Provider.Update to be called")
			}
		})
	}
}
