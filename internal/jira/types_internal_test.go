package jira

import (
	"encoding/json"
	"testing"
)

func TestIssueFields_UnmarshalJSON(t *testing.T) {
	raw := `{
		"summary": "Fix login bug",
		"issuetype": {"id": "10001", "name": "Bug", "subtask": false},
		"status": {"id": "3", "name": "In Progress", "statusCategory": {"id": 4, "key": "indeterminate", "name": "In Progress"}},
		"priority": {"id": "2", "name": "High"},
		"assignee": {"accountId": "abc123", "displayName": "Alice"},
		"reporter": {"accountId": "def456", "displayName": "Bob"},
		"parent": {"key": "PROJ-100", "id": "10050"},
		"labels": ["backend", "urgent"],
		"components": [{"id": "1", "name": "Auth"}],
		"created": "2024-03-15T10:30:00.000+0000",
		"updated": "2024-03-16T14:00:00.000+0000",
		"customfield_15000": "team-uuid-here",
		"customfield_10016": {"value": "3"}
	}`

	var fields issueFields
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if fields.Summary != "Fix login bug" {
		t.Errorf("summary = %q, want 'Fix login bug'", fields.Summary)
	}
	if fields.IssueType.Name != "Bug" {
		t.Errorf("issuetype.name = %q, want 'Bug'", fields.IssueType.Name)
	}
	if fields.IssueType.ID != "10001" {
		t.Errorf("issuetype.id = %q, want '10001'", fields.IssueType.ID)
	}
	if fields.Status.Name != "In Progress" {
		t.Errorf("Status.Name = %q; want \"In Progress\"", fields.Status.Name)
	}
	if fields.Status.StatusCategory.Key != "indeterminate" {
		t.Errorf("Status.StatusCategory.Key = %q; want \"indeterminate\"", fields.Status.StatusCategory.Key)
	}
	if fields.Priority.Name != "High" {
		t.Errorf("Priority.Name = %q; want \"High\"", fields.Priority.Name)
	}
	if fields.Assignee == nil || fields.Assignee.DisplayName != "Alice" {
		t.Errorf("Assignee = %v; want DisplayName=Alice", fields.Assignee)
	}
	if fields.Reporter == nil || fields.Reporter.DisplayName != "Bob" {
		t.Errorf("Reporter = %v; want DisplayName=Bob", fields.Reporter)
	}
	if fields.Parent == nil || fields.Parent.Key != "PROJ-100" {
		t.Errorf("Parent = %v; want Key=PROJ-100", fields.Parent)
	}
	if len(fields.Labels) != 2 || fields.Labels[0] != "backend" {
		t.Errorf("Labels = %v; want [backend, urgent]", fields.Labels)
	}
	if len(fields.Components) != 1 || fields.Components[0].Name != "Auth" {
		t.Errorf("Components = %v; want [{Name:Auth}]", fields.Components)
	}

	// Custom fields should be captured.
	if len(fields.Customs) < 2 {
		t.Fatalf("expected at least 2 custom fields, got %d", len(fields.Customs))
	}
	if fields.CustomString("customfield_15000") != "team-uuid-here" {
		t.Errorf("CustomString(\"customfield_15000\") = %q; want \"team-uuid-here\"", fields.CustomString("customfield_15000"))
	}
	if fields.CustomString("customfield_10016") != "3" {
		t.Errorf("custom 10016 = %q, want '3'", fields.CustomString("customfield_10016"))
	}
}

