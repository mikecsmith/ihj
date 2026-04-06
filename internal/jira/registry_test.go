package jira

import (
	"encoding/json"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestIssuesToWorkItems_ExtractsAllCustomFields(t *testing.T) {
	// Extraction uses the union map — all known custom fields are
	// extracted regardless of issue type. Display-time filtering
	// (via TypeConfig.Fields) controls per-type visibility.
	fields := &issueFields{
		Summary:   "A story",
		IssueType: issueType{ID: "10", Name: "Story"},
		Status:    status{Name: "To Do"},
		Customs: map[string]json.RawMessage{
			"customfield_10001": json.RawMessage(`"5"`),
			"customfield_10002": json.RawMessage(`"some value"`),
		},
	}

	customFields := map[string]customFieldBinding{
		"customfield_10001": {Alias: "story_points", Type: core.FieldString},
		"customfield_10002": {Alias: "bug_details", Type: core.FieldString},
	}

	items := issuesToWorkItems([]issue{{Key: "S-1", Fields: *fields}}, nil, customFields)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	// Both fields extracted — extraction is broad on purpose.
	if items[0].Fields["story_points"] != "5" {
		t.Errorf("story_points = %v; want \"5\"", items[0].Fields["story_points"])
	}
	if items[0].Fields["bug_details"] != "some value" {
		t.Errorf("bug_details = %v; want \"some value\"", items[0].Fields["bug_details"])
	}
}

func TestIssuesToWorkItems_RichTextExtracted(t *testing.T) {
	fields := &issueFields{
		Summary:   "Has AC",
		IssueType: issueType{ID: "10", Name: "Story"},
		Status:    status{Name: "Open"},
		Customs: map[string]json.RawMessage{
			"customfield_10003": json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"criterion"}]}]}`),
		},
	}

	customFields := map[string]customFieldBinding{
		"customfield_10003": {Alias: "acceptance_criteria", Type: core.FieldRichText},
	}

	items := issuesToWorkItems([]issue{{Key: "S-1", Fields: *fields}}, nil, customFields)
	if items[0].Fields["acceptance_criteria"] == nil {
		t.Error("rich text field should be extracted as document node")
	}
}

func TestIssuesToWorkItems_MissingCustomFieldSkipped(t *testing.T) {
	// Custom field not present in Jira response — should not appear.
	fields := &issueFields{
		Summary:   "No customs",
		IssueType: issueType{ID: "10", Name: "Story"},
		Status:    status{Name: "Open"},
		Customs:   map[string]json.RawMessage{},
	}

	customFields := map[string]customFieldBinding{
		"customfield_10001": {Alias: "story_points", Type: core.FieldString},
	}

	items := issuesToWorkItems([]issue{{Key: "S-1", Fields: *fields}}, nil, customFields)
	if _, ok := items[0].Fields["story_points"]; ok {
		t.Error("field not in Jira response should not appear in Fields")
	}
}
