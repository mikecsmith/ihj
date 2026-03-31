package tui_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/tui"
)

func testBlackboxListModel(registry map[string]*core.WorkItem) tui.ListModel {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	sw := map[string]int{"open": 0, "to do": 1, "in progress": 2, "done": 3}
	to := map[string]core.TypeOrderEntry{
		"10": {Order: 10, Color: "purple", HasChildren: true},
		"20": {Order: 20, Color: "blue"},
	}
	lm := tui.NewListModel(registry, styles, sw, to)
	lm.SetSize(120, 40)
	return lm
}

func testBlackboxListRegistry() map[string]*core.WorkItem {
	registry := map[string]*core.WorkItem{
		"TEST-1": {ID: "TEST-1", Summary: "Epic One", Type: "Epic", Status: "Open"},
		"TEST-2": {ID: "TEST-2", Summary: "Story One", Type: "Story", Status: "To Do", ParentID: "TEST-1"},
	}
	core.LinkChildren(registry)
	return registry
}

func TestListSelectedIssue(t *testing.T) {
	t.Run("returns non-nil for populated list", func(t *testing.T) {
		registry := testBlackboxListRegistry()
		lm := testBlackboxListModel(registry)
		iss := lm.SelectedIssue()
		if iss == nil {
			t.Fatal("SelectedIssue() = nil; want non-nil")
		}
	})

	t.Run("nil on empty", func(t *testing.T) {
		registry := map[string]*core.WorkItem{}
		lm := testBlackboxListModel(registry)
		if lm.SelectedIssue() != nil {
			t.Errorf("SelectedIssue() = %v; want nil on empty list", lm.SelectedIssue())
		}
	})
}

func TestListScrollList(t *testing.T) {
	t.Run("scroll down changes selection", func(t *testing.T) {
		// Build a registry with multiple root items so scrolling is meaningful.
		registry := map[string]*core.WorkItem{
			"ITEM-1": {ID: "ITEM-1", Summary: "Item 1", Type: "Story", Status: "Open"},
			"ITEM-2": {ID: "ITEM-2", Summary: "Item 2", Type: "Story", Status: "Open"},
			"ITEM-3": {ID: "ITEM-3", Summary: "Item 3", Type: "Story", Status: "Open"},
		}
		lm := testBlackboxListModel(registry)

		first := lm.SelectedIssue()
		if first == nil {
			t.Fatal("SelectedIssue() = nil; want non-nil before scroll")
		}

		lm.ScrollList(1)
		second := lm.SelectedIssue()
		if second == nil {
			t.Fatal("SelectedIssue() = nil; want non-nil after scroll")
		}
		if first.ID == second.ID {
			t.Error("SelectedIssue() did not change after ScrollList(1)")
		}
	})

	t.Run("scroll past end clamps", func(t *testing.T) {
		registry := map[string]*core.WorkItem{
			"ITEM-1": {ID: "ITEM-1", Summary: "Item 1", Type: "Story", Status: "Open"},
			"ITEM-2": {ID: "ITEM-2", Summary: "Item 2", Type: "Story", Status: "Open"},
		}
		lm := testBlackboxListModel(registry)

		// Scroll way past the end.
		lm.ScrollList(100)
		iss := lm.SelectedIssue()
		if iss == nil {
			t.Fatal("SelectedIssue() = nil; want non-nil after over-scroll")
		}

		// Scrolling further should not panic and should stay clamped.
		lm.ScrollList(1)
		after := lm.SelectedIssue()
		if after == nil {
			t.Fatal("SelectedIssue() = nil after second scroll")
		}
		if iss.ID != after.ID {
			t.Errorf("SelectedIssue() changed after scrolling past end: %q -> %q", iss.ID, after.ID)
		}
	})

	t.Run("scroll up clamps at top", func(t *testing.T) {
		registry := map[string]*core.WorkItem{
			"ITEM-1": {ID: "ITEM-1", Summary: "Item 1", Type: "Story", Status: "Open"},
			"ITEM-2": {ID: "ITEM-2", Summary: "Item 2", Type: "Story", Status: "Open"},
		}
		lm := testBlackboxListModel(registry)

		first := lm.SelectedIssue()
		lm.ScrollList(-5)
		after := lm.SelectedIssue()
		if after == nil {
			t.Fatal("SelectedIssue() = nil after scroll up")
		}
		if first.ID != after.ID {
			t.Errorf("SelectedIssue() changed after scrolling up from top: %q -> %q", first.ID, after.ID)
		}
	})
}

func TestListRebuild(t *testing.T) {
	registry := testBlackboxListRegistry()
	lm := testBlackboxListModel(registry)

	// Verify we can select an issue before rebuild.
	before := lm.SelectedIssue()
	if before == nil {
		t.Fatal("SelectedIssue() = nil before rebuild")
	}

	// Add a new issue and rebuild.
	registry["TEST-3"] = &core.WorkItem{
		ID: "TEST-3", Summary: "New Task", Type: "Story", Status: "Open",
	}
	lm.Rebuild(registry)

	// SelectedIssue should still work after rebuild.
	after := lm.SelectedIssue()
	if after == nil {
		t.Fatal("SelectedIssue() = nil after rebuild")
	}

	// The new item should appear in the View output.
	view := lm.View()
	if !strings.Contains(view, "TEST-3") {
		t.Error("View() does not contain TEST-3 after Rebuild")
	}
}

func TestListView_ContainsIssueData(t *testing.T) {
	registry := testBlackboxListRegistry()
	lm := testBlackboxListModel(registry)

	view := lm.View()

	if !strings.Contains(view, "TEST-1") {
		t.Error("View() does not contain issue ID TEST-1")
	}
	if !strings.Contains(view, "TEST-2") {
		t.Error("View() does not contain issue ID TEST-2")
	}
	if !strings.Contains(view, "Epic One") {
		t.Error("View() does not contain summary 'Epic One'")
	}
	if !strings.Contains(view, "Story One") {
		t.Error("View() does not contain summary 'Story One'")
	}
}

func TestListView_UnassignedShowsEmDash(t *testing.T) {
	registry := map[string]*core.WorkItem{
		"T-1": {
			ID: "T-1", Summary: "Assigned", Type: "Task", Status: "Open",
			Fields:        map[string]any{"assignee": "alice@test.com"},
			DisplayFields: map[string]any{"assignee": "Alice"},
		},
		"T-2": {
			ID: "T-2", Summary: "Unassigned", Type: "Task", Status: "Open",
			Fields:        map[string]any{},
			DisplayFields: map[string]any{},
		},
	}
	core.LinkChildren(registry)
	lm := testBlackboxListModel(registry)

	view := stripANSI(lm.View())

	if !strings.Contains(view, "Alice") {
		t.Error("assigned item should show assignee name")
	}
	if !strings.Contains(view, "—") {
		t.Error("unassigned item should show em dash (—) placeholder")
	}
}