func TestIssueFields_CustomString_Variants(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"plain string", `{"customfield_1": "hello"}`, "hello"},
		{"value object", `{"customfield_1": {"value": "world"}}`, "world"},
		{"name object", `{"customfield_1": {"name": "team-a"}}`, "team-a"},
		{"null field", `{"customfield_1": null}`, ""},
		{"missing field", `{}`, ""},
		{"number (not string)", `{"customfield_1": 42}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fields issueFields
			if err := json.Unmarshal([]byte(tt.json), &fields); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := fields.CustomString("customfield_1")
			if got != tt.want {
				t.Errorf("CustomString = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueFields_NilAssignee(t *testing.T) {
	raw := `{"summary": "test", "issuetype": {"id": "1", "name": "Task"}, "status": {"id": "1", "name": "Open"}, "priority": {"id": "3", "name": "Medium"}}`
	var fields issueFields
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}

	if fields.Assignee != nil {
		t.Errorf("Assignee = %v; want nil", fields.Assignee)
	}
	if got := fields.Assignee.DisplayNameOrDefault("Unassigned"); got != "Unassigned" {
		t.Errorf("DisplayNameOrDefault = %q, want 'Unassigned'", got)
	}
}

func TestUser_DisplayNameOrDefault(t *testing.T) {
	tests := []struct {
		u        *user
		fallback string
		want     string
	}{
		{nil, "N/A", "N/A"},
		{&user{}, "N/A", "N/A"},
		{&user{DisplayName: "Alice"}, "N/A", "Alice"},
	}
	for _, tt := range tests {
		got := tt.u.DisplayNameOrDefault(tt.fallback)
		if got != tt.want {
			t.Errorf("DisplayNameOrDefault(%v, %q) = %q, want %q", tt.u, tt.fallback, got, tt.want)
		}
	}
}

func TestIssue_FullUnmarshal(t *testing.T) {
	raw := `{
		"key": "PROJ-42",
		"id": "10042",
		"self": "https://jira.example.com/rest/api/3/issue/10042",
		"fields": {
			"summary": "Implement feature",
			"issuetype": {"id": "10", "name": "Story"},
			"status": {"id": "1", "name": "Open", "statusCategory": {"id": 2, "key": "new"}},
			"priority": {"id": "3", "name": "Medium"},
			"labels": [],
			"components": [],
			"created": "2024-01-01T00:00:00.000+0000",
			"updated": "2024-01-02T00:00:00.000+0000",
			"comment": {
				"comments": [
					{
						"id": "100",
						"author": {"accountId": "u1", "displayName": "Eve"},
						"body": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Looks good"}]}]},
						"created": "2024-01-01T12:00:00.000+0000"
					}
				],
				"total": 1,
				"maxResults": 50,
				"startAt": 0
			}
		}
	}`

	var iss issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if iss.Key != "PROJ-42" {
		t.Errorf("issue.Key = %q; want \"PROJ-42\"", iss.Key)
	}
	if iss.Fields.Summary != "Implement feature" {
		t.Errorf("issue.Fields.Summary = %q; want \"Implement feature\"", iss.Fields.Summary)
	}
	if iss.Fields.Comment == nil {
		t.Fatal("comment is nil")
	}
	if len(iss.Fields.Comment.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(iss.Fields.Comment.Comments))
	}
	c := iss.Fields.Comment.Comments[0]
	if c.Author == nil || c.Author.DisplayName != "Eve" {
		t.Errorf("comment.Author = %v; want DisplayName=Eve", c.Author)
	}
	if len(c.Body) == 0 {
		t.Errorf("comment.Body length = %d; want > 0", len(c.Body))
	}
}

func TestSearchResponse_Unmarshal(t *testing.T) {
	raw := `{
		"issues": [{"key": "A-1", "id": "1", "fields": {"summary": "one", "issuetype": {"id": "1", "name": "Task"}, "status": {"id": "1", "name": "Open"}, "priority": {"id": "3", "name": "Medium"}, "labels": [], "components": [], "created": "2024-01-01T00:00:00.000+0000", "updated": "2024-01-01T00:00:00.000+0000"}}],
		"total": 1,
		"isLast": true
	}`

	var resp searchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Issues) != 1 || resp.Issues[0].Key != "A-1" {
		t.Errorf("searchResponse.Issues = %v; want 1 issue with Key=A-1", resp.Issues)
	}
	if !resp.IsLast {
		t.Errorf("searchResponse.IsLast = %v; want true", resp.IsLast)
	}
}
