package terminal_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/terminal"
)

func TestApplyShortcuts_ReplacesBinding(t *testing.T) {
	km := terminal.DefaultKeyMap()
	if err := km.ApplyShortcuts(map[string]string{
		"refresh": "ctrl+r",
	}); err != nil {
		t.Fatal(err)
	}

	keys := km.Refresh.Keys()
	if len(keys) != 1 || keys[0] != "ctrl+r" {
		t.Errorf("Refresh keys = %v, want [ctrl+r]", keys)
	}
	if km.Refresh.Help().Key != "Ctrl-R" {
		t.Errorf("Refresh help key = %q, want %q", km.Refresh.Help().Key, "Ctrl-R")
	}
	if km.Refresh.Help().Desc != "Refresh" {
		t.Errorf("Refresh help desc = %q, want %q", km.Refresh.Help().Desc, "Refresh")
	}
}

func TestApplyShortcuts_PreservesUnspecified(t *testing.T) {
	km := terminal.DefaultKeyMap()
	origFilterKeys := km.Filter.Keys()

	if err := km.ApplyShortcuts(map[string]string{
		"refresh": "ctrl+r",
	}); err != nil {
		t.Fatal(err)
	}

	// Filter should be unchanged.
	gotKeys := km.Filter.Keys()
	if len(gotKeys) != len(origFilterKeys) {
		t.Fatalf("Filter keys length changed: got %d, want %d", len(gotKeys), len(origFilterKeys))
	}
	for i, k := range gotKeys {
		if k != origFilterKeys[i] {
			t.Errorf("Filter key %d = %q, want %q", i, k, origFilterKeys[i])
		}
	}
}

func TestApplyShortcuts_IgnoresUnknown(t *testing.T) {
	km := terminal.DefaultKeyMap()
	if err := km.ApplyShortcuts(map[string]string{
		"nonexistent": "ctrl+z",
	}); err != nil {
		t.Errorf("unknown action should not error, got: %v", err)
	}
}

func TestApplyShortcuts_NilMap(t *testing.T) {
	km := terminal.DefaultKeyMap()
	if err := km.ApplyShortcuts(nil); err != nil {
		t.Errorf("nil map should not error, got: %v", err)
	}
}

func TestApplyShortcuts_NavigationAndModalKeysNotConfigurable(t *testing.T) {
	protected := []string{
		"up", "down", "home", "end", "pageup", "pagedown",
		"detail_up", "detail_down", "focus", "tab",
		"search", "command", "submit", "cancel", "quit",
	}

	for _, name := range protected {
		km := terminal.DefaultKeyMap()
		origUp := km.Up.Keys()
		origCancel := km.Cancel.Keys()

		_ = km.ApplyShortcuts(map[string]string{
			name: "alt+f12",
		})

		// Verify navigation and modal keys are unchanged.
		if km.Up.Keys()[0] != origUp[0] {
			t.Errorf("Up was modified by shortcut %q", name)
		}
		if km.Cancel.Keys()[0] != origCancel[0] {
			t.Errorf("Cancel was modified by shortcut %q", name)
		}
	}
}

func TestApplyShortcuts_RejectsReservedKey(t *testing.T) {
	km := terminal.DefaultKeyMap()
	err := km.ApplyShortcuts(map[string]string{
		"refresh": "ctrl+c", // Reserved by quit.
	})
	if err == nil {
		t.Fatal("expected error when binding to reserved key")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error should mention 'reserved', got: %v", err)
	}
}

func TestApplyShortcuts_RejectsDuplicateKey(t *testing.T) {
	km := terminal.DefaultKeyMap()
	err := km.ApplyShortcuts(map[string]string{
		"refresh": "ctrl+r",
		"filter":  "ctrl+r", // Same key as refresh.
	})
	if err == nil {
		t.Fatal("expected error when two actions share the same key")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error should mention 'already used', got: %v", err)
	}
}

func TestApplyShortcuts_RejectsBareCharacter(t *testing.T) {
	tests := []string{"e", "?", "1", "/"}
	for _, k := range tests {
		km := terminal.DefaultKeyMap()
		err := km.ApplyShortcuts(map[string]string{
			"refresh": k,
		})
		if err == nil {
			t.Errorf("expected error for bare character %q (would break search)", k)
		}
		if err != nil && !strings.Contains(err.Error(), "modifier") {
			t.Errorf("error for %q should mention 'modifier', got: %v", k, err)
		}
	}
}

