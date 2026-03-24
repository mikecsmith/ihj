package tui

import (
	"testing"
)

func TestSplitShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"vim", []string{"vim"}},
		{"code --wait", []string{"code", "--wait"}},
		{`vim -c "set paste"`, []string{"vim", "-c", "set paste"}},
	}

	for _, tt := range tests {
		got := splitShellCommand(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitShellCommand(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitShellCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsVimLike(t *testing.T) {
	for _, name := range []string{"vim", "nvim", "vi", "Vim"} {
		if !isVimLike(name) {
			t.Errorf("isVimLike(%q) should be true", name)
		}
	}
	for _, name := range []string{"code", "nano"} {
		if isVimLike(name) {
			t.Errorf("isVimLike(%q) should be false", name)
		}
	}
}
