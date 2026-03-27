package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikecsmith/ihj/internal/commands"
)

// mockRuneKey generates a simple v2 tea.KeyPressMsg for standard characters
func mockRuneKey(code rune) tea.Msg {
	return tea.KeyPressMsg{Code: code}
}

func TestSelectModel_Update(t *testing.T) {
	keys := DefaultKeyMap()
	m := selectModel{
		title:   "Test",
		options: []string{"A", "B", "C"},
		cursor:  0,
		chosen:  -1,
		keys:    keys,
	}

	m1, cmd := m.Update(mockRuneKey('3'))
	sm1 := m1.(selectModel)

	if got, want := sm1.chosen, 2; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
	if got, want := cmd != nil, true; got != want {
		t.Errorf("cmd returned = %v, want %v (tea.Quit)", got, want)
	}

	m2, cmd2 := sm1.Update(mockRuneKey(27))
	sm2 := m2.(selectModel)

	if got, want := sm2.chosen, -1; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
	if got, want := cmd2 != nil, true; got != want {
		t.Errorf("cmd returned = %v, want %v (tea.Quit)", got, want)
	}
}

func TestConfirmModel_Update(t *testing.T) {
	keys := DefaultKeyMap()
	m := confirmModel{prompt: "Sure?", keys: keys}

	m1, cmd := m.Update(mockRuneKey('y'))
	cm1 := m1.(confirmModel)

	if got, want := cm1.yes, true; got != want {
		t.Errorf("yes = %v, want %v", got, want)
	}
	if got, want := cmd != nil, true; got != want {
		t.Errorf("cmd returned = %v, want %v (tea.Quit)", got, want)
	}

	m2, cmd2 := m.Update(mockRuneKey(tea.KeyEnter))
	cm2 := m2.(confirmModel)

	if got, want := cm2.yes, false; got != want {
		t.Errorf("yes = %v, want %v", got, want)
	}
	if got, want := cmd2 != nil, true; got != want {
		t.Errorf("cmd returned = %v, want %v (tea.Quit)", got, want)
	}
}

func TestDiffModel_Update(t *testing.T) {
	keys := DefaultKeyMap()
	m := diffModel{
		title:   "Review",
		changes: []commands.FieldDiff{{Field: "Summary", Old: "foo", New: "bar"}},
		options: []string{"Apply", "Skip"},
		cursor:  0,
		chosen:  -1,
		keys:    keys,
	}

	m1, cmd := m.Update(mockRuneKey('2'))
	dm1 := m1.(diffModel)

	if got, want := dm1.chosen, 1; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
	if got, want := cmd != nil, true; got != want {
		t.Errorf("cmd returned = %v, want %v (tea.Quit)", got, want)
	}
}

func TestRenderRichDiff(t *testing.T) {
	theme := DefaultTheme()

	// Using a shared context ensures DiffCleanupSemantic doesn't collapse the diff
	oldText := "The quick brown fox"
	newText := "The fast brown fox"

	result := renderRichDiff(oldText, newText, theme)

	// Strip ANSI escape codes to verify the text content safely
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	cleanResult := ansiRegex.ReplaceAllString(result, "")

	if got, want := strings.Contains(cleanResult, "quick"), true; got != want {
		t.Errorf("strings.Contains(cleanResult, %q) = %v, want %v\nCleaned Output: %q\nRaw Output: %q", "quick", got, want, cleanResult, result)
	}
	if got, want := strings.Contains(cleanResult, "fast"), true; got != want {
		t.Errorf("strings.Contains(cleanResult, %q) = %v, want %v\nCleaned Output: %q", "fast", got, want, cleanResult)
	}
	if got, want := strings.Contains(cleanResult, "brown fox"), true; got != want {
		t.Errorf("strings.Contains(cleanResult, %q) = %v, want %v\nCleaned Output: %q", "brown fox", got, want, cleanResult)
	}
}
