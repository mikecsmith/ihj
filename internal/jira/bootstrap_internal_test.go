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
	if strings.Contains(active, "status IN") {
		t.Errorf("scrum active filter should not use status IN, got: %s", active)
	}
	if !strings.Contains(active, "resolved >= -2w") {
		t.Errorf("scrum active filter should include resolved window, got: %s", active)
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
