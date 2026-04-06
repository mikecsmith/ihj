package encoding_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/encoding"
)

func TestFrontmatterSchema_Validation(t *testing.T) {
	ws := &core.Workspace{
		Types:    []core.TypeConfig{{Name: "Story"}, {Name: "Sub-task"}},
		Statuses: []core.StatusConfig{{Name: "To Do", Order: 10, Color: "default"}, {Name: "Done", Order: 20, Color: "green"}},
	}

	sch := encoding.FrontmatterSchema(ws, nil)

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// Valid Story should pass.
	validInst := map[string]any{
		"summary": "Test Story",
		"type":    "Story",
		"status":  "To Do",
	}
	if err := resolved.Validate(validInst); err != nil {
		t.Errorf("Expected valid instance to pass, got error: %v", err)
	}
}

func TestBuildFrontmatterDoc_Roundtrip(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true, Role: core.RoleUrgency},
		{Key: "sprint", Label: "Sprint", Type: core.FieldEnum, Primary: true},
	}

	tests := []struct {
		name string
		item *core.WorkItem
		body string
	}{
		{
			name: "typical edit",
			item: &core.WorkItem{
				ID: "ENG-42", Type: "Story", Status: "In Progress",
				Summary: "Implement feature X",
				Fields:  map[string]any{"priority": "High"},
			},
			body: "Some description here.",
		},
		{
			name: "create with empty summary",
			item: &core.WorkItem{
				Type: "Task", Status: "Backlog",
				Fields: map[string]any{"priority": "Medium"},
			},
		},
		{
			name: "subtask with parent",
			item: &core.WorkItem{
				Type: "Sub-task", Summary: "Child task",
				ParentID: "ENG-1",
				Fields:   map[string]any{},
			},
		},
		{
			name: "sprint field",
			item: &core.WorkItem{
				Type: "Task", Summary: "Sprint item",
				Fields: map[string]any{"sprint": "active"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := encoding.BuildFrontmatterDoc("/tmp/schema.json", tt.item, defs, tt.body)

			got, gotBody, err := encoding.ParseFrontmatter(doc, defs)
			if err != nil {
				t.Fatalf("ParseFrontmatter failed: %v", err)
			}

			if strings.TrimSpace(gotBody) != strings.TrimSpace(tt.body) {
				t.Errorf("body mismatch:\n  got:  %q\n  want: %q", gotBody, tt.body)
			}

			if got.Type != tt.item.Type {
				t.Errorf("type = %q, want %q", got.Type, tt.item.Type)
			}
			if got.Summary != tt.item.Summary {
				t.Errorf("summary = %q, want %q", got.Summary, tt.item.Summary)
			}
			if got.ParentID != tt.item.ParentID {
				t.Errorf("parent = %q, want %q", got.ParentID, tt.item.ParentID)
			}
		})
	}
}

func TestBuildFrontmatterDoc_FieldOrder(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
	}
	item := &core.WorkItem{
		ID: "ENG-1", Type: "Story", Status: "In Progress",
		Summary: "Test", ParentID: "ENG-0",
		Fields: map[string]any{"priority": "High"},
	}
	doc := encoding.BuildFrontmatterDoc("/tmp/s.json", item, defs, "")

	lines := strings.Split(doc, "\n")
	var yamlLines []string
	for _, l := range lines {
		if l == "---" || strings.HasPrefix(l, "#") || l == "" {
			continue
		}
		yamlLines = append(yamlLines, strings.SplitN(l, ":", 2)[0])
	}

	want := []string{"key", "type", "status", "parent", "priority", "summary"}
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
	item := &core.WorkItem{Type: "Task", Fields: map[string]any{}}
	doc := encoding.BuildFrontmatterDoc("/tmp/s.json", item, nil, "")

	if strings.Contains(doc, "null") {
		t.Error("empty summary should not contain 'null'")
	}
}

