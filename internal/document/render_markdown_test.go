package document_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

func TestRenderMarkdown_Paragraph(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(document.NewText("Hello world")))
	md := document.RenderMarkdown(doc)
	if strings.TrimSpace(md) != "Hello world" {
		t.Errorf("RenderMarkdown() = %q; want \"Hello world\"", md)
	}
}

func TestRenderMarkdown_Heading(t *testing.T) {
	doc := document.NewDoc(document.NewHeading(2, document.NewText("Title")))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "## Title") {
		t.Errorf("RenderMarkdown() = %q; want containing \"## Title\"", md)
	}
}

func TestRenderMarkdown_InlineMarks(t *testing.T) {
	doc := document.NewDoc(document.NewParagraph(
		document.NewText("before "),
		document.NewStyledText("bold", document.Bold()),
		document.NewText(" and "),
		document.NewStyledText("italic", document.Italic()),
		document.NewText(" and "),
		document.NewStyledText("code", document.Code()),
		document.NewText(" and "),
		document.NewStyledText("click", document.Bold(), document.Link("https://x.com")),
	))
	md := document.RenderMarkdown(doc)

	checks := []string{"**bold**", "*italic*", "`code`", "[**click**](https://x.com)"}
	for _, c := range checks {
		if !strings.Contains(md, c) {
			t.Errorf("missing %q in:\n%s", c, md)
		}
	}
}

func TestRenderMarkdown_BulletList(t *testing.T) {
	doc := document.NewDoc(document.NewBulletList(
		document.NewListItem(document.NewParagraph(document.NewText("one"))),
		document.NewListItem(document.NewParagraph(document.NewText("two"))),
	))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "- one") || !strings.Contains(md, "- two") {
		t.Errorf("RenderMarkdown() = %q; want containing \"- one\" and \"- two\"", md)
	}
}

func TestRenderMarkdown_OrderedList(t *testing.T) {
	doc := document.NewDoc(document.NewOrderedList(
		document.NewListItem(document.NewParagraph(document.NewText("first"))),
		document.NewListItem(document.NewParagraph(document.NewText("second"))),
	))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "1. first") || !strings.Contains(md, "2. second") {
		t.Errorf("RenderMarkdown() = %q; want containing \"1. first\" and \"2. second\"", md)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	doc := document.NewDoc(document.NewCodeBlock("go", "func main() {}"))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "```go") || !strings.Contains(md, "func main() {}") {
		t.Errorf("RenderMarkdown() = %q; want containing code fence and content", md)
	}
}

func TestRenderMarkdown_Table(t *testing.T) {
	doc := document.NewDoc(document.NewTable(
		document.NewTableRow(
			document.NewTableHeader(document.NewParagraph(document.NewText("Col A"))),
			document.NewTableHeader(document.NewParagraph(document.NewText("Col B"))),
		),
		document.NewTableRow(
			document.NewTableCell(document.NewParagraph(document.NewText("val 1"))),
			document.NewTableCell(document.NewParagraph(document.NewText("val 2"))),
		),
	))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "| Col A | Col B |") {
		t.Errorf("RenderMarkdown() = %q; want containing \"| Col A | Col B |\"", md)
	}
	if !strings.Contains(md, "| --- | --- |") {
		t.Errorf("RenderMarkdown() = %q; want containing \"| --- | --- |\"", md)
	}
	if !strings.Contains(md, "| val 1 | val 2 |") {
		t.Errorf("RenderMarkdown() = %q; want containing \"| val 1 | val 2 |\"", md)
	}
}

func TestRenderMarkdown_Blockquote(t *testing.T) {
	doc := document.NewDoc(document.NewBlockquote(document.NewParagraph(document.NewText("quoted text"))))
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "> quoted text") {
		t.Errorf("RenderMarkdown() = %q; want containing \"> quoted text\"", md)
	}
}

func TestRenderMarkdown_Rule(t *testing.T) {
	doc := document.NewDoc(
		document.NewParagraph(document.NewText("before")),
		document.NewRule(),
		document.NewParagraph(document.NewText("after")),
	)
	md := document.RenderMarkdown(doc)
	if !strings.Contains(md, "---") {
		t.Errorf("RenderMarkdown() = %q; want containing \"---\"", md)
	}
}

// Regression: NodeDoc must NOT insert extra blank lines.
func TestRenderMarkdown_NoExtraBlankLines(t *testing.T) {
	p := func(text string) *document.Node { return document.NewParagraph(document.NewText(text)) }

	tests := []struct {
		name string
		doc  *document.Node
		want string
	}{
		{
			"two paragraphs",
			document.NewDoc(p("First"), p("Second")),
			"First\n\nSecond\n",
		},
		{
			"heading then paragraph",
			document.NewDoc(document.NewHeading(1, document.NewText("Title")), p("Body")),
			"# Title\n\nBody\n",
		},
		{
			"paragraph then list",
			document.NewDoc(
				p("Intro"),
				document.NewBulletList(
					document.NewListItem(p("A")),
					document.NewListItem(p("B")),
				),
			),
			"",
		},
		{
			"paragraph then code block",
			document.NewDoc(p("Before"), document.NewCodeBlock("go", "x := 1\n")),
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := document.RenderMarkdown(tt.doc)
			if strings.Contains(got, "\n\n\n") {
				t.Errorf("RenderMarkdown() contains triple newline (spurious blank line):\n%q", got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("RenderMarkdown() = %q; want %q", got, tt.want)
			}
		})
	}
}
