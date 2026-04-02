package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/testutil"
)

// newTestModel builds a synchronous AppModel for white-box unit tests.
func newTestModel(t *testing.T) AppModel {
	t.Helper()
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()

	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, true)
	m.width = 120
	m.height = 40
	m.ready = true
	m.cachedUserName = "Demo User"
	m.recalcLayout()
	m.syncDetail()
	return m
}

// newVimTestModel creates a fully initialised AppModel with vim mode enabled.
func newVimTestModel(t *testing.T) AppModel {
	t.Helper()
	m := newTestModel(t)
	m.vimMode = true
	m.capture = CaptureNone
	m.keys = terminal.VimKeyMap()
	m.list.search.Blur()
	return m
}

// sendKey sends a key press to the model and returns the updated model.
func sendKey(t *testing.T, m AppModel, keyStr string) AppModel {
	t.Helper()
	result, _ := m.Update(tea.KeyPressMsg{Code: rune(keyStr[0]), Text: keyStr})
	return result.(AppModel)
}

func TestVim_StartsInNormalMode(t *testing.T) {
	m := newVimTestModel(t)
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone", m.capture)
	}
}

func TestVim_SlashEntersSearchMode(t *testing.T) {
	m := newVimTestModel(t)
	m = sendKey(t, m, "/")
	if m.capture != CaptureSearch {
		t.Errorf("capture = %d, want CaptureSearch", m.capture)
	}
}

func TestVim_ColonEntersCommandMode(t *testing.T) {
	m := newVimTestModel(t)
	m = sendKey(t, m, ":")
	if m.capture != CaptureCommand {
		t.Errorf("capture = %d, want CaptureCommand", m.capture)
	}
}

func TestVim_EscFromSearchReturnsToNormal(t *testing.T) {
	m := newVimTestModel(t)
	m = sendKey(t, m, "/")
	if m.capture != CaptureSearch {
		t.Fatal("expected CaptureSearch after /")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(AppModel)
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone after Esc", m.capture)
	}
}

func TestVim_EscFromCommandReturnsToNormal(t *testing.T) {
	m := newVimTestModel(t)
	m = sendKey(t, m, ":")
	if m.capture != CaptureCommand {
		t.Fatal("expected CaptureCommand after :")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(AppModel)
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone after Esc", m.capture)
	}
	if m.cmdBuf != "" {
		t.Errorf("cmdBuf = %q, want empty", m.cmdBuf)
	}
}

func TestVim_CommandQQuits(t *testing.T) {
	m := newVimTestModel(t)

	// Enter command mode and type "q".
	m = sendKey(t, m, ":")
	m = sendKey(t, m, "q")

	// Press enter to execute.
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(AppModel)

	if cmd == nil {
		t.Fatal("expected quit command from :q")
	}
	// Verify it returned to normal mode.
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone after :q", m.capture)
	}
}

func TestVim_UnknownCommandShowsNotification(t *testing.T) {
	m := newVimTestModel(t)

	m = sendKey(t, m, ":")
	m = sendKey(t, m, "z")

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(AppModel)

	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone", m.capture)
	}
	if !strings.Contains(m.View().Content, "Unknown command") {
		t.Error("expected unknown command notification")
	}
}

func TestVim_JKNavigation(t *testing.T) {
	m := newVimTestModel(t)
	if m.list.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.list.cursor)
	}

	// j moves down.
	m = sendKey(t, m, "j")
	if m.list.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m.list.cursor)
	}

	// k moves back up.
	m = sendKey(t, m, "k")
	if m.list.cursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", m.list.cursor)
	}
}

func TestVim_GAndShiftGNavigation(t *testing.T) {
	m := newVimTestModel(t)

	// G goes to end (uppercase letter, no modifier — that's how terminals send it).
	result, _ := m.Update(tea.KeyPressMsg{Code: 'G'})
	m = result.(AppModel)
	lastIdx := len(m.list.filtered) - 1
	if m.list.cursor != lastIdx {
		t.Errorf("after G: cursor = %d, want %d", m.list.cursor, lastIdx)
	}

	// g goes to start.
	m = sendKey(t, m, "g")
	if m.list.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", m.list.cursor)
	}
}

