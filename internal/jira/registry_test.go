package jira

import (
	"encoding/json"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestIssuesToWorkItems_TypeSpecificCustomFields(t *testing.T) {
	// Two issue types: Story has "story_points", Bug has "bug_details".
	// Both issues carry values for BOTH custom fields in their Customs map
	// (Jira returns all requested fields). Only the type-specific binding
	// should be extracted.

	storyFields := &issueFields{
		Summary:   "A story",
		IssueType: issueType{ID: "10", Name: "Story"},
		Status:    status{Name: "To Do"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"5"`),
			"customfield_10002": json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"bug template"}]}]}`),
		},
	}
	bugFields := &issueFields{
		Summary:   "A bug",
		IssueType: issueType{ID: "11", Name: "Bug"},
		Status:    status{Name: "Open"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"3"`),
			"customfield_10002": json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"real bug details"}]}]}`),
		},
	}

	issues := []issue{
		{Key: "PROJ-1", Fields: *storyFields},
		{Key: "PROJ-2", Fields: *bugFields},
	}

	byType := map[string]map[string]customFieldBinding{
		"Story": {
			"customfield_10001": {Alias: "story_points", Type: core.FieldString},
		},
		"Bug": {
			"customfield_10002": {Alias: "bug_details", Type: core.FieldRichText},
		},
	}

	items := issuesToWorkItems(issues, nil, byType)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	story := items[0]
	bug := items[1]

	// Story should have story_points but NOT bug_details.
	if story.Fields["story_points"] != "5" {
		t.Errorf("story story_points = %v; want \"5\"", story.Fields["story_points"])
	}
	if _, ok := story.Fields["bug_details"]; ok {
		t.Error("story should NOT have bug_details — field belongs to Bug type")
	}

	// Bug should have bug_details but NOT story_points.
	if bug.Fields["bug_details"] == nil {
		t.Error("bug should have bug_details")
	}
	if _, ok := bug.Fields["story_points"]; ok {
		t.Error("bug should NOT have story_points — field belongs to Story type")
	}
}

func TestIssuesToWorkItems_UnknownTypeGetsNoCustomFields(t *testing.T) {
	// An issue whose type isn't in the byType map should not pick up
	// any custom fields — graceful no-op, not a panic.
	fields := &issueFields{
		Summary:   "Mystery",
		IssueType: issueType{ID: "99", Name: "Unknown"},
		Status:    status{Name: "Open"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"should be ignored"`),
		},
	}

	byType := map[string]map[string]customFieldBinding{
		"Story": {
			"customfield_10001": {Alias: "story_points", Type: core.FieldString},
		},
	}

	items := issuesToWorkItems([]issue{{Key: "X-1", Fields: *fields}}, nil, byType)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if _, ok := items[0].Fields["story_points"]; ok {
		t.Error("unknown type should not get Story custom fields")
	}
}

func TestIssuesToWorkItems_SharedFieldExtractedPerType(t *testing.T) {
	// Same Jira field ID aliased differently per type. Each issue should
	// get its type's alias, not the other type's.
	storyFields := &issueFields{
		Summary:   "Story",
		IssueType: issueType{ID: "10", Name: "Story"},
		Status:    status{Name: "To Do"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"High"`),
		},
	}
	bugFields := &issueFields{
		Summary:   "Bug",
		IssueType: issueType{ID: "11", Name: "Bug"},
		Status:    status{Name: "Open"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"Critical"`),
		},
	}

	byType := map[string]map[string]customFieldBinding{
		"Story": {
			"customfield_10001": {Alias: "severity", Type: core.FieldString},
		},
		"Bug": {
			"customfield_10001": {Alias: "impact", Type: core.FieldString},
		},
	}

	items := issuesToWorkItems(
		[]issue{
			{Key: "S-1", Fields: *storyFields},
			{Key: "B-1", Fields: *bugFields},
		},
		nil, byType,
	)

	story := items[0]
	bug := items[1]

	if story.Fields["severity"] != "High" {
		t.Errorf("story severity = %v; want \"High\"", story.Fields["severity"])
	}
	if _, ok := story.Fields["impact"]; ok {
		t.Error("story should not have Bug's 'impact' alias")
	}

	if bug.Fields["impact"] != "Critical" {
		t.Errorf("bug impact = %v; want \"Critical\"", bug.Fields["impact"])
	}
	if _, ok := bug.Fields["severity"]; ok {
		t.Error("bug should not have Story's 'severity' alias")
	}
}
