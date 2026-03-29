package core

import (
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
