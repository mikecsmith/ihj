package terminal_test

import (
	"image/color"
	"testing"

	"github.com/mikecsmith/ihj/internal/terminal"
)

func TestTypeColor(t *testing.T) {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")

	tests := []struct {
		input string
		want  color.Color
	}{
		{input: "Initiative", want: theme.TypeInitiative},
		{"Epic", theme.TypeEpic},
		{"epic", theme.TypeEpic},
		{"Story", theme.TypeStory},
		{"Bug", theme.TypeBug},
		{"Sub-task", theme.TypeSubtask},
		{"subtask", theme.TypeSubtask},
		{"Task", theme.TypeTask},
		{"Unknown", theme.TypeTask},
	}

	for _, tt := range tests {
		got := styles.TypeColor(tt.input)
		if got != tt.want {
			t.Errorf("TypeColor(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestStatusStyle(t *testing.T) {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")

	tests := []struct {
		input    string
		wantIcon string
		wantClr  color.Color
	}{
		{"Done", "✔", theme.StatusDone},
		{"Blocked", "✘", theme.StatusBlocked},
		{"In Review", "◉", theme.StatusReview},
		{"In Progress", "▶", theme.StatusActive},
		{"Refined", "★", theme.StatusReady},
		{"To Do", "○", theme.StatusDefault},
	}

	for _, tt := range tests {
		icon, clr := styles.StatusStyle(tt.input)
		if icon != tt.wantIcon {
			t.Errorf("StatusStyle(%q) icon = %q, want %q", tt.input, icon, tt.wantIcon)
		}
		if clr != tt.wantClr {
			t.Errorf("StatusStyle(%q) color = %v, want %v", tt.input, clr, tt.wantClr)
		}
	}
}
