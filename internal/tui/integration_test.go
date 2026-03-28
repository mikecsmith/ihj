package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/mikecsmith/ihj/internal/testutil"
)

// newTestModel creates a fully initialised AppModel for white-box tests.
// Only used by the integration tests below, which need to send internal
// message types into a running Bubble Tea program.
func newTestModel() AppModel {
	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	s := testutil.NewTestSession(&testutil.MockUI{})

	m := NewAppModel(s, ws, "default", items, time.Time{})
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
