package jira

import (
	"strings"
	"testing"
)

func TestBuildBootstrapFilters_Scrum(t *testing.T) {
	filters := buildBootstrapFilters("scrum", `"To Do", "In Progress", "Done"`)

	active, ok := filters["active"]
	if !ok {
		t.Fatal("missing 'active' filter")
	}
	if !strings.Contains(active, "openSprints()") {
		t.Errorf("scrum active filter should reference openSprints(), got: %s", active)
	}
	if !strings.Contains(active, "NOT IN futureSprints()") {
		t.Errorf("scrum active filter should exclude future sprints, got: %s", active)
	}
	if strings.Contains(active, "status IN") {
		t.Errorf("scrum active filter should not use status IN, got: %s", active)
	}
	if !strings.Contains(active, "resolved >= -2w") {
		t.Errorf("scrum active filter should include resolved window, got: %s", active)
	}

	backlog, ok := filters["backlog"]
	if !ok {
		t.Fatal("scrum boards should have a 'backlog' filter")
	}
	if !strings.Contains(backlog, "sprint") {
		t.Errorf("backlog filter should reference sprint, got: %s", backlog)
	}
}

func TestBuildBootstrapFilters_Kanban(t *testing.T) {
	statusJQL := `"To Do", "In Progress", "Done"`
	filters := buildBootstrapFilters("kanban", statusJQL)

	active, ok := filters["active"]
	if !ok {
		t.Fatal("missing 'active' filter")
	}
	if strings.Contains(active, "sprint") {
		t.Errorf("kanban active filter must not reference sprints, got: %s", active)
	}
	if !strings.Contains(active, "status IN") {
		t.Errorf("kanban active filter should use status IN, got: %s", active)
	}
	if !strings.Contains(active, "resolved >= -2w") {
		t.Errorf("kanban active filter should include resolved window, got: %s", active)
	}
	if _, ok := filters["backlog"]; ok {
		t.Error("kanban boards should not have a 'backlog' filter")
	}
}

func TestBuildBootstrapFilters_Simple(t *testing.T) {
	// "simple" boards should behave like kanban (no sprints).
	filters := buildBootstrapFilters("simple", `"Open", "Closed"`)

	active := filters["active"]
	if strings.Contains(active, "sprint") {
		t.Errorf("simple board filter must not reference sprints, got: %s", active)
	}
	if !strings.Contains(active, "status IN") {
		t.Errorf("simple board filter should use status IN, got: %s", active)
	}
}

func TestInferStatusColor(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		// Terminal — green.
		{"Done", "green"},
		{"Closed", "green"},
		{"Resolved", "green"},
		{"Complete", "green"},

		// Blocked — red.
		{"Blocked", "red"},
		{"Stopped", "red"},
		{"On Hold", "red"},
		{"Cancelled", "red"},

		// Review — magenta (must beat "ready" → cyan).
		{"In Review", "magenta"},
		{"Ready for Review", "magenta"},
		{"Ready for QA", "magenta"},
		{"In Test", "magenta"},
		{"QA", "magenta"},

		// Active — blue.
		{"In Progress", "blue"},
		{"Doing", "blue"},
		{"Active Development", "blue"},

		// Ready / refined — cyan.
		{"Ready to start", "cyan"},
		{"Ready For Refinement", "cyan"},
		{"Refinement", "cyan"},
		{"Approved", "cyan"},

		// Triage / intake — dim.
		{"Intake", "dim"},
		{"Triage", "dim"},
		{"Discovery", "dim"},
		{"Assessment", "dim"},
		{"New", "dim"},
		{"Open", "dim"},

		// Waiting — dim.
		{"Waiting for customer", "dim"},
		{"Pending approval", "dim"},

		// Backlog / planning — white.
		{"Backlog", "white"},
		{"To Do", "white"},
		{"Prioritisation", "white"},
		{"Selected for Development", "blue"}, // contains "dev" → active

		// Unknown — default.
		{"Something Unusual", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := inferStatusColor(tt.status)
			if got != tt.want {
				t.Errorf("inferStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestBuildStatusesList(t *testing.T) {
	columns := []string{"Backlog", "In Progress", "In Review", "Done"}
	result := buildStatusesList(columns)

	if len(result) != 4 {
		t.Fatalf("len = %d, want 4", len(result))
	}

	// Orders are sequential multiples of 10.
	for i, s := range result {
		wantOrder := (i + 1) * 10
		if s.Order != wantOrder {
			t.Errorf("result[%d].Order = %d, want %d", i, s.Order, wantOrder)
		}
		if s.Name != columns[i] {
			t.Errorf("result[%d].Name = %q, want %q", i, s.Name, columns[i])
		}
	}

	// Spot-check colors.
	if result[0].Color != "white" { // Backlog
		t.Errorf("Backlog color = %q, want white", result[0].Color)
	}
	if result[1].Color != "blue" { // In Progress
		t.Errorf("In Progress color = %q, want blue", result[1].Color)
	}
	if result[3].Color != "green" { // Done
		t.Errorf("Done color = %q, want green", result[3].Color)
	}
}

func TestBuildBootstrapFilters_CommonFilters(t *testing.T) {
	for _, boardType := range []string{"scrum", "kanban", "simple"} {
		filters := buildBootstrapFilters(boardType, `"Open"`)

		if _, ok := filters["all"]; !ok {
			t.Errorf("%s: missing 'all' filter", boardType)
		}
		if filters["all"] != "" {
			t.Errorf("%s: 'all' filter should be empty, got: %s", boardType, filters["all"])
		}

		me, ok := filters["me"]
		if !ok {
			t.Errorf("%s: missing 'me' filter", boardType)
		}
		if !strings.Contains(me, "currentUser()") {
			t.Errorf("%s: 'me' filter should reference currentUser(), got: %s", boardType, me)
		}
	}
}
