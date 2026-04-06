package commands

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

func mustParseMarkdown(s string) *document.Node {
	node, _ := document.ParseMarkdownString(s)
	return node
}

func TestFieldToString(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"nil returns empty", nil, ""},
		{"string passthrough", "hello", "hello"},
		{"empty string", "", ""},
		{"integer", 42, "42"},
		{"bool true", true, "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldToString(tt.val); got != tt.want {
				t.Errorf("fieldToString(%v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

func TestDiffItem_FieldDefs(t *testing.T) {
	assigneeDef := core.FieldDef{
		Key: "assignee", Label: "Assignee", Type: core.FieldAssignee,
		Primary: true,
	}
	reporterDef := core.FieldDef{
		Key: "reporter", Label: "Reporter", Type: core.FieldEmail,
	}
	priorityDef := core.FieldDef{
		Key: "priority", Label: "Priority", Type: core.FieldEnum,
		Primary: true,
	}
	readOnlyDef := core.FieldDef{
		Key: "created", Label: "Created", Type: core.FieldString,
		Derived: true, Immutable: true,
	}
	defs := []core.FieldDef{assigneeDef, reporterDef, priorityDef, readOnlyDef}

	// Sentinel normalization now happens in the decoder — these tests use
	// already-normalized values ("" instead of "unassigned"/"none").
	tests := []struct {
		name      string
		current   *core.WorkItem
		target    *core.WorkItem
		wantDiffs []FieldDiff
	}{
		{
			name: "assignee cleared (no diff when both empty)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": ""},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": ""},
			},
			wantDiffs: nil,
		},
		{
			name: "assignee email change produces diff",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "alice@example.com"},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "bob@example.com"},
			},
			wantDiffs: []FieldDiff{
				{Field: "Assignee", Old: "alice@example.com", New: "bob@example.com"},
			},
		},
		{
			name: "omitted target fields not diffed (manifest omit = no-op)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{
					"assignee": "alice@example.com",
					"reporter": "bob@example.com",
					"priority": "High",
					"created":  "2024-01-01",
				},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{
					"priority": "High",
				},
			},
			wantDiffs: nil,
		},
		{
			name: "readonly fields never diffed",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"created": "2024-01-01"},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"created": "2025-12-31"},
			},
			wantDiffs: nil,
		},
		{
			name: "nil current field with non-nil target shows diff",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "alice@example.com"},
			},
			wantDiffs: []FieldDiff{
				{Field: "Assignee", Old: "", New: "alice@example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := diffItem(tt.current, tt.target, "", defs)
			if err != nil {
				t.Fatalf("diffItem: %v", err)
			}
			if len(got) != len(tt.wantDiffs) {
				t.Fatalf("expected %d diffs, got %d: %+v", len(tt.wantDiffs), len(got), got)
			}
			for i, w := range tt.wantDiffs {
				if got[i].Field != w.Field || got[i].Old != w.Old || got[i].New != w.New {
					t.Errorf("diff %d: got %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestDiffItem_CoreFields(t *testing.T) {
	baseCurrent := &core.WorkItem{
		Summary:     "Original Summary",
		Type:        "Task",
		Status:      "To Do",
		ParentID:    "EPIC-1",
		Description: mustParseMarkdown("Original desc"),
	}

	tests := []struct {
		name      string
		current   *core.WorkItem
		target    *core.WorkItem
		parentKey string
		want      []FieldDiff
	}{
		{
			name:    "no changes",
			current: baseCurrent,
			target: &core.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				Description: mustParseMarkdown("Original desc"),
			},
			parentKey: "EPIC-1",
			want:      nil,
		},
		{
			name:    "description changed",
			current: baseCurrent,
			target: &core.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				Description: mustParseMarkdown("New markdown desc"),
			},
			parentKey: "EPIC-1",
			want: []FieldDiff{
				{Field: "Description", Old: "Original desc", New: "New markdown desc"},
			},
		},
		{
			name: "description unchanged (semantic AST match ignores formatting)",
			current: &core.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				ParentID:    "EPIC-1",
				Description: mustParseMarkdown("- Bullet 1"),
			},
			target: &core.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				Description: mustParseMarkdown("* Bullet 1\n\n"),
			},
			parentKey: "EPIC-1",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := diffItem(tt.current, tt.target, tt.parentKey, nil)
			if err != nil {
				t.Fatalf("diffItem: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d changes, got %d: %+v", len(tt.want), len(got), got)
			}
			for i, w := range tt.want {
				if got[i].Field != w.Field || got[i].Old != w.Old || got[i].New != w.New {
					t.Errorf("change %d mismatch: got %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}
