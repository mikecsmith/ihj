package document_test

import (
	"strings"
	"testing"

	"charm.land/glamour/v2/ansi"
	"github.com/mikecsmith/ihj/internal/document"
)

func TestRenderANSI_PlainParagraph(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewText("Hello")))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "Hello") {
		t.Errorf("RenderANSI() = %q; want containing \"Hello\"", out)
	}
}

func TestRenderANSI_BoldText(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewStyledText("bold", document.Bold())))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "bold") {
		t.Errorf("RenderANSI() = %q; want containing \"bold\"", out)
	}
	// Glamour renders bold with ANSI escape sequences — verify some
	// escape is present (exact codes depend on theme).
	if !strings.Contains(out, "\033[") {
		t.Errorf("RenderANSI() = %q; want containing ANSI escape sequences", out)
	}
}

func TestRenderANSI_Link(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewStyledText("click", document.Link("https://x.com"))))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	// Glamour renders links with OSC 8 hyperlink sequences.
	if !strings.Contains(out, "click") {
		t.Errorf("RenderANSI() = %q; want containing \"click\"", out)
	}
	if !strings.Contains(out, "https://x.com") {
		t.Errorf("RenderANSI() = %q; want containing URL", out)
	}
}

func TestRenderANSI_CodeBlock(t *testing.T) {
	doc := document.NewDoc(document.NewCodeBlock("go", "x := 1"))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	// Glamour renders code with syntax highlighting — the code content
	// may be split across escape sequences, so check for the variable name.
	if !strings.Contains(out, "x") {
		t.Errorf("RenderANSI() = %q; want containing \"x\"", out)
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

func TestRenderANSI_NilNode(t *testing.T) {
	out := document.RenderANSI(nil, document.ANSIConfig{})
	if out != "" {
		t.Errorf("RenderANSI(nil) = %q; want empty", out)
	}
}

func TestRenderANSI_EmptyDoc(t *testing.T) {
	doc := document.NewDoc()
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if out != "" {
		t.Errorf("RenderANSI(empty doc) = %q; want empty", out)
	}
}

func TestRenderANSI_Heading(t *testing.T) {
	doc := document.NewDoc(document.NewHeading(2, document.NewText("Title")))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "Title") {
		t.Errorf("RenderANSI() = %q; want containing \"Title\"", out)
	}
}

func TestRenderANSI_BulletList(t *testing.T) {
	doc := document.NewDoc(document.NewBulletList(
		document.NewListItem(document.NewParagraph(document.NewText("one"))),
		document.NewListItem(document.NewParagraph(document.NewText("two"))),
	))
	out := document.RenderANSI(doc, document.ANSIConfig{})
	if !strings.Contains(out, "one") || !strings.Contains(out, "two") {
		t.Errorf("RenderANSI() = %q; want containing \"one\" and \"two\"", out)
	}
}

func TestRenderANSI_CustomStyle(t *testing.T) {
	bold := true
	style := ansi.StyleConfig{
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  &bold,
				Upper: &bold,
			},
		},
	}
	doc := document.NewDoc(document.NewHeading(1, document.NewText("hello")))
	out := document.RenderANSI(doc, document.ANSIConfig{Style: &style})
	if !strings.Contains(out, "HELLO") {
		t.Errorf("RenderANSI(custom style) = %q; want uppercase \"HELLO\"", out)
	}
}

func TestContentTheme_DefaultReturnsNonNil(t *testing.T) {
	for _, name := range []string{"", "default"} {
		s := document.ContentTheme(name)
		if s == nil {
			t.Errorf("ContentTheme(%q) = nil; want non-nil", name)
		}
	}
}

func TestContentTheme_BuiltinThemes(t *testing.T) {
	for _, name := range []string{"dark", "light", "dracula", "tokyo-night", "pink", "ascii", "notty"} {
		s := document.ContentTheme(name)
		if s == nil {
			t.Errorf("ContentTheme(%q) = nil; want non-nil", name)
		}
	}
}

func TestContentTheme_UnknownFallsToDefault(t *testing.T) {
	s := document.ContentTheme("nonexistent-theme")
	if s == nil {
		t.Fatal("ContentTheme(unknown) = nil; want default")
	}
	def := document.ContentTheme("default")
	if s != def {
		t.Error("ContentTheme(unknown) should return the same pointer as ContentTheme(default)")
	}
}

// TestContentTheme_CodeBlockRendersWithoutPanic verifies that the Chroma
// config in each theme uses valid color values. Chroma requires hex colors
// (e.g. "#FF5F5F"), not ANSI numbers ("1"). Invalid colors cause a panic
// inside glamour when rendering code blocks.
func TestContentTheme_CodeBlockRendersWithoutPanic(t *testing.T) {
	themes := []string{"", "default", "dark", "light", "dracula", "tokyo-night", "pink", "ascii", "notty"}
	doc := document.NewDoc(document.NewCodeBlock("go", "func main() {\n\tx := 42\n\tfmt.Println(x)\n}"))

	for _, name := range themes {
		t.Run(name, func(t *testing.T) {
			style := document.ContentTheme(name)
			// This will panic if the Chroma config contains invalid color values.
			out := document.RenderANSI(doc, document.ANSIConfig{
				WrapWidth: 80,
				Style:     style,
			})
			if !strings.Contains(out, "main") {
				t.Errorf("RenderANSI(%q) = %q; want containing \"main\"", name, out)
			}
		})
	}
}
