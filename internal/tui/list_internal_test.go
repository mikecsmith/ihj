package tui

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

// testItems builds a flat list of listItems from work items, simulating
// a simple tree with optional depth. No sorting or tree glyph metadata —
// just enough structure to exercise filterItems.
func testItems(items ...*core.WorkItem) []listItem {
	out := make([]listItem, len(items))
	for i, iss := range items {
		depth := 0
		if iss.ParentID != "" {
			depth = 1
		}
		out[i] = listItem{Issue: iss, Depth: depth}
	}
	return out
}

func TestFilterItems(t *testing.T) {
	tests := []struct {
		name      string
		items     []listItem
		query     string
		ownerKey  string
		wantN     int    // expected number of results
		wantID    string // if non-empty, assert first result has this ID
		wantReset bool
	}{
		{
			name: "empty query returns all items",
			items: testItems(
				&core.WorkItem{ID: "A-1", Summary: "First", Status: "Open", Type: "Story"},
				&core.WorkItem{ID: "A-2", Summary: "Second", Status: "Open", Type: "Story"},
			),
			query:     "",
			wantN:     2,
			wantReset: false,
		},
		{
			name: "whitespace query returns all items",
			items: testItems(
				&core.WorkItem{ID: "A-1", Summary: "First", Status: "Open", Type: "Story"},
			),
			query:     "   ",
			wantN:     1,
			wantReset: false,
		},
		{
			name: "match by ID",
			items: testItems(
				&core.WorkItem{ID: "FOO-1", Summary: "Alpha", Status: "Open", Type: "Story"},
				&core.WorkItem{ID: "BAR-2", Summary: "Beta", Status: "Open", Type: "Story"},
			),
			query:     "FOO",
			wantN:     1,
			wantID:    "FOO-1",
			wantReset: true,
		},
		{
			name: "match by summary",
			items: testItems(
				&core.WorkItem{ID: "X-1", Summary: "Implement login flow", Status: "Open", Type: "Story"},
				&core.WorkItem{ID: "X-2", Summary: "Fix database migration", Status: "Open", Type: "Bug"},
			),
			query:     "login",
			wantN:     1,
			wantID:    "X-1",
			wantReset: true,
		},
		{
			name: "match by status",
			items: testItems(
				&core.WorkItem{ID: "X-1", Summary: "One", Status: "In Progress", Type: "Story"},
				&core.WorkItem{ID: "X-2", Summary: "Two", Status: "Done", Type: "Story"},
			),
			query:     "Progress",
			wantN:     1,
			wantID:    "X-1",
			wantReset: true,
		},
		{
			name: "match by type",
			items: testItems(
				&core.WorkItem{ID: "X-1", Summary: "One", Status: "Open", Type: "Epic"},
				&core.WorkItem{ID: "X-2", Summary: "Two", Status: "Open", Type: "Bug"},
			),
			query:     "Epic",
			wantN:     1,
			wantID:    "X-1",
			wantReset: true,
		},
		{
			name: "match by owner via ownerKey",
			items: testItems(
				&core.WorkItem{ID: "X-1", Summary: "One", Status: "Open", Type: "Story",
					DisplayFields: map[string]any{"assignee": "Alice"}},
				&core.WorkItem{ID: "X-2", Summary: "Two", Status: "Open", Type: "Story",
					DisplayFields: map[string]any{"assignee": "Bob"}},
			),
			query:     "Alice",
			ownerKey:  "assignee",
			wantN:     1,
			wantID:    "X-1",
			wantReset: true,
		},
		{
			name: "no matches",
			items: testItems(
				&core.WorkItem{ID: "X-1", Summary: "Alpha", Status: "Open", Type: "Story"},
			),
			query:     "zzzzzzz",
			wantN:     0,
			wantReset: true,
		},
		{
			name: "no duplicates when item matches multiple fields",
			items: testItems(
				&core.WorkItem{ID: "BUG-1", Summary: "Bug in parser", Status: "Open", Type: "Bug"},
			),
			query:     "Bug",
			wantN:     1,
			wantID:    "BUG-1",
			wantReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterItems(tt.items, tt.query, tt.ownerKey)

			if got := len(result.items); got != tt.wantN {
				t.Fatalf("got %d items; want %d", got, tt.wantN)
			}
			if result.reset != tt.wantReset {
				t.Errorf("reset = %v; want %v", result.reset, tt.wantReset)
			}
			if tt.wantID != "" && result.items[0].Issue.ID != tt.wantID {
				t.Errorf("first item = %q; want %q", result.items[0].Issue.ID, tt.wantID)
			}
		})
	}
}

func TestFilterItems_ChildMatchedParentExcluded(t *testing.T) {
	parent := &core.WorkItem{ID: "EPIC-1", Summary: "Epic", Status: "Open", Type: "Epic"}
	child := &core.WorkItem{ID: "STORY-1", Summary: "Unique child story", Status: "Open", Type: "Story", ParentID: "EPIC-1"}
	items := testItems(parent, child)

	result := filterItems(items, "Unique child", "")

	if len(result.items) != 1 {
		t.Fatalf("got %d items; want 1 (only matched child)", len(result.items))
	}
	if result.items[0].Issue.ID != "STORY-1" {
		t.Errorf("item = %q; want STORY-1", result.items[0].Issue.ID)
	}
}

func TestFilterItems_TreeMetadataStripped(t *testing.T) {
	items := []listItem{
		{
			Issue:         &core.WorkItem{ID: "X-1", Summary: "Match me", Status: "Open", Type: "Story"},
			Depth:         2,
			IsLast:        true,
			Ancestors:     []bool{false, true},
			AncestorTypes: []string{"Epic", "Story"},
		},
	}

	result := filterItems(items, "Match", "")

	if len(result.items) != 1 {
		t.Fatalf("got %d items; want 1", len(result.items))
	}
	item := result.items[0]
	if item.Depth != 0 {
		t.Errorf("Depth = %d; want 0", item.Depth)
	}
	if item.IsLast {
		t.Error("IsLast should be false")
	}
	if item.Ancestors != nil {
		t.Error("Ancestors should be nil")
	}
	if item.AncestorTypes != nil {
		t.Error("AncestorTypes should be nil")
	}
}
