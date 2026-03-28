package tui

import (
	"testing"
)

func TestReviewDiff_EmptyOptions(t *testing.T) {
	b := NewBubbleTeaUI()

	// Pass an empty options slice to trigger the early return
	got, err := b.ReviewDiff("Review", nil, []string{})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if want := -1; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
}

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
