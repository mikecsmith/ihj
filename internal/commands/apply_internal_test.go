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

func TestNormaliseUserField(t *testing.T) {
	assigneeDef := core.FieldDef{Key: "assignee", Type: core.FieldAssignee}
	emailDef := core.FieldDef{Key: "reporter", Type: core.FieldEmail}
	stringDef := core.FieldDef{Key: "priority", Type: core.FieldString}
	enumDef := core.FieldDef{Key: "severity", Type: core.FieldEnum}

	tests := []struct {
		name string
		def  core.FieldDef
		val  any
		want any
	}{
		// FieldAssignee sentinels → ""
		{"assignee: unassigned lowercase", assigneeDef, "unassigned", ""},
		{"assignee: UNASSIGNED uppercase", assigneeDef, "UNASSIGNED", ""},
		{"assignee: Unassigned mixed", assigneeDef, "Unassigned", ""},
		{"assignee: none lowercase", assigneeDef, "none", ""},
		{"assignee: NONE uppercase", assigneeDef, "NONE", ""},
		{"assignee: None mixed", assigneeDef, "None", ""},

		// FieldAssignee with real emails — passthrough
		{"assignee: email passthrough", assigneeDef, "alice@example.com", "alice@example.com"},
		{"assignee: empty string passthrough", assigneeDef, "", ""},

		// Non-assignee fields — sentinels NOT normalised
		{"email field: none passthrough", emailDef, "none", "none"},
		{"string field: unassigned passthrough", stringDef, "unassigned", "unassigned"},
		{"enum field: none passthrough", enumDef, "none", "none"},

		// Non-string value on assignee — passthrough
		{"assignee: nil passthrough", assigneeDef, nil, nil},
		{"assignee: int passthrough", assigneeDef, 42, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normaliseUserField(tt.def, tt.val)
			if got != tt.want {
				t.Errorf("normaliseUserField(%s, %v) = %v, want %v", tt.def.Type, tt.val, got, tt.want)
			}
		})
	}
}

func TestComputeDiff_FieldDefs(t *testing.T) {
	assigneeDef := core.FieldDef{
		Key: "assignee", Label: "Assignee", Type: core.FieldAssignee,
		Primary: true,
	}
	reporterDef := core.FieldDef{
		Key: "reporter", Label: "Reporter", Type: core.FieldEmail,
		// Not Primary — only exported with --full, but still diffable.
	}
	priorityDef := core.FieldDef{
		Key: "priority", Label: "Priority", Type: core.FieldEnum,
		Primary: true,
	}
	readOnlyDef := core.FieldDef{
		Key: "created", Label: "Created", Type: core.FieldString,
		Derived: true, Immutable: true, // Not diffable, only exported with --full.
	}
	defs := []core.FieldDef{assigneeDef, reporterDef, priorityDef, readOnlyDef}

	tests := []struct {
		name      string
		current   *core.WorkItem
		target    *core.WorkItem
		wantDiffs []FieldDiff
	}{
		{
			name: "assignee unassigned clears field (no diff when current is empty)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": ""},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "unassigned"},
			},
			wantDiffs: []FieldDiff{},
		},
		{
			name: "assignee none clears field (diff when current has value)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "alice@example.com"},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "none"},
			},
			wantDiffs: []FieldDiff{
				{Field: "Assignee", Old: "alice@example.com", New: ""},
			},
		},
		{
			name: "assignee UNASSIGNED clears field (case insensitive)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "bob@example.com"},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"assignee": "UNASSIGNED"},
			},
			wantDiffs: []FieldDiff{
				{Field: "Assignee", Old: "bob@example.com", New: ""},
			},
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
			name: "nil target field skipped (not in manifest)",
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
					// assignee and reporter omitted — should NOT appear as diffs.
				},
			},
			wantDiffs: []FieldDiff{},
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
			wantDiffs: []FieldDiff{},
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
		{
			name: "reporter none is NOT normalised (only FieldAssignee)",
			current: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"reporter": "alice@example.com"},
			},
			target: &core.WorkItem{
				Summary: "Test", Type: "Task", Status: "To Do",
				Fields: map[string]any{"reporter": "none"},
			},
			wantDiffs: []FieldDiff{
				{Field: "Reporter", Old: "alice@example.com", New: "none"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeDiff(tt.current, tt.target, "", defs)
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

func TestComputeDiff(t *testing.T) {
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
			want:      []FieldDiff{},
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
				Summary: "Original Summary",
				Type:    "Task",
				Status:  "To Do",
				// The YAML contains an asterisk bullet and extra blank lines.
				// Our AST normalizer should realize this is semantically identical.
				Description: mustParseMarkdown("* Bullet 1\n\n"),
			},
			parentKey: "EPIC-1",
			want:      []FieldDiff{}, // We expect exactly ZERO diffs!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeDiff(tt.current, tt.target, tt.parentKey, nil)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d changes, got %d", len(tt.want), len(got))
			}
			for i, w := range tt.want {
				if got[i].Field != w.Field || got[i].Old != w.Old || got[i].New != w.New {
					t.Errorf("change %d mismatch: got %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}
