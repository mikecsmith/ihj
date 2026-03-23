package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// ─────────────────────────────────────────────────────────────
// Integration tests using teatest v2
//
// These tests run a full Bubble Tea program in test mode and
// verify rendered output via WaitFor.
// ─────────────────────────────────────────────────────────────

func newTestModelForTeatest() AppModel {
	m := newTestModel()
	// teatest handles window sizing via WithInitialTermSize, so reset
	// the ready flag so Init + WindowSizeMsg flows naturally.
	m.ready = false
	return m
}

func TestTUI_InitialRender(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	// Wait for the board name and an issue key to appear in rendered output.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "Test Board") && strings.Contains(s, "TEST-1")
	}, teatest.WithDuration(3*time.Second))
}

func TestTUI_NotificationAppearsInOutput(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	// Wait for initial render first.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	// Inject a transition done message to trigger a notification.
	tm.Send(transitionDoneMsg{issueKey: "TEST-1", newStatus: "Done"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Done")
	}, teatest.WithDuration(3*time.Second))
}

func TestTUI_TransitionPopup(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	defer func() { _ = tm.Quit() }()

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	// Press alt+t to trigger transition fetch.
	tm.Send(tea.KeyPressMsg{Code: 't', Mod: tea.ModAlt})

	// Feed transitions loaded message directly (since mock client's async
	// cmd would need to be executed by the tea runtime, which teatest handles).
	// Wait briefly for the cmd to execute.
	time.Sleep(200 * time.Millisecond)

	// Inject the transitions loaded message to ensure popup appears.
	tm.Send(transitionsLoadedMsg{
		issueKey: "TEST-1",
		transitions: []popupTransition{
			{ID: "10", Name: "To Do"},
			{ID: "20", Name: "In Progress"},
			{ID: "30", Name: "Done"},
		},
	})

	// The popup should display transition options.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		return strings.Contains(s, "To Do") || strings.Contains(s, "In Progress") || strings.Contains(s, "Done")
	}, teatest.WithDuration(3*time.Second))
}

func TestTUI_QuitViaCtrlC(t *testing.T) {
	m := newTestModelForTeatest()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "TEST-1")
	}, teatest.WithDuration(3*time.Second))

	// Send ctrl+c to quit.
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	// FinalModel should return without hanging.
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	if fm == nil {
		t.Fatal("FinalModel should not be nil")
	}
}