func TestVim_SearchFilters(t *testing.T) {
	m := newVimTestModel(t)

	// Enter search mode.
	m = sendKey(t, m, "/")
	if m.capture != CaptureSearch {
		t.Fatal("expected CaptureSearch")
	}

	// Type a search query — the search input should receive it.
	for _, c := range "test" {
		r, _ := m.Update(tea.KeyPressMsg{Code: c, Text: string(c)})
		m = r.(AppModel)
	}

	if m.list.search.Value() == "" {
		t.Error("search input should have content after typing in search mode")
	}

	// Enter exits search mode but keeps filter.
	r, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = r.(AppModel)
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone after Enter", m.capture)
	}
	if m.list.search.Value() == "" {
		t.Error("search value should be preserved after exiting search mode")
	}
}

func TestVim_NormalModeDoesNotTypeIntoSearch(t *testing.T) {
	m := newVimTestModel(t)

	// Type 'j' in normal mode — should navigate, not search.
	m = sendKey(t, m, "j")
	if m.list.search.Value() != "" {
		t.Errorf("search = %q, want empty — normal mode chars should not go to search", m.list.search.Value())
	}
}

func TestVim_CtrlCQuitsFromAnyMode(t *testing.T) {
	modes := []struct {
		name string
		mode InputCapture
	}{
		{"normal", CaptureNone},
		{"search", CaptureSearch},
		{"command", CaptureCommand},
	}

	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			m := newVimTestModel(t)
			m.capture = tt.mode

			_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
			if cmd == nil {
				t.Errorf("expected quit command from Ctrl+C in %s mode", tt.name)
			}
		})
	}
}

func TestVim_BackspaceExitsEmptyCommandMode(t *testing.T) {
	m := newVimTestModel(t)
	m = sendKey(t, m, ":")
	if m.capture != CaptureCommand {
		t.Fatal("expected CaptureCommand")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = result.(AppModel)
	if m.capture != CaptureNone {
		t.Errorf("capture = %d, want CaptureNone after backspace on empty cmd", m.capture)
	}
}

func TestVim_EscDoesNotQuitFromNormalMode(t *testing.T) {
	m := newVimTestModel(t)

	// Esc in normal mode with no search/child to clear should be a no-op,
	// not a quit. Use :q to quit in vim mode.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Error("Esc in normal mode should not produce a quit command")
	}
}

func TestVim_HelpBarShowsMode(t *testing.T) {
	m := newVimTestModel(t)
	bar := m.renderVimHelpBar(120)
	if !strings.Contains(bar, "NORMAL") {
		t.Error("expected NORMAL in help bar")
	}

	m = sendKey(t, m, ":")
	bar = m.renderVimHelpBar(120)
	if !strings.Contains(bar, ":") {
		t.Error("expected : prompt in command mode help bar")
	}
}

func TestVim_ResolveActionViaKeyMap(t *testing.T) {
	m := newVimTestModel(t)

	tests := []struct {
		key  rune
		want Action
	}{
		{'r', ActionRefresh},
		{'f', ActionFilter},
		{'a', ActionAssign},
		{'t', ActionTransition},
		{'o', ActionOpen},
		{'e', ActionEdit},
		{'c', ActionComment},
		{'b', ActionBranch},
		{'x', ActionExtract},
		{'n', ActionNew},
		{'z', ActionNone},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			got := m.resolveAction(tea.KeyPressMsg{Code: tt.key})
			if got != tt.want {
				t.Errorf("resolveAction(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestVim_AltKeysDoNotResolve(t *testing.T) {
	m := newVimTestModel(t)

	// In vim mode, Alt-R should NOT resolve to an action — vim is opt-in,
	// no backwards compatibility with alt-key bindings.
	action := m.resolveAction(tea.KeyPressMsg{Code: 'r', Mod: tea.ModAlt})
	if action != ActionNone {
		t.Errorf("Alt-R action = %d, want ActionNone in vim mode", action)
	}
}
