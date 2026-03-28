package commands

import (
	"testing"
)

func TestCalculateCursor_EmptySummary(t *testing.T) {
	line, pat := calculateCursor("---\ntype: Story\nsummary: \"\"\n---\n", "")
	if pat != "^summary:" {
		t.Errorf("pattern = %q, want '^summary:'", pat)
	}
	_ = line
}

func TestCalculateCursor_WithSummary(t *testing.T) {
	doc := "---\ntype: Story\nsummary: \"test\"\n---\n\nBody here"
	line, pat := calculateCursor(doc, "test")
	if pat != "" {
		t.Errorf("pattern = %q, want empty (cursor by line)", pat)
	}
	if line != 5 {
		t.Errorf("line = %d, want 5 (after closing ---)", line)
	}
}

func TestFirst(t *testing.T) {
	if got := first("", "", "c"); got != "c" {
		t.Errorf("first(\"\", \"\", \"c\") = %q; want \"c\"", got)
	}
	if got := first("a", "b"); got != "a" {
		t.Errorf("first(\"a\", \"b\") = %q; want \"a\"", got)
	}
	if got := first(""); got != "" {
		t.Errorf("first(\"\") = %q; want \"\"", got)
	}
}