func TestApplyShortcuts_RejectsShiftOnly(t *testing.T) {
	km := terminal.DefaultKeyMap()
	err := km.ApplyShortcuts(map[string]string{
		"refresh": "shift+r",
	})
	if err == nil {
		t.Fatal("expected error: shift alone is not a valid modifier for shortcuts")
	}
	if !strings.Contains(err.Error(), "modifier") {
		t.Errorf("error should mention 'modifier', got: %v", err)
	}
}

func TestApplyShortcuts_RejectsCollisionWithExistingDefault(t *testing.T) {
	km := terminal.DefaultKeyMap()
	err := km.ApplyShortcuts(map[string]string{
		"refresh": "alt+e", // Default binding for "edit".
	})
	if err == nil {
		t.Fatal("expected error when shortcut collides with existing default binding")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error should mention 'already used', got: %v", err)
	}
}

func TestApplyShortcuts_RejectsHelpKey(t *testing.T) {
	km := terminal.DefaultKeyMap()
	err := km.ApplyShortcuts(map[string]string{
		"refresh": "alt+/", // Reserved by help.
	})
	if err == nil {
		t.Fatal("expected error when binding to help key")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error should mention 'reserved', got: %v", err)
	}
}

// ── HintKeys Tests ──────────────────────────────────────────────

func TestHintKeys_DefaultMode_AllDigitsAndLettersAvailable(t *testing.T) {
	km := terminal.DefaultKeyMap()
	hints := km.HintKeys()

	// Default mode uses modifier keys for actions (alt+r, ctrl+f, etc.)
	// so all bare 0-9 and a-z should be available.
	want := []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	if len(hints) != len(want) {
		t.Errorf("HintKeys() returned %d hints, want %d", len(hints), len(want))
	}
	for _, r := range want {
		if !slices.Contains(hints, r) {
			t.Errorf("HintKeys() missing %q", r)
		}
	}
}

func TestHintKeys_DefaultMode_StartsWithZero(t *testing.T) {
	km := terminal.DefaultKeyMap()
	hints := km.HintKeys()
	if len(hints) == 0 {
		t.Fatal("HintKeys() returned empty")
	}
	if hints[0] != '0' {
		t.Errorf("first hint = %q, want '0'", hints[0])
	}
}

func TestHintKeys_DefaultMode_DigitsBeforeLetters(t *testing.T) {
	km := terminal.DefaultKeyMap()
	hints := km.HintKeys()

	lastDigitIdx := -1
	firstLetterIdx := -1
	for i, r := range hints {
		if r >= '0' && r <= '9' {
			lastDigitIdx = i
		}
		if r >= 'a' && r <= 'z' && firstLetterIdx == -1 {
			firstLetterIdx = i
		}
	}
	if firstLetterIdx <= lastDigitIdx {
		t.Errorf("letters should come after digits: last digit at %d, first letter at %d",
			lastDigitIdx, firstLetterIdx)
	}
}

func TestHintKeys_VimMode_ExcludesBoundKeys(t *testing.T) {
	km := terminal.VimKeyMap()
	hints := km.HintKeys()

	// Vim mode binds these single-char keys to actions/navigation/modes.
	excludedChars := []rune{'j', 'k', 'g', 'r', 'f', 'a', 't', 'o', 'e', 'c', 'b', 'x', 'n', 'w', '?', '/', ':'}
	for _, r := range excludedChars {
		if slices.Contains(hints, r) {
			t.Errorf("HintKeys() should exclude %q (bound in vim mode)", r)
		}
	}

	// Digits should still be available (not bound to actions).
	for c := '0'; c <= '9'; c++ {
		if !slices.Contains(hints, c) {
			t.Errorf("HintKeys() should include digit %q in vim mode", c)
		}
	}
}

func TestHintKeys_VimMode_HasFewerThanDefault(t *testing.T) {
	defaultHints := terminal.DefaultKeyMap().HintKeys()
	vimHints := terminal.VimKeyMap().HintKeys()

	if len(vimHints) >= len(defaultHints) {
		t.Errorf("vim hints (%d) should be fewer than default hints (%d)",
			len(vimHints), len(defaultHints))
	}
}

func TestHintKeys_NoDuplicates(t *testing.T) {
	for _, tt := range []struct {
		name string
		km   terminal.KeyMap
	}{
		{"default", terminal.DefaultKeyMap()},
		{"vim", terminal.VimKeyMap()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hints := tt.km.HintKeys()
			seen := map[rune]bool{}
			for _, r := range hints {
				if seen[r] {
					t.Errorf("duplicate hint key %q", r)
				}
				seen[r] = true
			}
		})
	}
}
