package commands

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/ui"
	"github.com/mikecsmith/ihj/internal/work"
)

func TestComputeDiff(t *testing.T) {
	baseCurrent := client.Issue{
		Fields: client.IssueFields{
			Summary:     "Original Summary",
			IssueType:   client.IssueType{Name: "Task"},
			Status:      client.Status{Name: "To Do"},
			Parent:      &client.ParentRef{Key: "EPIC-1"},
			Description: []byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Original desc"}]}]}`),
		},
	}

	tests := []struct {
		name      string
		current   *client.Issue
		target    *work.WorkItem
		parentKey string
		want      []ui.Change
	}{
		{
			name:    "no changes",
			current: &baseCurrent,
			target: &work.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				Description: "Original desc",
			},
			parentKey: "EPIC-1",
			want:      []ui.Change{},
		},
		{
			name:    "description changed (ADF to MD)",
			current: &baseCurrent,
			target: &work.WorkItem{
				Summary:     "Original Summary",
				Type:        "Task",
				Status:      "To Do",
				Description: "New markdown desc",
			},
			parentKey: "EPIC-1",
			want: []ui.Change{
				{Field: "Description", Old: "Original desc", New: "New markdown desc"},
			},
		},
		{
			name: "description unchanged (semantic AST match ignores formatting)",
			current: &client.Issue{
				Fields: client.IssueFields{
					Summary:   "Original Summary",
					IssueType: client.IssueType{Name: "Task"},
					Status:    client.Status{Name: "To Do"},
					Parent:    &client.ParentRef{Key: "EPIC-1"},
					// This ADF represents a bullet list.
					// Jira's renderer will output this as "- Bullet 1"
					Description: []byte(`{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Bullet 1"}]}]}]}]}`),
				},
			},
			target: &work.WorkItem{
				Summary: "Original Summary",
				Type:    "Task",
				Status:  "To Do",
				// The YAML contains an asterisk bullet and extra blank lines.
				// Our AST normalizer should realize this is semantically identical to the ADF above.
				Description: "* Bullet 1\n\n",
			},
			parentKey: "EPIC-1",
			want:      []ui.Change{}, // We expect exactly ZERO diffs!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDiff(tt.current, tt.target, tt.parentKey)
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
