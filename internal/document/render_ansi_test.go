package document

import (
	"strings"
	"testing"
)

const (
	ansiBold  = "\033[1m"
	ansiReset = "\033[0m"
)

func TestRenderANSI_PlainParagraph(t *testing.T) {
	doc := NewDoc(NewParagraph(NewText("Hello")))
	out := RenderANSI(doc, ANSIConfig{})
	if !strings.Contains(out, "Hello") {
		t.Errorf("RenderANSI() = %q; want containing \"Hello\"", out)
	}
}

func TestRenderANSI_BoldText(t *testing.T) {
	doc := NewDoc(NewParagraph(NewStyledText("bold", Bold())))
	out := RenderANSI(doc, ANSIConfig{Styles: ANSIStyles{}})
	if !strings.Contains(out, ansiBold) {
		t.Errorf("RenderANSI() = %q; want containing bold escape sequence", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("RenderANSI() = %q; want containing \"bold\"", out)
	}
}

func TestRenderANSI_Link(t *testing.T) {
	doc := NewDoc(NewParagraph(NewStyledText("click", Link("https://x.com"))))
	out := RenderANSI(doc, ANSIConfig{Styles: ANSIStyles{}})
	// Should contain OSC 8 hyperlink escape.
	if !strings.Contains(out, "\033]8;;https://x.com\a") {
		t.Errorf("RenderANSI() = %q; want containing OSC 8 link start", out)
	}
	if !strings.Contains(out, "\033]8;;\a") {
		t.Errorf("RenderANSI() = %q; want containing OSC 8 link end", out)
	}
}

func TestRenderANSI_CodeBlock(t *testing.T) {
	doc := NewDoc(NewCodeBlock("go", "x := 1"))
	out := RenderANSI(doc, ANSIConfig{})
	if !strings.Contains(out, "GO") {
		t.Errorf("RenderANSI() = %q; want containing \"GO\" language label", out)
	}
	if !strings.Contains(out, "x := 1") {
		t.Errorf("RenderANSI() = %q; want containing \"x := 1\"", out)
	}
}

func TestRenderANSI_WrapWidth(t *testing.T) {
	long := strings.Repeat("word ", 20) // ~100 chars
	doc := NewDoc(NewParagraph(NewText(long)))
	out := RenderANSI(doc, ANSIConfig{WrapWidth: 40})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Errorf("RenderANSI() produced %d lines; want >= 2 with WrapWidth=40", len(lines))
	}
}

func TestVisibleLength(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{ansiBold + "bold" + ansiReset, 4},
		{"\033]8;;https://x.com\ahello\033]8;;\a", 5},
		{"", 0},
	}
	for _, tc := range cases {
		got := visibleLength(tc.input)
		if got != tc.want {
			t.Errorf("visibleLength(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
