package document

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Paragraph(t *testing.T) {
	doc := NewDoc(NewParagraph(NewText("Hello world")))
	md := RenderMarkdown(doc)
	if strings.TrimSpace(md) != "Hello world" {
		t.Errorf("RenderMarkdown() = %q; want \"Hello world\"", md)
	}
}

func TestRenderMarkdown_Heading(t *testing.T) {
	doc := NewDoc(NewHeading(2, NewText("Title")))
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "## Title") {
		t.Errorf("RenderMarkdown() = %q; want containing \"## Title\"", md)
	}
}

func TestRenderMarkdown_InlineMarks(t *testing.T) {
	doc := NewDoc(NewParagraph(
		NewText("before "),
		NewStyledText("bold", Bold()),
		NewText(" and "),
		NewStyledText("italic", Italic()),
		NewText(" and "),
		NewStyledText("code", Code()),
		NewText(" and "),
		NewStyledText("click", Bold(), Link("https://x.com")),
	))
	md := RenderMarkdown(doc)

	checks := []string{"**bold**", "*italic*", "`code`", "[**click**](https://x.com)"}
	for _, c := range checks {
		if !strings.Contains(md, c) {
			t.Errorf("missing %q in:\n%s", c, md)
		}
	}
}

func TestRenderMarkdown_BulletList(t *testing.T) {
	doc := NewDoc(NewBulletList(
		NewListItem(NewParagraph(NewText("one"))),
		NewListItem(NewParagraph(NewText("two"))),
	))
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "- one") || !strings.Contains(md, "- two") {
		t.Errorf("RenderMarkdown() = %q; want containing \"- one\" and \"- two\"", md)
	}
}

func TestRenderMarkdown_OrderedList(t *testing.T) {
	doc := NewDoc(NewOrderedList(
		NewListItem(NewParagraph(NewText("first"))),
		NewListItem(NewParagraph(NewText("second"))),
	))
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "1. first") || !strings.Contains(md, "2. second") {
		t.Errorf("RenderMarkdown() = %q; want containing \"1. first\" and \"2. second\"", md)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	doc := NewDoc(NewCodeBlock("go", "func main() {}"))
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "```go") || !strings.Contains(md, "func main() {}") {
		t.Errorf("RenderMarkdown() = %q; want containing code fence and content", md)
	}
}

func TestRenderMarkdown_Table(t *testing.T) {
	doc := NewDoc(NewTable(
		NewTableRow(
			NewTableHeader(NewParagraph(NewText("Col A"))),
			NewTableHeader(NewParagraph(NewText("Col B"))),
		),
		NewTableRow(
			NewTableCell(NewParagraph(NewText("val 1"))),
			NewTableCell(NewParagraph(NewText("val 2"))),
		),
	))
	md := RenderMarkdown(doc)
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
	doc := NewDoc(NewBlockquote(NewParagraph(NewText("quoted text"))))
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "> quoted text") {
		t.Errorf("RenderMarkdown() = %q; want containing \"> quoted text\"", md)
	}
}

func TestRenderMarkdown_Rule(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("before")),
		NewRule(),
		NewParagraph(NewText("after")),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "---") {
		t.Errorf("RenderMarkdown() = %q; want containing \"---\"", md)
	}
}

// Regression: NodeDoc must NOT insert extra blank lines.
func TestRenderMarkdown_NoExtraBlankLines(t *testing.T) {
	p := func(text string) *Node { return NewParagraph(NewText(text)) }

	tests := []struct {
		name string
		doc  *Node
		want string
	}{
		{
			"two paragraphs",
			NewDoc(p("First"), p("Second")),
			"First\n\nSecond\n",
		},
		{
			"heading then paragraph",
			NewDoc(NewHeading(1, NewText("Title")), p("Body")),
			"# Title\n\nBody\n",
		},
		{
			"paragraph then list",
			NewDoc(
				p("Intro"),
				NewBulletList(
					NewListItem(p("A")),
					NewListItem(p("B")),
				),
			),
			"",
		},
		{
			"paragraph then code block",
			NewDoc(p("Before"), NewCodeBlock("go", "x := 1\n")),
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderMarkdown(tt.doc)
			if strings.Contains(got, "\n\n\n") {
				t.Errorf("RenderMarkdown() contains triple newline (spurious blank line):\n%q", got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("RenderMarkdown() = %q; want %q", got, tt.want)
			}
		})
	}
}
