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

// ─────────────────────────────────────────────────────────────
// flattenTree
// ─────────────────────────────────────────────────────────────

func TestFlattenTree_BasicHierarchy(t *testing.T) {
	child1 := &jira.IssueView{Key: "C-1", Summary: "Child 1", Type: "Task", Status: "To Do"}
	child2 := &jira.IssueView{Key: "C-2", Summary: "Child 2", Type: "Task", Status: "To Do"}
	parent := &jira.IssueView{
		Key:      "P-1",
		Summary:  "Parent",
		Type:     "Epic",
		Status:   "In Progress",
		Children: map[string]*jira.IssueView{"C-1": child1, "C-2": child2},
	}

	var items []listItem
	flattenTree([]*jira.IssueView{parent}, 0, nil, nil, &items, nil, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if items[0].Depth != 0 {
		t.Errorf("root depth should be 0, got %d", items[0].Depth)
	}

	if items[1].Depth != 1 {
		t.Errorf("first child depth should be 1, got %d", items[1].Depth)
	}

	if !items[2].IsLast {
		t.Errorf("items[2].IsLast = false; want true")
	}
}

func TestFlattenTree_MultipleRoots(t *testing.T) {
	r1 := &jira.IssueView{Key: "R-1", Summary: "Root 1", Type: "Epic", Status: "Done"}
	r2 := &jira.IssueView{Key: "R-2", Summary: "Root 2", Type: "Epic", Status: "Done"}

	var items []listItem
	flattenTree([]*jira.IssueView{r1, r2}, 0, nil, nil, &items, nil, nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Depth != 0 {
			t.Errorf("root item %s should have depth 0", item.Issue.Key)
		}
	}
}

func TestFlattenTree_AncestorTypes(t *testing.T) {
	// Epic → Story → Sub-task: ancestor types track the chain.
	subtask := &jira.IssueView{Key: "S-1", Summary: "Sub", Type: "Sub-task", Status: "To Do"}
	story := &jira.IssueView{
		Key: "ST-1", Summary: "Story", Type: "Story", Status: "To Do",
		Children: map[string]*jira.IssueView{"S-1": subtask},
	}
	epic := &jira.IssueView{
		Key: "E-1", Summary: "Epic", Type: "Epic", Status: "In Progress",
		Children: map[string]*jira.IssueView{"ST-1": story},
	}

	var items []listItem
	flattenTree([]*jira.IssueView{epic}, 0, nil, nil, &items, nil, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Root (Epic): no ancestors.
	if len(items[0].AncestorTypes) != 0 {
		t.Errorf("root AncestorTypes should be empty, got %v", items[0].AncestorTypes)
	}
	if items[0].ParentType != "" {
		t.Errorf("root ParentType should be empty, got %q", items[0].ParentType)
	}

	// Story: ancestor is Epic.
	if len(items[1].AncestorTypes) != 1 || items[1].AncestorTypes[0] != "Epic" {
		t.Errorf("story AncestorTypes should be [Epic], got %v", items[1].AncestorTypes)
	}
	if items[1].ParentType != "Epic" {
		t.Errorf("story ParentType should be Epic, got %q", items[1].ParentType)
	}

	// Sub-task: ancestors are [Epic, Story].
	if len(items[2].AncestorTypes) != 2 {
		t.Fatalf("subtask AncestorTypes should have 2 entries, got %v", items[2].AncestorTypes)
	}
	if items[2].AncestorTypes[0] != "Epic" || items[2].AncestorTypes[1] != "Story" {
		t.Errorf("subtask AncestorTypes should be [Epic, Story], got %v", items[2].AncestorTypes)
	}
	if items[2].ParentType != "Story" {
		t.Errorf("subtask ParentType should be Story, got %q", items[2].ParentType)
	}
}
