package testutil

import (
	"io"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// TestWorkspace returns a canonical workspace for testing.
// Includes types, statuses, and weights sufficient for both
// commands and TUI tests.
func TestWorkspace() *core.Workspace {
	return &core.Workspace{
		Slug:     "eng",
		Name:     "Engineering",
		Provider: "test",
		BaseURL:  "https://test.example.com",
		Filters:  map[string]string{"default": "status != Done"},
		Statuses: []string{"Backlog", "To Do", "In Progress", "In Review", "Done"},
		Types: []core.TypeConfig{
			{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
			{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true},
			{ID: 11, Name: "Task", Order: 30, Color: "default"},
			{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
			{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
		},
		StatusWeights: map[string]int{
			"Backlog": 0, "To Do": 1, "In Progress": 2, "In Review": 3, "Done": 4,
		},
		TypeOrderMap: map[string]core.TypeOrderEntry{
			"Epic":     {Order: 20, Color: "magenta", HasChildren: true},
			"Story":    {Order: 30, Color: "blue", HasChildren: true},
			"Task":     {Order: 30, Color: "default"},
			"Spike":    {Order: 30, Color: "yellow"},
			"Sub-task": {Order: 40, Color: "white"},
		},
	}
}

// TestItems returns a standard set of work items for testing.
func TestItems() []*core.WorkItem {
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

// NewMockProvider creates a MockProvider pre-populated with TestItems
// and standard capabilities. Callers can override fields as needed.
func NewMockProvider() *MockProvider {
	items := TestItems()
	mp := &MockProvider{
		Registry:   make(map[string]*core.WorkItem, len(items)),
		Caps:       core.Capabilities{HasTransitions: true, HasTypes: true, HasHierarchy: true},
		UserReturn: &core.User{DisplayName: "Demo User", ID: "test-user"},
	}
	for _, item := range items {
		mp.Registry[item.ID] = item
	}
	return mp
}

// NewTestSession creates a Session backed by a MockUI, the canonical
// TestWorkspace, and a pre-populated MockProvider.
func NewTestSession(ui *MockUI) *commands.Session {
	ws := TestWorkspace()
	return &commands.Session{
		DefaultWorkspace: ws.Slug,
		Workspaces:       map[string]*core.Workspace{ws.Slug: ws},
		Provider:         NewMockProvider(),
		UI:               ui,
		Out:              io.Discard,
		Err:              io.Discard,
	}
}
