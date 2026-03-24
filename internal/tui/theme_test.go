package tui

import (
	"image/color"
	"testing"
)

func TestTypeColor(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme, nil) // Passing nil for BoardConfig is safe for fallbacks

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
	theme := DefaultTheme()
	styles := NewStyles(theme, nil) // Passing nil for BoardConfig

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

func TestContainsAny(t *testing.T) {
	if !containsAny("in progress", "progress", "active") {
		t.Error("containsAny(\"in progress\", \"progress\", \"active\") = false; want true")
	}
	if containsAny("to do", "progress", "active") {
		t.Error("containsAny(\"to do\", \"progress\", \"active\") = true; want false")
	}
}
