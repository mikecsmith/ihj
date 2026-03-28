package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

// newTestModel creates a fully initialised AppModel for white-box tests.
// Only used by the integration tests below, which need to send internal
// message types into a running Bubble Tea program.
func newTestModel() AppModel {
	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	ui := &testutil.MockUI{}
	rt := testutil.NewTestRuntime(ui)
	provider := testutil.NewMockProvider()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := testutil.NewTestFactory(provider)

	m := NewAppModel(rt, wsSess, factory, ws, "default", items, time.Time{})
	m.width = 120
	m.height = 40
	m.ready = true
	m.cachedUserName = "Demo User"
	m.recalcLayout()
	m.syncDetail()
	return m
}

func newTestModelForTeatest() AppModel {
	m := newTestModel()
	// teatest handles window sizing via WithInitialTermSize, so reset
	// the ready flag so Init + WindowSizeMsg flows naturally.
	m.ready = false
	return m
}

// Integration tests using teatest v2
//
// These tests run a full Bubble Tea program in test mode and
// verify rendered output via WaitFor.

func TestTUI_InitialRender(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "Engineering") && strings.Contains(s, "TEST-1")
	}, teatest.WithDuration(3*time.Second))
}

func TestTUI_NotificationAppearsInOutput(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(transitionDoneMsg{issueKey: "TEST-1", newStatus: "Done"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Done")
	}, teatest.WithDuration(3*time.Second))
}

func TestTUI_TransitionPopup(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 't', Mod: tea.ModAlt})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "To Do") || strings.Contains(s, "In Progress") || strings.Contains(s, "Done")
	}, teatest.WithDuration(3*time.Second))
}

// Post-upsert merge tests
//
// These test mergeIssueIntoRegistry behavior by sending
// postUpsertCompleteMsg directly into the model and verifying
// the rendered list output.

// viewContainsID checks whether an issue ID appears in the rendered View.
func viewContainsID(m AppModel, id string) bool {
	return strings.Contains(m.View().Content, id)
}

func TestMerge_EditSetParent_ItemStaysVisible(t *testing.T) {
	// Regression: editing an item to set a parent caused it to vanish
	// because LinkChildren wasn't called on the edit path.
	m := newTestModel()

	if !viewContainsID(m, "TEST-1") || !viewContainsID(m, "TEST-2") {
		t.Fatal("setup: both items should be visible initially")
	}

	// Simulate editing TEST-1 to set parent=TEST-2.
	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-1", Summary: "Epic One", Type: "Epic",
			Status: "In Progress", ParentID: "TEST-2",
		},
		issueKey: "TEST-1",
		mode:     modeEdit,
	})
	m = result.(AppModel)

	// Both items should still be visible — TEST-1 as a child of TEST-2.
	if !viewContainsID(m, "TEST-1") {
		t.Error("TEST-1 should remain visible after setting parent to TEST-2")
	}
	if !viewContainsID(m, "TEST-2") {
		t.Error("TEST-2 (parent) should remain visible")
	}
}

func TestMerge_EditUpdateFields_ItemVisible(t *testing.T) {
	m := newTestModel()

	// Edit TEST-1 — change summary and status.
	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-1", Summary: "Updated Epic", Type: "Epic",
			Status: "Done",
		},
		issueKey: "TEST-1",
		mode:     modeEdit,
	})
	m = result.(AppModel)

	content := m.View().Content
	if !strings.Contains(content, "Updated Epic") {
		t.Error("View should contain updated summary \"Updated Epic\"")
	}
	if !strings.Contains(content, "TEST-1") {
		t.Error("TEST-1 should remain visible after field update")
	}
}

func TestMerge_CreateWithParent_AppearsAsChild(t *testing.T) {
	m := newTestModel()

	// Create a new item with TEST-1 as parent.
	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-3", Summary: "New Sub-task", Type: "Sub-task",
			Status: "To Do",
		},
		issueKey:  "TEST-3",
		mode:      modeCreate,
		parentKey: "TEST-1",
	})
	m = result.(AppModel)

	if !viewContainsID(m, "TEST-3") {
		t.Error("newly created TEST-3 should be visible in the list")
	}
	if !viewContainsID(m, "TEST-1") {
		t.Error("parent TEST-1 should remain visible")
	}
}

func TestMerge_CreateWithoutParent_AppearsAsRoot(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-3", Summary: "New Root Task", Type: "Task",
			Status: "To Do",
		},
		issueKey: "TEST-3",
		mode:     modeCreate,
	})
	m = result.(AppModel)

	if !viewContainsID(m, "TEST-3") {
		t.Error("newly created TEST-3 should be visible as a root item")
	}
}

func TestMerge_CreateWithMissingParent_AppearsAsRoot(t *testing.T) {
	m := newTestModel()

	// Create with a parent that isn't in the registry.
	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-3", Summary: "Orphan Task", Type: "Task",
			Status: "To Do",
		},
		issueKey:  "TEST-3",
		mode:      modeCreate,
		parentKey: "MISSING-99",
	})
	m = result.(AppModel)

	if !viewContainsID(m, "TEST-3") {
		t.Error("TEST-3 with missing parent should appear as a root item, not vanish")
	}
}

func TestMerge_EditClearParent_BecomesRoot(t *testing.T) {
	// Regression: removing a parent server-side returned ParentID=""",
	// but the merge preserved the stale old ParentID.
	m := newTestModel()

	// First, set TEST-1 as a child of TEST-2.
	result, _ := m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-1", Summary: "Epic One", Type: "Epic",
			Status: "In Progress", ParentID: "TEST-2",
		},
		issueKey: "TEST-1",
		mode:     modeEdit,
	})
	m = result.(AppModel)

	// Verify TEST-1 is now a child (the tree prefix indicates nesting).
	if !viewContainsID(m, "TEST-1") {
		t.Fatal("setup: TEST-1 should be visible as child of TEST-2")
	}

	// Now simulate removing the parent — API returns ParentID="".
	result, _ = m.Update(postUpsertCompleteMsg{
		item: &core.WorkItem{
			ID: "TEST-1", Summary: "Epic One", Type: "Epic",
			Status: "In Progress", ParentID: "",
		},
		issueKey: "TEST-1",
		mode:     modeEdit,
	})
	m = result.(AppModel)

	if !viewContainsID(m, "TEST-1") {
		t.Error("TEST-1 should be visible as a root after parent removal")
	}

	// Verify it's actually unparented in the registry.
	if item, ok := m.registry["TEST-1"]; ok {
		if item.ParentID != "" {
			t.Errorf("TEST-1.ParentID = %q; want empty after parent removal", item.ParentID)
		}
	} else {
		t.Error("TEST-1 should exist in registry")
	}
}

func TestMerge_FetchError_NoMerge(t *testing.T) {
	m := newTestModel()

	// Simulate a fetch error — item should not be merged.
	result, _ := m.Update(postUpsertCompleteMsg{
		issueKey: "TEST-1",
		mode:     modeEdit,
		fetchErr: fmt.Errorf("network timeout"),
	})
	m = result.(AppModel)

	// Original item should still be visible with unchanged data.
	if !viewContainsID(m, "TEST-1") {
		t.Error("TEST-1 should remain visible despite fetch error")
	}
	content := m.View().Content
	if !strings.Contains(content, "Sync warning") {
		t.Error("View should contain sync warning notification")
	}
}

func TestTUI_QuitViaCtrlC(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	if fm == nil {
		t.Fatal("FinalModel should not be nil")
	}
}
