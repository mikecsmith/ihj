package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/storage"
)

// altKey creates a KeyPressMsg for alt+<key> that String() resolves to "alt+<key>".
func altKey(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Mod: tea.ModAlt}
}

// ctrlKey creates a KeyPressMsg for ctrl+<key>.
func ctrlKey(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Mod: tea.ModCtrl}
}

// enterKey creates an enter KeyPressMsg.
func enterKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// testWorkspace creates a minimal workspace for testing.
func testWorkspace() *core.Workspace {
	return &core.Workspace{
		Slug:     "test",
		Name:     "Test Board",
		Provider: "jira",
		BaseURL:  "https://test.atlassian.net",
		Statuses: []string{"Backlog", "To Do", "In Progress", "In Review", "Done"},
		StatusWeights: map[string]int{
			"Backlog": 0, "To Do": 1, "In Progress": 2, "In Review": 3, "Done": 4,
		},
		TypeOrderMap: map[string]core.TypeOrderEntry{},
		Filters:      map[string]string{"default": ""},
		ProviderConfig: &jira.Config{
			Server:     "https://test.atlassian.net",
			ProjectKey: "TEST",
		},
	}
}

// testApp creates a minimal App with a MockClient and Provider for testing.
func testApp() *commands.App {
	ws := testWorkspace()
	mc := jira.NewMockClient(nil,
		[]string{"Backlog", "To Do", "In Progress", "In Review", "Done"},
		"TEST",
	)
	provider := jira.NewProvider(mc, ws)

	return &commands.App{
		Config: &storage.AppConfig{
			DefaultWorkspace: "test",
			Editor:           "vim",
			Workspaces:       map[string]*core.Workspace{"test": ws},
		},
		Client:   mc,
		Provider: provider,
		UI:       &BubbleTeaUI{},
		CacheDir: "/tmp/ihj-test",
	}
}

// testItems creates a set of WorkItems for testing.
func testItems() []*core.WorkItem {
	return []*core.WorkItem{
		{
			ID: "TEST-1", Summary: "Epic One", Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority": "High", "assignee": "Alice", "reporter": "Bob",
				"created": "1 Jan 2025", "updated": "15 Jan 2025",
			},
		},
		{
			ID: "TEST-2", Summary: "Story One", Type: "Story", Status: "To Do",
			Fields: map[string]any{
				"priority": "Medium", "assignee": "Charlie", "reporter": "Alice",
				"created": "2 Jan 2025", "updated": "16 Jan 2025",
			},
		},
	}
}

// seedMockClient adds the test items to the mock client so transitions etc. work.
func seedMockClient(app *commands.App, items []*core.WorkItem) {
	mc := app.Client.(*jira.MockClient)
	for _, item := range items {
		raw := jira.Issue{
			Key: item.ID,
			Fields: jira.IssueFields{
				Summary:   item.Summary,
				Status:    jira.Status{Name: item.Status},
				IssueType: jira.IssueType{Name: item.Type},
			},
		}
		mc.AddIssue(raw)
	}
}

func newTestModel() AppModel {
	app := testApp()
	ws := testWorkspace()
	items := testItems()
	seedMockClient(app, items)
	m := NewAppModel(app, ws, "default", items, time.Time{})
	// Simulate window size.
	m.width = 120
	m.height = 40
	m.ready = true
	// Pre-populate cached user for tests that need it (e.g. assign).
	m.cachedUserName = "Demo User"
	m.recalcLayout()
	m.syncDetail()
	return m
}

// ─────────────────────────────────────────────────────────────
// Transition flow
// ─────────────────────────────────────────────────────────────

