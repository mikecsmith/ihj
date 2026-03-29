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
