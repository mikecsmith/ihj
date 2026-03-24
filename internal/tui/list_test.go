package tui

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
)

func testListModel(registry map[string]*jira.IssueView) ListModel {
	theme := DefaultTheme()
	styles := NewStyles(theme, nil)
	sw := map[string]int{"Open": 0, "To Do": 1, "In Progress": 2, "Done": 3}
	to := map[string]config.TypeOrderEntry{
		"10": {Order: 10, Color: "purple", HasChildren: true},
		"20": {Order: 20, Color: "blue"},
	}
	lm := NewListModel(registry, styles, sw, to)
	lm.SetSize(120, 40)
	return lm
}

func testListRegistry() map[string]*jira.IssueView {
	registry := map[string]*jira.IssueView{
		"TEST-1": {Key: "TEST-1", Summary: "Epic One", Type: "Epic", Status: "Open", Children: make(map[string]*jira.IssueView)},
		"TEST-2": {Key: "TEST-2", Summary: "Story One", Type: "Story", Status: "To Do", ParentKey: "TEST-1", Children: make(map[string]*jira.IssueView)},
	}
	jira.LinkChildren(registry)
	return registry
}

// --- buildTreePrefix ---

func TestBuildTreePrefix(t *testing.T) {
	tests := []struct {
		name   string
		depth  int
		isLast bool
		want   string
	}{
		{"root", 0, false, ""},
		{"depth 1 not last", 1, false, "  ├─ "},
		{"depth 1 last", 1, true, "  └─ "},
		{"depth 2 last", 2, true, "    └─ "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTreePrefix(tt.depth, nil, tt.isLast)
			if got != tt.want {
				t.Errorf("buildTreePrefix(%d, nil, %v) = %q; want %q", tt.depth, tt.isLast, got, tt.want)
			}
		})
	}
}

// --- SelectedIssue ---

func TestListSelectedIssue(t *testing.T) {
	t.Run("returns first at cursor 0", func(t *testing.T) {
		registry := testListRegistry()
		lm := testListModel(registry)
		iss := lm.SelectedIssue()
		if iss == nil {
			t.Fatal("SelectedIssue() = nil; want non-nil")
		}
		// Should be the first item after sorting and flattening.
		if iss.Key != lm.filtered[0].Issue.Key {
			t.Errorf("SelectedIssue().Key = %q; want %q", iss.Key, lm.filtered[0].Issue.Key)
		}
	})

	t.Run("nil on empty", func(t *testing.T) {
		registry := map[string]*jira.IssueView{}
		lm := testListModel(registry)
		if lm.SelectedIssue() != nil {
			t.Errorf("SelectedIssue() = %v; want nil on empty list", lm.SelectedIssue())
		}
	})
}

// --- ScrollList ---

func TestListScrollList(t *testing.T) {
	tests := []struct {
		name       string
		items      int
		startCur   int
		delta      int
		wantCursor int
	}{
		{"down", 3, 0, 1, 1},
		{"up clamps at 0", 3, 0, -1, 0},
		{"past end clamps", 3, 2, 5, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a registry with the desired number of root items.
			registry := make(map[string]*jira.IssueView)
			for i := range tt.items {
				key := "ITEM-" + string(rune('1'+i))
				registry[key] = &jira.IssueView{
					Key: key, Summary: "Item", Type: "Story", Status: "Open",
					Children: make(map[string]*jira.IssueView),
				}
			}
			lm := testListModel(registry)
			lm.cursor = tt.startCur
			lm.ScrollList(tt.delta)
			if lm.cursor != tt.wantCursor {
				t.Errorf("ScrollList(%d) cursor = %d; want %d", tt.delta, lm.cursor, tt.wantCursor)
			}
		})
	}
}

// --- Rebuild ---

func TestListRebuild(t *testing.T) {
	registry := testListRegistry()
	lm := testListModel(registry)
	initialCount := len(lm.allItems)

	// Add a new issue.
	registry["TEST-3"] = &jira.IssueView{
		Key: "TEST-3", Summary: "New Task", Type: "Story", Status: "Open",
		Children: make(map[string]*jira.IssueView),
	}
	lm.Rebuild(registry)

	if len(lm.allItems) <= initialCount {
		t.Errorf("Rebuild() allItems = %d; want > %d after adding issue", len(lm.allItems), initialCount)
	}
}

// --- applyFilter ---

func TestListApplyFilter(t *testing.T) {
	registry := testListRegistry()
	lm := testListModel(registry)
	totalBefore := len(lm.filtered)

	// Set search to "Epic" and re-filter.
	lm.search.SetValue("Epic")
	lm.applyFilter()

	if len(lm.filtered) >= totalBefore {
		t.Errorf("applyFilter(\"Epic\") filtered = %d; want < %d (should filter out non-matching)", len(lm.filtered), totalBefore)
	}
	if len(lm.filtered) == 0 {
		t.Error("applyFilter(\"Epic\") filtered = 0; want at least 1 match")
	}

	// Clear filter.
	lm.search.SetValue("")
	lm.applyFilter()
	if len(lm.filtered) != len(lm.allItems) {
		t.Errorf("applyFilter(\"\") filtered = %d; want %d (all items)", len(lm.filtered), len(lm.allItems))
	}
}
