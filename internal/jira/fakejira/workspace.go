package fakejira

import (
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
)

// Workspace returns the scrum demo workspace metadata pointing at the
// DEMO project. It leaves BaseURL empty and ProviderConfig as a raw
// map[string]any so the caller can plug in the fakejira server's URL
// and then run it through jira.HydrateWorkspace — the demo workspace
// goes through the same hydration path as any real Jira workspace.
func Workspace() *core.Workspace {
	types := []core.TypeConfig{
		{ID: 10, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
		{ID: 11, Name: "Story", Order: 30, Color: "cyan", HasChildren: true},
		{ID: 12, Name: "Task", Order: 30, Color: "default", HasChildren: true},
		{ID: 13, Name: "Bug", Order: 30, Color: "red", HasChildren: false},
		{ID: 14, Name: "Sub-task", Order: 40, Color: "white", HasChildren: false},
	}
	statuses := []core.StatusConfig{
		{Name: "To Do", Order: 10, Color: "cyan"},
		{Name: "In Progress", Order: 20, Color: "blue"},
		{Name: "In Review", Order: 30, Color: "magenta"},
		{Name: "Done", Order: 40, Color: "green"},
	}

	providerCfg := map[string]any{
		"project_key": ProjectKey,
		"board_id":    BoardID,
		"board_type":  BoardType,
		"jql":         `project = "{project_key}"`,
		"custom_fields": map[string]any{
			"story_points": 10016,
			"sprint":       10020,
		},
	}

	return &core.Workspace{
		Slug:           "demo",
		Name:           "Demo (Scrum)",
		Provider:       core.ProviderDemo,
		ServerAlias:    "demo",
		Types:          types,
		Statuses:       statuses,
		StatusOrderMap: statusOrderMap(statuses),
		TypeOrderMap:   typeOrderMap(types),
		Filters: map[string]string{
			"all":     "",
			"active":  "sprint IN openSprints() AND sprint NOT IN futureSprints()",
			"backlog": "sprint IS EMPTY OR sprint NOT IN openSprints()",
			"me":      `assignee = currentUser()`,
		},
		ProviderConfig: providerCfg,
	}
}

// WorkspaceKanban returns the kanban demo workspace metadata pointing at
// the OPS project. Unlike the scrum workspace it has no sprints —
// filters are status-based, and the board is kanban-typed so the
// provider skips sprint queries entirely.
func WorkspaceKanban() *core.Workspace {
	types := []core.TypeConfig{
		{ID: 10, Name: "Incident", Order: 10, Color: "red", HasChildren: false},
		{ID: 11, Name: "Request", Order: 20, Color: "cyan", HasChildren: false},
		{ID: 12, Name: "Task", Order: 30, Color: "default", HasChildren: false},
		{ID: 13, Name: "Bug", Order: 40, Color: "magenta", HasChildren: false},
	}
	statuses := []core.StatusConfig{
		{Name: "Triage", Order: 10, Color: "cyan"},
		{Name: "In Progress", Order: 20, Color: "blue"},
		{Name: "Blocked", Order: 30, Color: "red"},
		{Name: "Done", Order: 40, Color: "green"},
	}

	providerCfg := map[string]any{
		"project_key": KanbanProjectKey,
		"board_id":    KanbanBoardID,
		"board_type":  KanbanBoardType,
		"jql":         `project = "{project_key}"`,
	}

	return &core.Workspace{
		Slug:           "ops",
		Name:           "Ops (Kanban)",
		Provider:       core.ProviderDemo,
		ServerAlias:    "ops",
		Types:          types,
		Statuses:       statuses,
		StatusOrderMap: statusOrderMap(statuses),
		TypeOrderMap:   typeOrderMap(types),
		Filters: map[string]string{
			"all":    "",
			"active": `status != "Done"`,
			"done":   `status = "Done"`,
			"me":     `assignee = currentUser()`,
		},
		ProviderConfig: providerCfg,
	}
}

func typeOrderMap(types []core.TypeConfig) map[string]core.TypeOrderEntry {
	m := make(map[string]core.TypeOrderEntry, len(types))
	for _, t := range types {
		m[strings.ToLower(t.Name)] = core.TypeOrderEntry{
			Order: t.Order, Color: t.Color, HasChildren: t.HasChildren,
		}
	}
	return m
}

func statusOrderMap(statuses []core.StatusConfig) map[string]core.StatusOrderEntry {
	m := make(map[string]core.StatusOrderEntry, len(statuses))
	for _, s := range statuses {
		m[strings.ToLower(s.Name)] = core.StatusOrderEntry{
			Weight: s.Order, Color: s.Color,
		}
	}
	return m
}
