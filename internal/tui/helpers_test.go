package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
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
		Provider: "test",
		BaseURL:  "https://test.example.com",
		Statuses: []string{"Backlog", "To Do", "In Progress", "In Review", "Done"},
		StatusWeights: map[string]int{
			"Backlog": 0, "To Do": 1, "In Progress": 2, "In Review": 3, "Done": 4,
		},
		TypeOrderMap:   map[string]core.TypeOrderEntry{},
		Filters:        map[string]string{"default": ""},
	}
}

// testSession creates a minimal Session with a MockProvider for testing.
func testSession() *commands.Session {
	ws := testWorkspace()
	items := testItems()
	mp := &core.MockProvider{
		Registry:   map[string]*core.WorkItem{},
		Caps:       core.Capabilities{HasTransitions: true, HasTypes: true, HasHierarchy: true},
		UserReturn: &core.User{DisplayName: "Demo User", ID: "test-user"},
	}
	for _, item := range items {
		mp.Registry[item.ID] = item
	}

	return &commands.Session{
		DefaultWorkspace: "test",
		Workspaces:       map[string]*core.Workspace{"test": ws},
		Provider:         mp,
		UI:               &BubbleTeaUI{},
		CacheDir:         "/tmp/ihj-test",
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

func newTestModel() AppModel {
	s := testSession()
	ws := testWorkspace()
	items := testItems()
	m := NewAppModel(s, ws, "default", items, time.Time{})
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
