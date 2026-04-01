package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/terminal"
)

// newVimTestModel creates a fully initialised AppModel with vim mode enabled.
func newVimTestModel() AppModel {
	m := newTestModel()
	m.vimMode = true
	m.inputMode = ModeNormal
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
	m := newVimTestModel()
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal", m.inputMode)
	}
}

func TestVim_SlashEntersSearchMode(t *testing.T) {
	m := newVimTestModel()
	m = sendKey(t, m, "/")
	if m.inputMode != ModeSearch {
		t.Errorf("inputMode = %d, want ModeSearch", m.inputMode)
	}
}

func TestVim_ColonEntersCommandMode(t *testing.T) {
	m := newVimTestModel()
	m = sendKey(t, m, ":")
	if m.inputMode != ModeCommand {
		t.Errorf("inputMode = %d, want ModeCommand", m.inputMode)
	}
}

func TestVim_EscFromSearchReturnsToNormal(t *testing.T) {
	m := newVimTestModel()
	m = sendKey(t, m, "/")
	if m.inputMode != ModeSearch {
		t.Fatal("expected ModeSearch after /")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(AppModel)
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal after Esc", m.inputMode)
	}
}

func TestVim_EscFromCommandReturnsToNormal(t *testing.T) {
	m := newVimTestModel()
	m = sendKey(t, m, ":")
	if m.inputMode != ModeCommand {
		t.Fatal("expected ModeCommand after :")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(AppModel)
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal after Esc", m.inputMode)
	}
	if m.cmdBuf != "" {
		t.Errorf("cmdBuf = %q, want empty", m.cmdBuf)
	}
}

func TestVim_CommandQQuits(t *testing.T) {
	m := newVimTestModel()

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
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal after :q", m.inputMode)
	}
}

func TestVim_UnknownCommandShowsNotification(t *testing.T) {
	m := newVimTestModel()

	m = sendKey(t, m, ":")
	m = sendKey(t, m, "z")

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(AppModel)

	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal", m.inputMode)
	}
	if !strings.Contains(m.View().Content, "Unknown command") {
		t.Error("expected unknown command notification")
	}
}

func TestVim_JKNavigation(t *testing.T) {
	m := newVimTestModel()
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
	m := newVimTestModel()

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
	m := newVimTestModel()

	// Enter search mode.
	m = sendKey(t, m, "/")
	if m.inputMode != ModeSearch {
		t.Fatal("expected ModeSearch")
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
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal after Enter", m.inputMode)
	}
	if m.list.search.Value() == "" {
		t.Error("search value should be preserved after exiting search mode")
	}
}

func TestVim_NormalModeDoesNotTypeIntoSearch(t *testing.T) {
	m := newVimTestModel()

	// Type 'j' in normal mode — should navigate, not search.
	m = sendKey(t, m, "j")
	if m.list.search.Value() != "" {
		t.Errorf("search = %q, want empty — normal mode chars should not go to search", m.list.search.Value())
	}
}

func TestVim_CtrlCQuitsFromAnyMode(t *testing.T) {
	modes := []struct {
		name string
		mode InputMode
	}{
		{"normal", ModeNormal},
		{"search", ModeSearch},
		{"command", ModeCommand},
	}

	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			m := newVimTestModel()
			m.inputMode = tt.mode

			_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
			if cmd == nil {
				t.Errorf("expected quit command from Ctrl+C in %s mode", tt.name)
			}
		})
	}
}

func TestVim_BackspaceExitsEmptyCommandMode(t *testing.T) {
	m := newVimTestModel()
	m = sendKey(t, m, ":")
	if m.inputMode != ModeCommand {
		t.Fatal("expected ModeCommand")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = result.(AppModel)
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal after backspace on empty cmd", m.inputMode)
	}
}

func TestVim_HelpBarShowsMode(t *testing.T) {
	m := newVimTestModel()
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
	m := newVimTestModel()

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
	m := newVimTestModel()

	// In vim mode, Alt-R should NOT resolve to an action — vim is opt-in,
	// no backwards compatibility with alt-key bindings.
	action := m.resolveAction(tea.KeyPressMsg{Code: 'r', Mod: tea.ModAlt})
	if action != ActionNone {
		t.Errorf("Alt-R action = %d, want ActionNone in vim mode", action)
	}
}
