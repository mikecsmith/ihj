package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// ─────────────────────────────────────────────────────────────
// Upsert state machine tests
// ─────────────────────────────────────────────────────────────

func newTestModelWithTypes() AppModel {
	m := newTestModel()
	m.ws.Types = []core.TypeConfig{
		{ID: 1, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
		{ID: 3, Name: "Task", Order: 30, Color: "default", HasChildren: true},
		{ID: 5, Name: "Sub-task", Order: 40, Color: "white", HasChildren: false},
	}
	return m
}

func TestUpsertEditFlow_Prepare(t *testing.T) {
	m := newTestModelWithTypes()
	iss := m.list.SelectedIssue()
	if iss == nil {
		t.Fatal("no selected issue")
	}

	// Press alt+e to start edit.
	result, cmd := m.Update(altKey('e'))
	m = result.(AppModel)

	if m.upsertPhase != upsertAwaitingEditor {
		t.Fatalf("expected upsertAwaitingEditor, got %d", m.upsertPhase)
	}
	if cmd == nil {
		t.Fatal("alt+e should return a cmd for async prepare")
	}
}

func TestUpsertCreateFlow_TypePopup(t *testing.T) {
	m := newTestModelWithTypes()

	// Press ctrl+n to start create.
	result, _ := m.Update(ctrlKey('n'))
	m = result.(AppModel)

	if m.upsertPhase != upsertAwaitingTypeSelect {
		t.Fatalf("expected upsertAwaitingTypeSelect, got %d", m.upsertPhase)
	}
	if !m.popup.Active() {
		t.Fatal("type selection popup should be active after ctrl+n")
	}
}

func TestUpsertEditorDone_NoChanges(t *testing.T) {
	m := newTestModelWithTypes()
	initialDoc := "---\nsummary: test\n---\nBody"

	m.upsertPhase = upsertAwaitingEditor
	m.upsertCtx = &upsertContext{
		opts:       commands.UpsertOpts{IsEdit: true, IssueKey: "TEST-1"},
		initialDoc: initialDoc,
	}

	// Simulate editor returning with no changes.
	msg := upsertEditorDoneMsg{
		ctx: &upsertContext{
			opts:       commands.UpsertOpts{IsEdit: true, IssueKey: "TEST-1"},
			initialDoc: initialDoc,
			edited:     initialDoc,
		},
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	if m.upsertPhase != upsertIdle {
		t.Fatalf("expected upsertIdle after no changes, got %d", m.upsertPhase)
	}
	if !strings.Contains(m.notify, "No changes") {
		t.Errorf("notify = %q; want substring \"No changes\"", m.notify)
	}
}

func TestUpsertEditorDone_Error(t *testing.T) {
	m := newTestModelWithTypes()
	m.upsertPhase = upsertAwaitingEditor
	m.upsertCtx = &upsertContext{
		opts: commands.UpsertOpts{IsEdit: true, IssueKey: "TEST-1"},
	}

	msg := upsertEditorDoneMsg{
		ctx: m.upsertCtx,
		err: os.ErrNotExist,
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	if m.upsertPhase != upsertIdle {
		t.Fatalf("expected upsertIdle after editor error, got %d", m.upsertPhase)
	}
	if !strings.Contains(m.notify, "Editor error") {
		t.Errorf("notify = %q; want substring \"Editor error\"", m.notify)
	}
}

func TestUpsertSubmitResult_Recovery(t *testing.T) {
	m := newTestModelWithTypes()
	m.upsertPhase = upsertAwaitingEditor
	ctx := &upsertContext{
		opts:   commands.UpsertOpts{IsEdit: true, IssueKey: "TEST-1"},
		edited: "---\nsummary: \"\"\n---\n",
	}
	m.upsertCtx = ctx

	// Simulate a recoverable error.
	msg := upsertSubmitResultMsg{
		ctx:    ctx,
		errMsg: "Summary is required.",
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	if m.upsertPhase != upsertAwaitingRecovery {
		t.Fatalf("expected upsertAwaitingRecovery, got %d", m.upsertPhase)
	}
	if !m.popup.Active() {
		t.Fatal("recovery popup should be active")
	}
}

func TestUpsertRecovery_Abort(t *testing.T) {
	m := newTestModelWithTypes()
	m.upsertPhase = upsertAwaitingRecovery
	m.upsertCtx = &upsertContext{
		opts:   commands.UpsertOpts{IsEdit: true, IssueKey: "TEST-1"},
		edited: "some content",
	}

	// Simulate selecting "Abort" (index 2) from recovery popup.
	result := &PopupResult{ID: "upsert-recovery", Index: 2, Value: "Abort"}
	m2, _ := m.handlePopupResult(result)
	m = m2.(AppModel)

	if m.upsertPhase != upsertIdle {
		t.Fatalf("expected upsertIdle after abort, got %d", m.upsertPhase)
	}
	if m.upsertCtx != nil {
		t.Fatal("upsertCtx should be nil after abort")
	}
}

func TestPostUpsertComplete_Success(t *testing.T) {
	m := newTestModelWithTypes()

	// Simulate a successful upsert of TEST-1 with status change.
	msg := postUpsertCompleteMsg{
		notifications: []string{"TEST-1 → Done"},
		item: &core.WorkItem{
			ID:      "TEST-1",
			Summary: "Updated Epic",
			Type:    "Epic",
			Status:  "Done",
			Fields: map[string]any{
				"priority": "High", "assignee": "Alice", "reporter": "Bob",
				"updated": "20 Mar 2026",
			},
		},
		issueKey: "TEST-1",
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	iss, ok := m.registry["TEST-1"]
	if !ok {
		t.Fatal("TEST-1 should be in registry after post-upsert")
	}
	if iss.Status != "Done" {
		t.Errorf("registry[\"TEST-1\"].Status = %q; want \"Done\"", iss.Status)
	}
	if iss.Summary != "Updated Epic" {
		t.Errorf("registry[\"TEST-1\"].Summary = %q; want \"Updated Epic\"", iss.Summary)
	}
}

func TestPostUpsertComplete_FetchError(t *testing.T) {
	m := newTestModelWithTypes()
	originalStatus := m.registry["TEST-1"].Status

	// Simulate post-upsert with fetch failure.
	msg := postUpsertCompleteMsg{
		notifications: []string{"TEST-1 → Done"},
		issueKey:      "TEST-1",
		fetchErr:      os.ErrPermission,
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	// Registry should NOT be updated on fetch failure.
	if m.registry["TEST-1"].Status != originalStatus {
		t.Errorf("registry[\"TEST-1\"].Status = %q; want %q (unchanged on fetch error)", m.registry["TEST-1"].Status, originalStatus)
	}
	if !strings.Contains(m.notify, "Sync warning") {
		t.Errorf("notify = %q; want substring \"Sync warning\"", m.notify)
	}
}

func TestPostUpsertComplete_Create(t *testing.T) {
	m := newTestModelWithTypes()
	initialCount := len(m.registry)

	// Simulate creating a new issue.
	msg := postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID:      "TEST-99",
			Summary: "Brand New Issue",
			Type:    "Task",
			Status:  "To Do",
			Fields: map[string]any{
				"priority": "Medium", "assignee": "Unassigned", "reporter": "Demo User",
			},
		},
		issueKey: "TEST-99",
		isCreate: true,
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	if len(m.registry) != initialCount+1 {
		t.Fatalf("expected %d issues in registry, got %d", initialCount+1, len(m.registry))
	}
	newIss, ok := m.registry["TEST-99"]
	if !ok {
		t.Fatal("TEST-99 should be in registry after create")
	}
	if newIss.Summary != "Brand New Issue" {
		t.Errorf("registry[\"TEST-99\"].Summary = %q; want \"Brand New Issue\"", newIss.Summary)
	}
}
