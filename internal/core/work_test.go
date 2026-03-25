package core

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/config"
)

func TestFrontmatterSchema_Validation(t *testing.T) {
	board := &config.BoardConfig{
		Types:       []config.IssueTypeConfig{{Name: "Story"}, {Name: "Sub-task"}},
		Transitions: []string{"To Do", "Done"},
	}
	cfg := &config.Config{CustomFields: map[string]int{"team": 123}}

	// Look how clean this is now! We get the schema natively.
	sch := FrontmatterSchema(cfg, board)

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
	board := &config.BoardConfig{
		Types:       []config.IssueTypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Transitions: []string{"Backlog", "Done"},
	}

	sch := ManifestSchema(board)

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