func TestTransitionFlow(t *testing.T) {
	m := newTestModel()

	// 1. Press alt+t to trigger transition popup (shows immediately from workspace statuses).
	result, cmd := m.Update(altKey('t'))
	m = result.(AppModel)
	// No async fetch — popup opens directly.
	if cmd != nil {
		t.Log("alt+t returned a cmd (unexpected but not fatal)")
	}
	if !m.popup.Active() {
		t.Fatal("popup should be active after alt+t")
	}

	// 2. Select first transition (press enter).
	result, cmd = m.Update(enterKey())
	m = result.(AppModel)
	if cmd == nil {
		t.Fatal("selecting a transition should return a cmd")
	}

	// 3. Execute the transition cmd.
	msg := cmd()
	done, ok := msg.(transitionDoneMsg)
	if !ok {
		t.Fatalf("expected transitionDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("transition error: %v", done.err)
	}

	// 4. Feed transitionDoneMsg — should update registry.
	result, _ = m.Update(done)
	m = result.(AppModel)

	// Check the registry was updated for the transitioned issue.
	if iss, ok := m.registry[done.issueKey]; ok {
		if iss.Status != done.newStatus {
			t.Errorf("registry status = %q, want %q", iss.Status, done.newStatus)
		}
	} else {
		t.Errorf("issue %s not found in registry", done.issueKey)
	}

	// Check notification was set.
	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty after transition", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Comment flow
// ─────────────────────────────────────────────────────────────

func TestCommentFlow(t *testing.T) {
	m := newTestModel()
	selectedKey := m.list.SelectedIssue().ID

	// 1. Press alt+c to open comment popup.
	result, _ := m.Update(altKey('c'))
	m = result.(AppModel)
	if !m.popup.Active() {
		t.Fatal("popup should be active after alt+c")
	}

	// 2. Type some text and submit.
	m.popup.input.SetValue("This is a test comment")
	result, cmd := m.Update(ctrlKey('s'))
	m = result.(AppModel)

	if cmd == nil {
		t.Fatal("submitting comment should return a cmd")
	}

	// 3. Execute the cmd.
	msg := cmd()
	done, ok := msg.(commentDoneMsg)
	if !ok {
		t.Fatalf("expected commentDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("comment error: %v", done.err)
	}
	if done.issueKey != selectedKey {
		t.Errorf("comment issueKey = %q, want %q", done.issueKey, selectedKey)
	}

	// 4. Feed commentDoneMsg — should append to WorkItem.
	prevCommentCount := len(m.registry[selectedKey].Comments)
	result, _ = m.Update(done)
	m = result.(AppModel)

	if got := len(m.registry[selectedKey].Comments); got != prevCommentCount+1 {
		t.Errorf("len(Comments) = %d; want %d", got, prevCommentCount+1)
	}
	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty after comment", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Assign flow
// ─────────────────────────────────────────────────────────────

func TestAssignFlow(t *testing.T) {
	m := newTestModel()
	selectedKey := m.list.SelectedIssue().ID

	// 1. Press alt+a.
	result, cmd := m.Update(altKey('a'))
	m = result.(AppModel)
	if cmd == nil {
		t.Fatal("alt+a should return a cmd for async assign")
	}

	// 2. Execute the cmd.
	msg := cmd()
	done, ok := msg.(assignDoneMsg)
	if !ok {
		t.Fatalf("expected assignDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("assign error: %v", done.err)
	}
	if done.issueKey != selectedKey {
		t.Errorf("assign issueKey = %q, want %q", done.issueKey, selectedKey)
	}

	// 3. Feed assignDoneMsg — should update registry.
	result, _ = m.Update(done)
	m = result.(AppModel)

	if m.registry[selectedKey].StringField("assignee") != done.assignee {
		t.Errorf("assignee = %q, want %q", m.registry[selectedKey].StringField("assignee"), done.assignee)
	}
	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty after assign", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Notification rendering
// ─────────────────────────────────────────────────────────────

func TestNotifyRenderedInView(t *testing.T) {
	m := newTestModel()
	m.setNotify("Test notification")

	v := m.View()
	content := v.Content
	if content == "" {
		t.Fatal("view should produce content")
	}
	// The notification text should appear somewhere in the rendered output.
	if !containsString(content, "Test notification") {
		t.Errorf("View() does not contain \"Test notification\"; want it visible")
	}
}

// ─────────────────────────────────────────────────────────────
// Notification auto-clear
// ─────────────────────────────────────────────────────────────

func TestNotifyAutoClear(t *testing.T) {
	m := newTestModel()
	m.notify = "Old notification"
	m.notifyAt = time.Now().Add(-5 * time.Second) // 5 seconds ago.

	result, _ := m.Update(tickMsg(time.Now()))
	m = result.(AppModel)
	if m.notify != "" {
		t.Errorf("notify should be cleared after 4s, got %q", m.notify)
	}
}

func TestNotifyNotClearedTooEarly(t *testing.T) {
	m := newTestModel()
	m.notify = "Recent notification"
	m.notifyAt = time.Now().Add(-2 * time.Second) // 2 seconds ago.

	result, _ := m.Update(tickMsg(time.Now()))
	m = result.(AppModel)
	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty (should not clear before 4s)", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Filter popup
// ─────────────────────────────────────────────────────────────

func TestFilterSingleFilterNotifiesOnly(t *testing.T) {
	m := newTestModel()
	// Workspace has only one filter (default).

	result, _ := m.Update(altKey('f'))
	m = result.(AppModel)

	// Should NOT open popup — should just notify.
	if m.popup.Active() {
		t.Errorf("popup.Active() = true; want false with only one filter")
	}
	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty (should inform user there's only one filter)", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Filter switching
// ─────────────────────────────────────────────────────────────

func TestFilterSwitch_MultipleFilters(t *testing.T) {
	m := newTestModel()
	// Add a second filter so popup should open.
	m.ws.Filters["backlog"] = "status = Backlog"

	result, _ := m.Update(altKey('f'))
	m = result.(AppModel)

	if !m.popup.Active() {
		t.Fatal("popup should open when multiple filters are available")
	}
}

func TestFilterSwitch_SameFilter(t *testing.T) {
	m := newTestModel()
	m.ws.Filters["backlog"] = "status = Backlog"

	// Open filter popup.
	result, _ := m.Update(altKey('f'))
	m = result.(AppModel)

	if !m.popup.Active() {
		t.Fatal("popup should be active")
	}

	// Select the current filter (default is at index 0 in sorted order).
	// Simulate selecting the already-active filter via popup result.
	pr := &PopupResult{ID: "filter", Index: 0, Value: m.filter}
	result2, _ := m.handlePopupResult(pr)
	m = result2.(AppModel)

	if m.notify == "" {
		t.Errorf("notify = %q; want non-empty when selecting same filter", m.notify)
	}
}

// ─────────────────────────────────────────────────────────────
// Post comment via shared postCommentCmd
// ─────────────────────────────────────────────────────────────

func TestPostCommentCmd(t *testing.T) {
	m := newTestModel()
	selectedKey := m.list.SelectedIssue().ID

	cmd := (&m).postCommentCmd(selectedKey, "Test comment via shared path")
	if cmd == nil {
		t.Fatal("postCommentCmd should return a tea.Cmd")
	}

	msg := cmd()
	done, ok := msg.(commentDoneMsg)
	if !ok {
		t.Fatalf("expected commentDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("comment error: %v", done.err)
	}
	if done.issueKey != selectedKey {
		t.Errorf("comment issueKey = %q, want %q", done.issueKey, selectedKey)
	}
}

// ─────────────────────────────────────────────────────────────
// Data reload
// ─────────────────────────────────────────────────────────────

func TestDataReloadMsg_UpdatesRegistry(t *testing.T) {
	m := newTestModel()
	initialCount := len(m.registry)

	msg := dataReloadedMsg{
		filter: "default",
		items: []*core.WorkItem{
			{
				ID: "TEST-50", Summary: "New Issue", Type: "Task", Status: "To Do",
				Fields: map[string]any{
					"priority": "Low", "assignee": "Eve", "reporter": "Alice",
					"created": "5 Jan 2025", "updated": "20 Jan 2025",
				},
			},
		},
	}

	result, _ := m.Update(msg)
	m = result.(AppModel)

	if len(m.registry) == initialCount {
		t.Errorf("len(registry) = %d; want > %d after reload", len(m.registry), initialCount)
	}
	if _, ok := m.registry["TEST-50"]; !ok {
		t.Fatal("TEST-50 should be in registry after reload")
	}
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		// Simple substring check (no ANSI awareness needed for basic test).
		indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
