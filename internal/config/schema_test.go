package config

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestFrontmatterSchema_Validation(t *testing.T) {
	board := &BoardConfig{
		Types:       []IssueTypeConfig{{Name: "Story"}, {Name: "Sub-task"}},
		Transitions: []string{"To Do", "Done"},
	}
	cfg := &Config{CustomFields: map[string]int{"team": 123}}

	schemaMap := FrontmatterSchema(cfg, board)
	schemaBytes, err := json.Marshal(schemaMap)
	if err != nil {
		t.Fatalf("Failed to marshal schema map: %v", err)
	}

	var sch jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &sch); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

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

func TestHierarchySchema_Validation(t *testing.T) {
	board := &BoardConfig{
		Types:       []IssueTypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Transitions: []string{"Backlog", "Done"},
	}

	schemaMap := HierarchySchema(board)
	schemaBytes, err := json.Marshal(schemaMap)
	if err != nil {
		t.Fatalf("Failed to marshal hierarchy schema map: %v", err)
	}

	var sch jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &sch); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// TEST: Valid Nested JSON payload
	validJSON := `[
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
	]`

	var inst any
	if err := json.Unmarshal([]byte(validJSON), &inst); err != nil {
		t.Fatalf("Failed to unmarshal valid JSON setup: %v", err)
	}

	if err := resolved.Validate(inst); err != nil {
		t.Errorf("Expected valid JSON to pass, got error: %v", err)
	}
}
