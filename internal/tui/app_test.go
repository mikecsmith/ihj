package tui_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

// Key helpers

func altKey(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Mod: tea.ModAlt}
}

// Model construction

// newTestModel builds an AppModel ready for View/Update testing.
// It sends a WindowSizeMsg to initialize the layout (sets ready=true,
// computes dimensions) and then runs Init() cmds to populate the
// cached user name (needed for assign flow).
func newTestModel(t *testing.T) tui.AppModel {
	t.Helper()

	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	rt := testutil.NewTestRuntime(ui)
	provider := testutil.NewMockProvider()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := testutil.NewTestFactory(provider)

	m := tui.NewAppModel(context.Background(), rt, wsSess, factory, ws, "default", items, time.Time{}, ui)

	// Initialize: run Init() and drain all batched cmds so the model
	// has its cached user name and other setup state.
	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	// Send a WindowSizeMsg to trigger layout calculation and mark ready.
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(tui.AppModel)

	return m
}

// drainCmds executes a cmd (which may be a batch) and feeds each
// resulting message back through Update. It recurses once to handle
// any secondary cmds produced by those messages.
func drainCmds(t *testing.T, m *tui.AppModel, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}

	// tea.Batch returns a BatchMsg; handle it by draining each sub-cmd.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			drainCmds(t, m, sub)
		}
		return
	}

	result, nextCmd := m.Update(msg)
	*m = result.(tui.AppModel)
	// Don't recurse into tick cmds (they'd loop forever).
	_ = nextCmd
}

// viewContent extracts the rendered string from the model's View().
func viewContent(m tui.AppModel) string {
	v := m.View()
	return v.Content
}

func TestInitialViewContainsIssueData(t *testing.T) {
	m := newTestModel(t)
	content := viewContent(m)
	if content == "" {
		t.Fatal("View() should produce non-empty content after WindowSizeMsg")
	}

	// The view should contain the workspace name.
	if !strings.Contains(content, "Engineering") {
		t.Error("View() should contain workspace name \"Engineering\"")
	}

	// The view should contain the test issue IDs.
	for _, id := range []string{"TEST-1", "TEST-2"} {
		if !strings.Contains(content, id) {
			t.Errorf("View() should contain issue ID %q", id)
		}
	}
}

// Filter: single filter

func TestFilterSingleFilter(t *testing.T) {
	m := newTestModel(t)
	// Workspace has only one filter ("default").

	result, _ := m.Update(altKey('f'))
	m = result.(tui.AppModel)

	content := viewContent(m)
	// With only one filter, should show "Only one filter" in view (as notification).
	if !strings.Contains(content, "Only one filter") {
		t.Error("View() should contain \"Only one filter\" when only default filter exists")
	}
}

// Filter: multiple filters

func TestFilterSwitch_MultipleFilters(t *testing.T) {
	// Build a workspace with multiple filters.
	ws := testutil.TestWorkspace()
	ws.Filters["backlog"] = "status = Backlog"

	items := testutil.TestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	rt := testutil.NewTestRuntime(ui)
	provider := testutil.NewMockProvider()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := testutil.NewTestFactory(provider)

	m := tui.NewAppModel(context.Background(), rt, wsSess, factory, ws, "default", items, time.Time{}, ui)

	// Initialize and set layout.
	initCmd := m.Init()
	drainCmds(t, &m, initCmd)
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(tui.AppModel)

	// Press alt+f → should open filter popup with options.
	result, _ = m.Update(altKey('f'))
	m = result.(tui.AppModel)

	content := viewContent(m)
	// The filter popup should show available filter names.
	if !strings.Contains(content, "backlog") && !strings.Contains(content, "default") {
		t.Error("View() after alt+f should contain filter names when multiple filters exist")
	}
}

// Notification rendering

func TestNotifyRenderedInView(t *testing.T) {
	m := newTestModel(t)

	// Press alt+f with single filter → notification appears.
	result, _ := m.Update(altKey('f'))
	m = result.(tui.AppModel)

	content := viewContent(m)
	if !strings.Contains(content, "Only one filter") {
		t.Error("View() should contain notification after action")
	}
}
