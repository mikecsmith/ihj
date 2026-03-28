package core

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestFrontmatterSchema_Validation(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Story"}, {Name: "Sub-task"}},
		Statuses: []string{"To Do", "Done"},
	}

	sch := FrontmatterSchema(ws)

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// TEST 1: Valid Story
	validYAML := `
summary: "Test Story"
type: "Story"
priority: "High"
status: "To Do"
team: "true"
`
	var validInst any
	if err := yaml.Unmarshal([]byte(validYAML), &validInst); err != nil {
		t.Fatalf("Failed to unmarshal valid YAML setup: %v", err)
	}

	if err := resolved.Validate(validInst); err != nil {
		t.Errorf("Expected valid YAML to pass, got error: %v", err)
	}

	// TEST 2: Invalid Sub-task (Missing Parent)
	invalidYAML := `
summary: "Missing parent"
type: "Sub-task"
`
	var invalidInst any
	if err := yaml.Unmarshal([]byte(invalidYAML), &invalidInst); err != nil {
		t.Fatalf("Failed to unmarshal invalid YAML setup: %v", err)
	}

	if err := resolved.Validate(invalidInst); err == nil {
		t.Error("Expected invalid YAML (sub-task missing parent) to fail validation")
	}
}

func TestManifestSchema_Validation(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Statuses: []string{"Backlog", "Done"},
	}

	sch := ManifestSchema(ws)

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// TEST: Valid Nested JSON payload (using the new Manifest structure)
	validJSON := `{
		"metadata": {
			"backend": "jira",
			"target": "eng"
		},
		"items": [
			{
				"type": "Epic",
				"summary": "Main Epic",
				"status": "Backlog",
				"children": [
					{
						"type": "Story",
						"summary": "Child Story",
						"status": "Backlog"
					}
				]
			}
		]
	}`

	var inst any
	if err := json.Unmarshal([]byte(validJSON), &inst); err != nil {
		t.Fatalf("Failed to unmarshal valid JSON setup: %v", err)
	}

	if err := resolved.Validate(inst); err != nil {
		t.Errorf("Expected valid JSON to pass, got error: %v", err)
	}
}

func TestValidateFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		fm   map[string]string
		want string
	}{
		{"valid", map[string]string{"summary": "test", "type": "Story"}, ""},
		{"missing summary", map[string]string{"type": "Story"}, "Summary is required."},
		{"subtask no parent", map[string]string{"summary": "x", "type": "Sub-task"}, "Sub-tasks require a parent issue key."},
		{"subtask with parent", map[string]string{"summary": "x", "type": "Sub-task", "parent": "FOO-1"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateFrontmatter(tt.fm)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkItem_Hashing(t *testing.T) {
	item1 := &WorkItem{
		ID:      "ENG-1",
		Type:    "Story",
		Summary: "Hash Test",
		Status:  "To Do",
		Fields:  map[string]any{"priority": "High", "sprint": 1},
	}

	// 1. Test Determinism
	hashA := item1.ContentHash()
	hashB := item1.ContentHash()
	if hashA != hashB {
		t.Errorf("ContentHash is not deterministic: %s != %s", hashA, hashB)
	}

	// 2. Test Core Field Change Detection
	item1.Summary = "Updated Hash Test"
	hashC := item1.ContentHash()
	if hashA == hashC {
		t.Error("ContentHash did not change when Summary was updated")
	}

	// 3. Test Flex Bucket Change Detection
	item1.Summary = "Hash Test" // revert
	item1.Fields["priority"] = "Low"
	hashD := item1.ContentHash()
	if hashA == hashD {
		t.Error("ContentHash did not change when Fields map was updated")
	}

	// 4. Test StateHash (Idempotency)
	state1 := item1.StateHash("PARENT-A")
	state2 := item1.StateHash("PARENT-B")
	if state1 == state2 {
		t.Error("StateHash did not change when parentID was different")
	}

	// Ensure ID does NOT affect StateHash (since ID doesn't exist during creation)
	item1.ID = "NEW-ID-2"
	state3 := item1.StateHash("PARENT-A")
	if state1 != state3 {
		t.Error("StateHash should remain identical even if ID changes")
	}
}
