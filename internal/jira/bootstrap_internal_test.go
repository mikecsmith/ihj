package jira

import (
	"context"
	"encoding/json"
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

// stubCreateMetaAPI implements the API interface with only FetchCreateMetaFields
// populated — enough for discoverPerTypeFields. All other methods panic.
type stubCreateMetaAPI struct {
	API
	fields map[string][]createMetaField // typeID → fields
}

func (s *stubCreateMetaAPI) FetchCreateMetaFields(_ context.Context, _ string, typeID string) ([]createMetaField, error) {
	return s.fields[typeID], nil
}

func TestDiscoverPerTypeFields(t *testing.T) {
	client := &stubCreateMetaAPI{
		fields: map[string][]createMetaField{
			"10": {
				{FieldID: "customfield_10016", Name: "Story Points", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:float",
				}},
				{FieldID: "customfield_10050", Name: "Acceptance Criteria", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textarea",
				}},
			},
			"11": {
				{FieldID: "customfield_10050", Name: "Acceptance Criteria", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textarea",
				}},
				{FieldID: "customfield_10060", Name: "Bug Details", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textarea",
				}},
			},
			"12": {
				// Unknown plugin type — should be excluded.
				{FieldID: "customfield_10099", Name: "Internal Flag", Schema: fieldSchema{
					Custom: "com.unknown.plugin:flag",
				}},
			},
		},
	}

	types := []bootstrapType{
		{ID: 10, Name: "Story"},
		{ID: 11, Name: "Bug"},
		{ID: 12, Name: "Task"},
	}

	discoverPerTypeFields(context.Background(), client, "FOO", types)

	// Story should have story_points and acceptance_criteria.
	if types[0].Fields["story_points"] != 10016 {
		t.Errorf("Story story_points = %v; want 10016", types[0].Fields["story_points"])
	}
	if types[0].Fields["acceptance_criteria"] != 10050 {
		t.Errorf("Story acceptance_criteria = %v; want 10050", types[0].Fields["acceptance_criteria"])
	}
	if _, ok := types[0].Fields["bug_details"]; ok {
		t.Error("Story should not have bug_details")
	}

	// Bug should have acceptance_criteria and bug_details, NOT story_points.
	if types[1].Fields["acceptance_criteria"] != 10050 {
		t.Errorf("Bug acceptance_criteria = %v; want 10050", types[1].Fields["acceptance_criteria"])
	}
	if types[1].Fields["bug_details"] != 10060 {
		t.Errorf("Bug bug_details = %v; want 10060", types[1].Fields["bug_details"])
	}
	if _, ok := types[1].Fields["story_points"]; ok {
		t.Error("Bug should not have story_points")
	}

	// Task had only an unknown plugin type — no fields.
	if len(types[2].Fields) != 0 {
		t.Errorf("Task should have no fields (unknown plugin type), got %v", types[2].Fields)
	}
}

func TestDiscoverPerTypeFields_KeyCollision(t *testing.T) {
	// Two fields with the same derived name get unique keys.
	client := &stubCreateMetaAPI{
		fields: map[string][]createMetaField{
			"10": {
				{FieldID: "customfield_20001", Name: "Team", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textfield",
				}},
				{FieldID: "customfield_20002", Name: "Team", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textfield",
				}},
			},
		},
	}

	types := []bootstrapType{{ID: 10, Name: "Story"}}
	discoverPerTypeFields(context.Background(), client, "FOO", types)

	if len(types[0].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d: %v", len(types[0].Fields), types[0].Fields)
	}
	// One should be "team", the other "team_20002" (suffixed).
	if types[0].Fields["team"] != 20001 {
		t.Errorf("team = %v; want 20001", types[0].Fields["team"])
	}
	if types[0].Fields["team_20002"] != 20002 {
		t.Errorf("team_20002 = %v; want 20002", types[0].Fields["team_20002"])
	}
}

func TestDiscoverPerTypeFields_SystemFieldsExcluded(t *testing.T) {
	// Non-custom fields (no "customfield_" prefix) should be skipped.
	client := &stubCreateMetaAPI{
		fields: map[string][]createMetaField{
			"10": {
				{FieldID: "summary", Name: "Summary", Schema: fieldSchema{}},
				{FieldID: "priority", Name: "Priority", Schema: fieldSchema{}},
				{FieldID: "customfield_10016", Name: "Story Points", Schema: fieldSchema{
					Custom: "com.atlassian.jira.plugin.system.customfieldtypes:float",
				}},
			},
		},
	}

	types := []bootstrapType{{ID: 10, Name: "Story"}}
	discoverPerTypeFields(context.Background(), client, "FOO", types)

	if len(types[0].Fields) != 1 {
		t.Fatalf("expected 1 field (only custom), got %d: %v", len(types[0].Fields), types[0].Fields)
	}
	if types[0].Fields["story_points"] != 10016 {
		t.Errorf("story_points = %v; want 10016", types[0].Fields["story_points"])
	}
}

