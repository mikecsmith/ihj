package terminal_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/terminal"
)

func TestCalculateCursor_EmptySummary(t *testing.T) {
	line, pat := terminal.CalculateCursor("---\ntype: Story\nsummary: \"\"\n---\n", "")
	if pat != "^summary:" {
		t.Errorf("pattern = %q, want '^summary:'", pat)
	}
	_ = line
}

func TestCalculateCursor_WithSummary(t *testing.T) {
	doc := "---\ntype: Story\nsummary: \"test\"\n---\n\nBody here"
	line, pat := terminal.CalculateCursor(doc, "test")
	if pat != "" {
		t.Errorf("pattern = %q, want empty (cursor by line)", pat)
	}
	if line != 5 {
		t.Errorf("line = %d, want 5 (after closing ---)", line)
	}
}
