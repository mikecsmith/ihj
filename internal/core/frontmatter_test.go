package core

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/document"
)

func TestFrontmatterSchema_Validation(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Story"}, {Name: "Sub-task"}},
		Statuses: []string{"To Do", "Done"},
	}

	sch := FrontmatterSchema(ws, nil)

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

func TestBuildFrontmatterDoc_Roundtrip(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		body     string
	}{
		{
			name: "typical edit",
			metadata: map[string]string{
				"key": "ENG-42", "type": "Story", "priority": "High",
				"status": "In Progress", "summary": "Implement feature X",
			},
			body: "Some description here.",
		},
		{
			name: "create with empty summary",
			metadata: map[string]string{
				"type": "Task", "priority": "Medium", "status": "Backlog",
				"summary": "",
			},
		},
		{
			name: "special characters in summary",
			metadata: map[string]string{
				"type": "Story", "summary": "Fix: handle edge case #123",
			},
		},
		{
			name: "sprint field with active value",
			metadata: map[string]string{
				"type": "Task", "summary": "Sprint item", "sprint": "active",
			},
		},
		{
			name: "subtask with parent",
			metadata: map[string]string{
				"type": "Sub-task", "summary": "Child task",
				"parent": "ENG-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := BuildFrontmatterDoc("/tmp/schema.json", tt.metadata, tt.body)

			// Parse it back.
			got, gotBody, err := ParseFrontmatter(doc)
			if err != nil {
				t.Fatalf("ParseFrontmatter failed: %v", err)
			}

			// Body should roundtrip.
			if strings.TrimSpace(gotBody) != strings.TrimSpace(tt.body) {
				t.Errorf("body mismatch:\n  got:  %q\n  want: %q", gotBody, tt.body)
			}

			// Every metadata value should roundtrip.
			for k, want := range tt.metadata {
				if got[k] != want {
					t.Errorf("metadata[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

func TestBuildFrontmatterDoc_FieldOrder(t *testing.T) {
	metadata := map[string]string{
		"key": "ENG-1", "type": "Story", "priority": "High",
		"status": "In Progress", "parent": "ENG-0", "summary": "Test",
	}
	doc := BuildFrontmatterDoc("/tmp/s.json", metadata, "")

	// Extract YAML lines between the --- delimiters (skip schema comment).
	lines := strings.Split(doc, "\n")
	var yamlLines []string
	for _, l := range lines {
		if l == "---" || strings.HasPrefix(l, "#") || l == "" {
			continue
		}
		yamlLines = append(yamlLines, strings.SplitN(l, ":", 2)[0])
	}

	want := []string{"key", "type", "priority", "status", "parent", "summary"}
	if len(yamlLines) != len(want) {
		t.Fatalf("field count = %d, want %d: %v", len(yamlLines), len(want), yamlLines)
	}
	for i, w := range want {
		if yamlLines[i] != w {
			t.Errorf("field[%d] = %q, want %q (order: %v)", i, yamlLines[i], w, yamlLines)
			break
		}
	}
}

func TestBuildFrontmatterDoc_EmptySummaryFormat(t *testing.T) {
	doc := BuildFrontmatterDoc("/tmp/s.json", map[string]string{
		"type": "Task", "summary": "",
	}, "")

	// Should have "summary: " (trailing space, no null or "").
	if !strings.Contains(doc, "summary: \n") && !strings.HasSuffix(
		strings.SplitN(doc, "---", 3)[1], "summary: ") {
		// Just check it doesn't contain null or ""
		if strings.Contains(doc, "null") {
			t.Error("empty summary should not contain 'null'")
		}
		if strings.Contains(doc, `""`) {
			t.Error("empty summary should not contain '\"\"'")
		}
	}
}

func TestParseFrontmatter_BodyWithHorizontalRule(t *testing.T) {
	raw := "---\ntype: Story\nsummary: test\n---\n\nSome text\n\n---\n\nMore text after HR"
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm["summary"] != "test" {
		t.Errorf("summary = %q, want 'test'", fm["summary"])
	}
	if !strings.Contains(body, "---") {
		t.Error("body should preserve horizontal rules (---)")
	}
	if !strings.Contains(body, "More text after HR") {
		t.Error("body should contain text after horizontal rule")
	}
}

func TestParseFrontmatter_NilAndEmptyValues(t *testing.T) {
	// Bare key (no value) should parse as empty string, not "<nil>".
	raw := "---\nsummary:\ntype: Task\n---\n"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm["summary"] != "" {
		t.Errorf("bare key summary = %q, want empty string", fm["summary"])
	}
	if fm["type"] != "Task" {
		t.Errorf("type = %q, want 'Task'", fm["type"])
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	raw := "Just some text without frontmatter."
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm != nil {
		t.Errorf("expected nil metadata, got %v", fm)
	}
	if body != raw {
		t.Errorf("body = %q, want original text", body)
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

func TestWorkItemToMetadata(t *testing.T) {
	item := &WorkItem{
		ID: "ENG-42", Type: "Story", Status: "In Progress",
		Summary: "Test summary", ParentID: "ENG-1",
		Fields: map[string]any{"priority": "High", "other": "ignored"},
	}
	m := WorkItemToMetadata(item)

	checks := map[string]string{
		"key": "ENG-42", "type": "Story", "status": "In Progress",
		"summary": "Test summary", "parent": "ENG-1", "priority": "High",
	}
	for k, want := range checks {
		if m[k] != want {
			t.Errorf("metadata[%q] = %q, want %q", k, m[k], want)
		}
	}
	// "other" field should not appear — only priority is extracted.
	if _, ok := m["other"]; ok {
		t.Error("unexpected 'other' field in metadata")
	}
}

func TestWorkItemToMetadata_MinimalItem(t *testing.T) {
	item := &WorkItem{ID: "X-1", Type: "Task", Summary: "Minimal"}
	m := WorkItemToMetadata(item)

	if _, ok := m["parent"]; ok {
		t.Error("empty ParentID should not produce a 'parent' key")
	}
	if _, ok := m["priority"]; ok {
		t.Error("missing priority field should not produce a 'priority' key")
	}
}

func TestFrontmatterToWorkItem(t *testing.T) {
	fm := map[string]string{
		"summary": "New task", "type": "Story", "status": "To Do",
		"parent": "ENG-1", "priority": "High", "sprint": "active",
	}
	desc, _ := document.ParseMarkdownString("Some description")
	item := FrontmatterToWorkItem(fm, desc)

	if item.Summary != "New task" {
		t.Errorf("Summary = %q", item.Summary)
	}
	if item.Type != "Story" {
		t.Errorf("Type = %q", item.Type)
	}
	if item.ParentID != "ENG-1" {
		t.Errorf("ParentID = %q", item.ParentID)
	}
	if item.Description == nil {
		t.Error("Description should not be nil")
	}
	if item.Fields["priority"] != "High" {
		t.Errorf("priority = %v", item.Fields["priority"])
	}
	if item.Fields["sprint"] != "active" {
		t.Errorf("sprint = %v", item.Fields["sprint"])
	}
}

func TestFrontmatterToChanges(t *testing.T) {
	orig := &WorkItem{
		ID: "ENG-1", Type: "Story", Status: "To Do",
		Summary: "Original", ParentID: "ENG-0",
		Fields: map[string]any{"priority": "Medium"},
	}

	t.Run("no changes", func(t *testing.T) {
		fm := map[string]string{
			"summary": "Original", "type": "Story", "status": "To Do",
			"parent": "ENG-0", "priority": "Medium",
		}
		changes := FrontmatterToChanges(fm, nil, orig)
		if changes != nil {
			t.Errorf("expected nil changes, got %+v", changes)
		}
	})

	t.Run("summary changed", func(t *testing.T) {
		fm := map[string]string{
			"summary": "Updated", "type": "Story", "status": "To Do",
			"parent": "ENG-0", "priority": "Medium",
		}
		changes := FrontmatterToChanges(fm, nil, orig)
		if changes == nil {
			t.Fatal("expected changes")
		}
		if changes.Summary == nil || *changes.Summary != "Updated" {
			t.Errorf("Summary = %v", changes.Summary)
		}
		// Other fields should be nil (unchanged).
		if changes.Type != nil {
			t.Error("Type should be nil")
		}
	})

	t.Run("type case insensitive", func(t *testing.T) {
		fm := map[string]string{
			"summary": "Original", "type": "story", "status": "To Do",
			"parent": "ENG-0", "priority": "Medium",
		}
		changes := FrontmatterToChanges(fm, nil, orig)
		if changes != nil {
			t.Error("case-only type change should not be detected")
		}
	})

	t.Run("parent cleared", func(t *testing.T) {
		fm := map[string]string{
			"summary": "Original", "type": "Story", "status": "To Do",
			"parent": "", "priority": "Medium",
		}
		changes := FrontmatterToChanges(fm, nil, orig)
		if changes == nil {
			t.Fatal("expected changes")
		}
		if changes.ParentID == nil || *changes.ParentID != "" {
			t.Errorf("ParentID = %v, want empty string (cleared)", changes.ParentID)
		}
	})

	t.Run("priority changed", func(t *testing.T) {
		fm := map[string]string{
			"summary": "Original", "type": "Story", "status": "To Do",
			"parent": "ENG-0", "priority": "High",
		}
		changes := FrontmatterToChanges(fm, nil, orig)
		if changes == nil {
			t.Fatal("expected changes")
		}
		if changes.Fields["priority"] != "High" {
			t.Errorf("priority = %v", changes.Fields["priority"])
		}
	})
}
