package tui

import (
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
func newTestModel() AppModel {
	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	rt := testutil.NewTestRuntime(ui)
	provider := testutil.NewMockProvider()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := testutil.NewTestFactory(provider)

	m := NewAppModel(rt, wsSess, factory, ws, "default", items, time.Time{}, ui)
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

// viewContainsID checks whether an issue ID appears in the rendered View.
func viewContainsID(m AppModel, id string) bool {
	return strings.Contains(m.View().Content, id)
}

func TestDataReloadUpdatesRegistry(t *testing.T) {
	m := newTestModel()

	if !viewContainsID(m, "TEST-1") || !viewContainsID(m, "TEST-2") {
		t.Fatal("setup: both items should be visible initially")
	}

	// Simulate a data reload that adds a new item.
	items := testutil.TestItems()
	items = append(items, &core.WorkItem{ID: "TEST-3", Summary: "New Task", Type: "Task", Status: "To Do"})
	result, _ := m.Update(dataReloadedMsg{
		filter:    "default",
		items:     items,
		fetchedAt: time.Now(),
	})
	m = result.(AppModel)

	if !viewContainsID(m, "TEST-3") {
		t.Error("TEST-3 should be visible after data reload")
	}
}