func TestParseFrontmatter_BodyWithHorizontalRule(t *testing.T) {
	raw := "---\ntype: Story\nsummary: test\n---\n\nSome text\n\n---\n\nMore text after HR"
	item, body, err := encoding.ParseFrontmatter(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Summary != "test" {
		t.Errorf("summary = %q, want 'test'", item.Summary)
	}
	if !strings.Contains(body, "---") {
		t.Error("body should preserve horizontal rules (---)")
	}
	if !strings.Contains(body, "More text after HR") {
		t.Error("body should contain text after horizontal rule")
	}
}

func TestParseFrontmatter_NilAndEmptyValues(t *testing.T) {
	raw := "---\nsummary:\ntype: Task\n---\n"
	item, _, err := encoding.ParseFrontmatter(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Summary != "" {
		t.Errorf("bare key summary = %q, want empty string", item.Summary)
	}
	if item.Type != "Task" {
		t.Errorf("type = %q, want 'Task'", item.Type)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	raw := "Just some text without frontmatter."
	item, body, err := encoding.ParseFrontmatter(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Summary != "" || item.Type != "" {
		t.Errorf("expected empty item, got summary=%q type=%q", item.Summary, item.Type)
	}
	if body != raw {
		t.Errorf("body = %q, want original text", body)
	}
}

func TestValidateFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		item *core.WorkItem
		want string
	}{
		{"valid", &core.WorkItem{Summary: "test", Type: "Story"}, ""},
		{"missing summary", &core.WorkItem{Type: "Story"}, "Summary is required."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoding.ValidateFrontmatter(tt.item)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatter_FieldsBag(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
		{Key: "sprint", Label: "Sprint", Type: core.FieldEnum},
	}
	raw := "---\ntype: Story\nsummary: test\npriority: High\nfields:\n  sprint: active\n---\n"
	item, _, err := encoding.ParseFrontmatter(raw, defs)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if item.Fields["priority"] != "High" {
		t.Errorf("priority = %v, want High", item.Fields["priority"])
	}
	if item.Fields["sprint"] != "active" {
		t.Errorf("sprint = %v, want active", item.Fields["sprint"])
	}
}

func TestFrontmatter_RichTextRoundtrip(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "acceptance", Label: "Acceptance", Type: core.FieldRichText, Primary: true, Required: true},
	}
	body := "## Context\n\n- condition one\n- condition two"
	node, err := document.ParseMarkdownString(body)
	if err != nil {
		t.Fatalf("ParseMarkdownString: %v", err)
	}
	item := &core.WorkItem{
		ID: "ENG-1", Type: "Story", Status: "To Do", Summary: "Test",
		Fields: map[string]any{"acceptance": node},
	}

	// 1. BuildFrontmatterDoc emits block literal for multi-line content.
	doc := encoding.BuildFrontmatterDoc("/tmp/schema.json", item, defs, "")
	if !strings.Contains(doc, "acceptance: |") && !strings.Contains(doc, "acceptance: |-") {
		t.Errorf("expected block literal for multi-line acceptance field, got:\n%s", doc)
	}

	// 2. ParseFrontmatter parses it back with typed coercion.
	round, _, err := encoding.ParseFrontmatter(doc, defs)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	gotNode, ok := round.Fields["acceptance"].(*document.Node)
	if !ok || gotNode == nil {
		t.Fatalf("expected *document.Node in roundtripped Fields, got %T", round.Fields["acceptance"])
	}
	gotMD := strings.TrimSpace(document.RenderMarkdown(gotNode))
	wantMD := strings.TrimSpace(document.RenderMarkdown(node))
	if gotMD != wantMD {
		t.Errorf("RichText roundtrip mismatch:\n  want: %q\n  got:  %q", wantMD, gotMD)
	}
}

func TestFrontmatter_StringArrayRoundtrip(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "labels", Label: "Labels", Type: core.FieldStringArray, Primary: true},
	}
	item := &core.WorkItem{
		Type: "Story", Summary: "Test",
		Fields: map[string]any{"labels": []string{"security", "q1"}},
	}

	doc := encoding.BuildFrontmatterDoc("/tmp/schema.json", item, defs, "")

	// Should be a YAML array, not comma-delimited.
	if strings.Contains(doc, "security, q1") {
		t.Error("labels should be YAML array, not comma-delimited")
	}

	round, _, err := encoding.ParseFrontmatter(doc, defs)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	labels, ok := round.Fields["labels"].([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", round.Fields["labels"])
	}
	if len(labels) != 2 || labels[0] != "security" || labels[1] != "q1" {
		t.Errorf("labels = %v, want [security q1]", labels)
	}
}

func TestFrontmatter_AssigneeRoundtrip(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "assignee", Label: "Assignee", Type: core.FieldAssignee, Primary: true},
	}
	item := &core.WorkItem{
		Type: "Story", Summary: "Test",
		Fields: map[string]any{"assignee": "alice@example.com"},
	}

	doc := encoding.BuildFrontmatterDoc("/tmp/schema.json", item, defs, "")
	round, _, err := encoding.ParseFrontmatter(doc, defs)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if round.Fields["assignee"] != "alice@example.com" {
		t.Errorf("assignee = %v, want alice@example.com", round.Fields["assignee"])
	}
}
