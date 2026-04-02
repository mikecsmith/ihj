package terminal_test

import (
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
	if km.Refresh.Help().Key != "ctrl+r" {
		t.Errorf("Refresh help key = %q, want %q", km.Refresh.Help().Key, "ctrl+r")
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
		"preview_up", "preview_down", "enter_child",
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
		"refresh": "alt+h", // Reserved by help.
	})
	if err == nil {
		t.Fatal("expected error when binding to help key")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error should mention 'reserved', got: %v", err)
	}
}
