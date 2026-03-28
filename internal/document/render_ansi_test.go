package document_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

const (
	ansiBold  = "\033[1m"
	ansiReset = "\033[0m"
)

// ansiStyles is a test-only StyleSet that uses raw ANSI escape codes,
// allowing tests to verify that RenderANSI delegates styling correctly.
type ansiStyles struct{}

func (ansiStyles) Bold(text string) string      { return "\033[1m" + text + "\033[0m" }
func (ansiStyles) Italic(text string) string    { return "\033[3m" + text + "\033[0m" }
func (ansiStyles) Code(text string) string      { return "\033[100m\033[36m" + text + "\033[0m" }
func (ansiStyles) Strike(text string) string    { return "\033[2m~" + text + "~\033[0m" }
func (ansiStyles) Underline(text string) string { return "\033[4m" + text + "\033[0m" }
func (ansiStyles) Dim(text string) string       { return "\033[2m" + text + "\033[0m" }

func (ansiStyles) Link(text, href string) string {
	return fmt.Sprintf("\033]8;;%s\a\033[34m\033[4m%s\033[24m\033[0m\033]8;;\a", href, text)
}

func (ansiStyles) Heading(text string, _ int) string {
	return "\033[36m\033[1m" + strings.ToUpper(text) + "\033[0m"
}

func (ansiStyles) CodeBlockLabel(lang string) string {
	return fmt.Sprintf("\033[100m\033[1m   %s \033[0m", lang)
}

func (ansiStyles) CodeBlockBorder() string         { return "┃" }
func (ansiStyles) BlockquoteBorder() string        { return "│" }
func (ansiStyles) HorizontalRule(width int) string { return strings.Repeat("─", width) }

func (ansiStyles) MediaPlaceholder(alt, url string) string {
	return fmt.Sprintf("[%s: %s]", alt, url)
}

func TestRenderANSI_PlainParagraph(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewText("Hello")))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "Hello") {
		t.Errorf("RenderANSI() = %q; want containing \"Hello\"", out)
	}
}

func TestRenderANSI_BoldText(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewStyledText("bold", document.Bold())))
	out := document.RenderANSI(doc, document.ANSIConfig{Styles: ansiStyles{}})
	if !strings.Contains(out, ansiBold) {
		t.Errorf("RenderANSI() = %q; want containing bold escape sequence", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("RenderANSI() = %q; want containing \"bold\"", out)
	}
}

func TestRenderANSI_Link(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewStyledText("click", document.Link("https://x.com"))))
	out := document.RenderANSI(doc, document.ANSIConfig{Styles: ansiStyles{}})
	// Should contain OSC 8 hyperlink escape.
	if !strings.Contains(out, "\033]8;;https://x.com\a") {
		t.Errorf("RenderANSI() = %q; want containing OSC 8 link start", out)
	}
	if !strings.Contains(out, "\033]8;;\a") {
		t.Errorf("RenderANSI() = %q; want containing OSC 8 link end", out)
	}
}

func TestRenderANSI_CodeBlock(t *testing.T) {
	doc := document.NewDoc(document.NewCodeBlock("go", "x := 1"))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "GO") {
		t.Errorf("RenderANSI() = %q; want containing \"GO\" language label", out)
	}
	if !strings.Contains(out, "x := 1") {
		t.Errorf("RenderANSI() = %q; want containing \"x := 1\"", out)
	}
}

func TestRenderANSI_WrapWidth(t *testing.T) {
	long := strings.Repeat("word ", 20) // ~100 chars
	doc := document.NewDoc(document.NewParagraph(document.NewText(long)))
	out := document.RenderANSI(doc, document.ANSIConfig{WrapWidth: 40})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Errorf("RenderANSI() produced %d lines; want >= 2 with WrapWidth=40", len(lines))
	}
}
