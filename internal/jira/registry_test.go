package jira

import (
	"testing"
)

func TestIssuesToWorkItems(t *testing.T) {
	issues := []Issue{
		testIssue("FOO-1", "Parent story", "Story", "10", "To Do", "High", ""),
		testIssue("FOO-2", "Child task", "Task", "11", "In Progress", "Medium", "FOO-1"),
	}

	items := IssuesToWorkItems(issues)

	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}

	v := items[0]
	if v.ID != "FOO-1" {
		t.Errorf("items[0].ID = %q; want \"FOO-1\"", v.ID)
	}
	if v.Summary != "Parent story" {
		t.Errorf("items[0].Summary = %q; want \"Parent story\"", v.Summary)
	}
	if v.Type != "Story" {
		t.Errorf("items[0].Type = %q; want \"Story\"", v.Type)
	}
	if v.StringField("assignee") != "Alice" {
		t.Errorf("items[0].assignee = %q; want \"Alice\"", v.StringField("assignee"))
	}
	if v.StringField("created") != "15 Mar 2024" {
		t.Errorf("items[0].created = %q; want \"15 Mar 2024\"", v.StringField("created"))
	}

	child := items[1]
	if child.ParentID != "FOO-1" {
		t.Errorf("items[1].ParentID = %q; want \"FOO-1\"", child.ParentID)
	}
}

func TestIssuesToWorkItems_NilAssignee(t *testing.T) {
	iss := Issue{
		Key: "X-1",
		Fields: IssueFields{
			Summary:   "test",
			IssueType: IssueType{ID: "1", Name: "Task"},
			Status:    Status{Name: "Open"},
			Priority:  Priority{Name: "Medium"},
			Created:   "2024-01-01T00:00:00.000+0000",
			Updated:   "2024-01-01T00:00:00.000+0000",
		},
	}

	items := IssuesToWorkItems([]Issue{iss})
	if items[0].StringField("assignee") != "Unassigned" {
		t.Errorf("assignee = %q, want 'Unassigned'", items[0].StringField("assignee"))
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
