package jira

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/config"
)

func TestBuildRegistry(t *testing.T) {
	issues := []client.Issue{
		testIssue("FOO-1", "Parent story", "Story", "10", "To Do", "High", ""),
		testIssue("FOO-2", "Child task", "Task", "11", "In Progress", "Medium", "FOO-1"),
	}

	reg := BuildRegistry(issues)

	if len(reg) != 2 {
		t.Fatalf("registry size = %d, want 2", len(reg))
	}

	v := reg["FOO-1"]
	if v.Summary != "Parent story" {
		t.Errorf("reg[\"FOO-1\"].Summary = %q; want \"Parent story\"", v.Summary)
	}
	if v.Type != "Story" {
		t.Errorf("reg[\"FOO-1\"].Type = %q; want \"Story\"", v.Type)
	}
	if v.Assignee != "Alice" {
		t.Errorf("reg[\"FOO-1\"].Assignee = %q; want \"Alice\"", v.Assignee)
	}
	if v.Created != "15 Mar 2024" {
		t.Errorf("reg[\"FOO-1\"].Created = %q; want \"15 Mar 2024\"", v.Created)
	}

	child := reg["FOO-2"]
	if child.ParentKey != "FOO-1" {
		t.Errorf("reg[\"FOO-2\"].ParentKey = %q; want \"FOO-1\"", child.ParentKey)
	}
}

func TestBuildRegistry_NilAssignee(t *testing.T) {
	iss := client.Issue{
		Key: "X-1",
		Fields: client.IssueFields{
			Summary:   "test",
			IssueType: client.IssueType{ID: "1", Name: "Task"},
			Status:    client.Status{Name: "Open"},
			Priority:  client.Priority{Name: "Medium"},
			Created:   "2024-01-01T00:00:00.000+0000",
			Updated:   "2024-01-01T00:00:00.000+0000",
		},
	}

	reg := BuildRegistry([]client.Issue{iss})
	if reg["X-1"].Assignee != "Unassigned" {
		t.Errorf("assignee = %q, want 'Unassigned'", reg["X-1"].Assignee)
	}
}

func TestLinkChildren(t *testing.T) {
	issues := []client.Issue{
		testIssue("P-1", "Parent", "Epic", "5", "Open", "High", ""),
		testIssue("P-2", "Child A", "Story", "10", "Open", "Medium", "P-1"),
		testIssue("P-3", "Child B", "Story", "10", "Open", "Low", "P-1"),
		testIssue("P-4", "Orphan", "Task", "11", "Open", "Medium", "MISSING-99"),
	}

	reg := BuildRegistry(issues)
	LinkChildren(reg)

	parent := reg["P-1"]
	if len(parent.Children) != 2 {
		t.Errorf("children count = %d, want 2", len(parent.Children))
	}

	roots := Roots(reg)
	// P-1 and P-4 (orphan parent not in registry) should be roots.
	if len(roots) != 2 {
		t.Errorf("roots = %d, want 2", len(roots))
	}
}

func TestSortViews(t *testing.T) {
	views := []*IssueView{
		{Key: "A-3", Status: "Done", TypeID: "10"},
		{Key: "A-1", Status: "To Do", TypeID: "10"},
		{Key: "A-2", Status: "To Do", TypeID: "5"},
	}

	weights := map[string]int{"to do": 0, "in progress": 1, "done": 2}
	typeOrder := map[string]config.TypeOrderEntry{
		"5":  {Order: 20},
		"10": {Order: 30},
	}

	SortViews(views, weights, typeOrder)

	expected := []string{"A-2", "A-1", "A-3"}
	for i, v := range views {
		if v.Key != expected[i] {
			t.Errorf("position %d: got %s, want %s", i, v.Key, expected[i])
		}
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-03-15T10:30:00.000+0000", "15 Mar 2024"},
		{"2024-01-01", "01 Jan 2024"},
		{"short", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatDate(tt.input)
		if got != tt.want {
			t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-03-15T14:30:00.000+0000", "15 Mar 2024, 14:30"},
		{"short", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatDateTime(tt.input)
		if got != tt.want {
			t.Errorf("formatDateTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