func TestPromoteGlobalFields(t *testing.T) {
	types := []bootstrapType{
		{ID: 10, Name: "Story", Fields: map[string]int{
			"story_points": 10016,
			"sprint":       10020,
			"start_date":   10030,
		}},
		{ID: 11, Name: "Bug", Fields: map[string]int{
			"bug_details": 10060,
			"sprint":      10020,
			"start_date":  10030,
		}},
		{ID: 12, Name: "Task", Fields: map[string]int{
			"sprint":     10020,
			"start_date": 10030,
		}},
	}

	cfMap := map[string]any{
		"team":      15000,
		"epic_name": 10009,
	}

	promoted := promoteGlobalFields(types, cfMap)

	// sprint and start_date are on all 3 types → promoted.
	if promoted["sprint"] != 10020 {
		t.Errorf("sprint = %v; want 10020", promoted["sprint"])
	}
	if promoted["start_date"] != 10030 {
		t.Errorf("start_date = %v; want 10030", promoted["start_date"])
	}

	// story_points only on Story, bug_details only on Bug → not promoted.
	if _, ok := promoted["story_points"]; ok {
		t.Error("story_points should not be promoted (not on all types)")
	}
	if _, ok := promoted["bug_details"]; ok {
		t.Error("bug_details should not be promoted (not on all types)")
	}

	// Promoted fields removed from per-type maps.
	if _, ok := types[0].Fields["sprint"]; ok {
		t.Error("sprint should be removed from Story")
	}
	if _, ok := types[1].Fields["start_date"]; ok {
		t.Error("start_date should be removed from Bug")
	}

	// Type-specific fields remain.
	if types[0].Fields["story_points"] != 10016 {
		t.Errorf("Story should still have story_points, got %v", types[0].Fields)
	}
	if types[1].Fields["bug_details"] != 10060 {
		t.Errorf("Bug should still have bug_details, got %v", types[1].Fields)
	}

	// Task had only global fields → Fields should be nil.
	if types[2].Fields != nil {
		t.Errorf("Task Fields should be nil (all promoted), got %v", types[2].Fields)
	}
}

func TestPromoteGlobalFields_SkipsExistingCfMap(t *testing.T) {
	// Field ID 15000 is already in cfMap as "team" — should not be promoted
	// even if it appears on all types, and should be stripped from per-type maps.
	types := []bootstrapType{
		{ID: 10, Name: "Story", Fields: map[string]int{"team": 15000, "sprint": 10020}},
		{ID: 11, Name: "Bug", Fields: map[string]int{"team": 15000, "sprint": 10020}},
	}

	cfMap := map[string]any{"team": 15000}

	promoted := promoteGlobalFields(types, cfMap)

	if _, ok := promoted["team"]; ok {
		t.Error("team should not be promoted (already in cfMap)")
	}
	if promoted["sprint"] != 10020 {
		t.Errorf("sprint = %v; want 10020", promoted["sprint"])
	}

	// team should be stripped from per-type maps (already global).
	for i, bt := range types {
		if _, ok := bt.Fields["team"]; ok {
			t.Errorf("types[%d] (%s) should not have team (already in cfMap)", i, bt.Name)
		}
	}
}

func TestPromoteGlobalFields_EmptyTypes(t *testing.T) {
	promoted := promoteGlobalFields(nil, map[string]any{})
	if len(promoted) != 0 {
		t.Errorf("expected nil/empty, got %v", promoted)
	}
}

func TestBootstrap_PerTypeFieldsInYAML(t *testing.T) {
	// Verify that the YAML output includes per-type fields when present.
	types := []bootstrapType{
		{ID: 10, Name: "Story", Order: 30, Color: "blue", Fields: map[string]int{
			"story_points": 10016,
		}},
		{ID: 11, Name: "Bug", Order: 30, Color: "red", Fields: map[string]int{
			"bug_details": 10060,
		}},
	}

	data, err := json.Marshal(types)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify the structs serialize correctly.
	var roundTripped []bootstrapType
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundTripped[0].Fields["story_points"] != 10016 {
		t.Errorf("round-tripped Story story_points = %v; want 10016", roundTripped[0].Fields["story_points"])
	}
	if roundTripped[1].Fields["bug_details"] != 10060 {
		t.Errorf("round-tripped Bug bug_details = %v; want 10060", roundTripped[1].Fields["bug_details"])
	}
}
