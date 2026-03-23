package document

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	ansiBold  = "\033[1m"
	ansiReset = "\033[0m"
)

// ──────────────────────────────────────────────────────────────
// ADF Parsing
// ──────────────────────────────────────────────────────────────

func TestParseADF_SimpleDoc(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "paragraph",
				"content": [
					{"type": "text", "text": "Hello world"}
				]
			}
		]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatalf("ParseADF failed: %v", err)
	}
	if node.Type != NodeDoc {
		t.Fatalf("expected NodeDoc, got %v", node.Type)
	}
	if len(node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(node.Children))
	}
	p := node.Children[0]
	if p.Type != NodeParagraph {
		t.Fatalf("expected NodeParagraph, got %v", p.Type)
	}
	txt := p.Children[0]
	if txt.Text != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", txt.Text)
	}
}

func TestParseADF_Marks(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "paragraph",
			"content": [{
				"type": "text",
				"text": "bold link",
				"marks": [
					{"type": "strong"},
					{"type": "link", "attrs": {"href": "https://example.com"}}
				]
			}]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}

	txt := node.Children[0].Children[0]
	if len(txt.Marks) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(txt.Marks))
	}
	if txt.Marks[0].Type != MarkBold {
		t.Errorf("expected MarkBold, got %v", txt.Marks[0].Type)
	}
	if txt.Marks[1].Type != MarkLink {
		t.Errorf("expected MarkLink, got %v", txt.Marks[1].Type)
	}
	if txt.Marks[1].Attrs["href"] != "https://example.com" {
		t.Errorf("expected href, got %v", txt.Marks[1].Attrs)
	}
}

func TestParseADF_Heading(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "heading",
			"attrs": {"level": 3},
			"content": [{"type": "text", "text": "Section"}]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	h := node.Children[0]
	if h.Type != NodeHeading || h.Level != 3 {
		t.Errorf("expected heading level 3, got type=%v level=%d", h.Type, h.Level)
	}
}

func TestParseADF_Lists(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "bulletList",
			"content": [
				{
					"type": "listItem",
					"content": [{
						"type": "paragraph",
						"content": [{"type": "text", "text": "item one"}]
					}]
				},
				{
					"type": "listItem",
					"content": [{
						"type": "paragraph",
						"content": [{"type": "text", "text": "item two"}]
					}]
				}
			]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	list := node.Children[0]
	if list.Type != NodeBulletList {
		t.Fatalf("expected bullet list, got %v", list.Type)
	}
	if len(list.Children) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list.Children))
	}
}

func TestParseADF_CodeBlock(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "codeBlock",
			"attrs": {"language": "go"},
			"content": [{"type": "text", "text": "fmt.Println(\"hi\")"}]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	cb := node.Children[0]
	if cb.Type != NodeCodeBlock || cb.Language != "go" {
		t.Errorf("expected go codeblock, got type=%v lang=%q", cb.Type, cb.Language)
	}
}

func TestParseADF_Table(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "table",
			"content": [{
				"type": "tableRow",
				"content": [
					{
						"type": "tableHeader",
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Name"}]}]
					},
					{
						"type": "tableCell",
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Value"}]}]
					}
				]
			}]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	table := node.Children[0]
	if table.Type != NodeTable {
		t.Fatalf("expected table, got %v", table.Type)
	}
	row := table.Children[0]
	if len(row.Children) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(row.Children))
	}
	if row.Children[0].Type != NodeTableHeader {
		t.Errorf("expected table header, got %v", row.Children[0].Type)
	}
}

func TestParseADF_UnknownNodes(t *testing.T) {
	// Unknown node types with children should preserve content.
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "panel",
			"content": [{
				"type": "paragraph",
				"content": [{"type": "text", "text": "inside panel"}]
			}]
		}]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	// Should be wrapped in a paragraph fallback.
	text := PlainText(node)
	if !strings.Contains(text, "inside panel") {
		t.Errorf("expected preserved text, got %q", text)
	}
}

// ──────────────────────────────────────────────────────────────
// ADF Roundtrip (ADF → AST → ADF)
// ──────────────────────────────────────────────────────────────

func TestADFRoundtrip(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "paragraph",
				"content": [
					{"type": "text", "text": "plain "},
					{"type": "text", "text": "bold", "marks": [{"type": "strong"}]},
					{"type": "text", "text": " and "},
					{"type": "text", "text": "linked", "marks": [{"type": "link", "attrs": {"href": "https://x.com"}}]}
				]
			},
			{
				"type": "heading",
				"attrs": {"level": 2},
				"content": [{"type": "text", "text": "Title"}]
			},
			{
				"type": "codeBlock",
				"attrs": {"language": "python"},
				"content": [{"type": "text", "text": "print('hi')"}]
			}
		]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}

	out, err := RenderADF(node)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse and verify structural equivalence.
	node2, err := ParseADF(out)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}

	if len(node2.Children) != 3 {
		t.Fatalf("expected 3 blocks after roundtrip, got %d", len(node2.Children))
	}
	if node2.Children[1].Level != 2 {
		t.Errorf("heading level lost in roundtrip")
	}
	if node2.Children[2].Language != "python" {
		t.Errorf("code language lost in roundtrip")
	}
}

// ──────────────────────────────────────────────────────────────
// Markdown Rendering
// ──────────────────────────────────────────────────────────────

func TestRenderMarkdown_Paragraph(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("Hello world")),
	)
	md := RenderMarkdown(doc)
	if strings.TrimSpace(md) != "Hello world" {
		t.Errorf("got %q", md)
	}
}

func TestRenderMarkdown_Heading(t *testing.T) {
	doc := NewDoc(
		NewHeading(2, NewText("Title")),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "## Title") {
		t.Errorf("got %q", md)
	}
}

func TestRenderMarkdown_InlineMarks(t *testing.T) {
	doc := NewDoc(
		NewParagraph(
			NewText("before "),
			NewStyledText("bold", Bold()),
			NewText(" and "),
			NewStyledText("italic", Italic()),
			NewText(" and "),
			NewStyledText("code", Code()),
			NewText(" and "),
			NewStyledText("click", Bold(), Link("https://x.com")),
		),
	)
	md := RenderMarkdown(doc)

	checks := []string{"**bold**", "*italic*", "`code`", "[**click**](https://x.com)"}
	for _, c := range checks {
		if !strings.Contains(md, c) {
			t.Errorf("missing %q in:\n%s", c, md)
		}
	}
}

func TestRenderMarkdown_BulletList(t *testing.T) {
	doc := NewDoc(
		NewBulletList(
			NewListItem(NewParagraph(NewText("one"))),
			NewListItem(NewParagraph(NewText("two"))),
		),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "- one") || !strings.Contains(md, "- two") {
		t.Errorf("got %q", md)
	}
}

func TestRenderMarkdown_OrderedList(t *testing.T) {
	doc := NewDoc(
		NewOrderedList(
			NewListItem(NewParagraph(NewText("first"))),
			NewListItem(NewParagraph(NewText("second"))),
		),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "1. first") || !strings.Contains(md, "2. second") {
		t.Errorf("got %q", md)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	doc := NewDoc(
		NewCodeBlock("go", "func main() {}"),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "```go") || !strings.Contains(md, "func main() {}") {
		t.Errorf("got %q", md)
	}
}

func TestRenderMarkdown_Table(t *testing.T) {
	doc := NewDoc(
		NewTable(
			NewTableRow(
				NewTableHeader(1, 1, NewParagraph(NewText("Col A"))),
				NewTableHeader(1, 1, NewParagraph(NewText("Col B"))),
			),
			NewTableRow(
				NewTableCell(1, 1, NewParagraph(NewText("val 1"))),
				NewTableCell(1, 1, NewParagraph(NewText("val 2"))),
			),
		),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "| Col A | Col B |") {
		t.Errorf("missing table header in:\n%s", md)
	}
	if !strings.Contains(md, "| --- | --- |") {
		t.Errorf("missing separator in:\n%s", md)
	}
	if !strings.Contains(md, "| val 1 | val 2 |") {
		t.Errorf("missing table row in:\n%s", md)
	}
}

func TestRenderMarkdown_Blockquote(t *testing.T) {
	doc := NewDoc(
		NewBlockquote(NewParagraph(NewText("quoted text"))),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "> quoted text") {
		t.Errorf("got %q", md)
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
		t.Errorf("missing rule in:\n%s", md)
	}
}

// ──────────────────────────────────────────────────────────────
// ANSI Rendering
// ──────────────────────────────────────────────────────────────

func TestRenderANSI_PlainParagraph(t *testing.T) {
	doc := NewDoc(NewParagraph(NewText("Hello")))
	out := RenderANSI(doc, ANSIConfig{})
	if !strings.Contains(out, "Hello") {
		t.Errorf("got %q", out)
	}
}

func TestRenderANSI_BoldText(t *testing.T) {
	doc := NewDoc(NewParagraph(NewStyledText("bold", Bold())))
	out := RenderANSI(doc, ANSIConfig{Styles: ANSIStyles{}})
	if !strings.Contains(out, ansiBold) {
		t.Error("missing bold escape")
	}
	if !strings.Contains(out, "bold") {
		t.Error("missing text content")
	}
}

func TestRenderANSI_Link(t *testing.T) {
	doc := NewDoc(NewParagraph(NewStyledText("click", Link("https://x.com"))))
	out := RenderANSI(doc, ANSIConfig{Styles: ANSIStyles{}})
	// Should contain OSC 8 hyperlink escape.
	if !strings.Contains(out, "\033]8;;https://x.com\a") {
		t.Error("missing OSC 8 link start")
	}
	if !strings.Contains(out, "\033]8;;\a") {
		t.Error("missing OSC 8 link end")
	}
}

func TestRenderANSI_CodeBlock(t *testing.T) {
	doc := NewDoc(NewCodeBlock("go", "x := 1"))
	out := RenderANSI(doc, ANSIConfig{})
	if !strings.Contains(out, "GO") {
		t.Error("missing language label")
	}
	if !strings.Contains(out, "x := 1") {
		t.Error("missing code content")
	}
}

func TestRenderANSI_WrapWidth(t *testing.T) {
	long := strings.Repeat("word ", 20) // ~100 chars
	doc := NewDoc(NewParagraph(NewText(long)))
	out := RenderANSI(doc, ANSIConfig{WrapWidth: 40})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapping at width 40, got %d lines", len(lines))
	}
}

// ──────────────────────────────────────────────────────────────
// Tree Utilities
// ──────────────────────────────────────────────────────────────

func TestPlainText(t *testing.T) {
	doc := NewDoc(
		NewParagraph(
			NewText("Hello "),
			NewStyledText("bold", Bold()),
			NewText(" world"),
		),
		NewHeading(2, NewText("Title")),
	)
	text := PlainText(doc)
	if text != "Hello bold world Title" {
		t.Errorf("got %q", text)
	}
}

func TestTruncate(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("one")),
		NewParagraph(NewText("two")),
		NewParagraph(NewText("three")),
	)
	truncated := Truncate(doc, 2)
	if len(truncated.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(truncated.Children))
	}
	// Original should be unchanged.
	if len(doc.Children) != 3 {
		t.Error("truncate mutated original")
	}
}

func TestWalk(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("a"), NewText("b")),
		NewParagraph(NewText("c")),
	)
	var texts []string
	Walk(doc, func(n *Node) bool {
		if n.Type == NodeText {
			texts = append(texts, n.Text)
		}
		return true
	})
	if len(texts) != 3 || texts[0] != "a" || texts[1] != "b" || texts[2] != "c" {
		t.Errorf("got %v", texts)
	}
}

func TestWalk_SkipSubtree(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("visible")),
		NewCodeBlock("go", "hidden"),
	)
	var texts []string
	Walk(doc, func(n *Node) bool {
		if n.Type == NodeCodeBlock {
			return false // Skip code block subtree.
		}
		if n.Type == NodeText {
			texts = append(texts, n.Text)
		}
		return true
	})
	if len(texts) != 1 || texts[0] != "visible" {
		t.Errorf("expected only 'visible', got %v", texts)
	}
}

func TestTransform_RemoveNodes(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("keep")),
		NewCodeBlock("go", "remove me"),
		NewParagraph(NewText("also keep")),
	)
	result := Transform(doc, func(n *Node) *Node {
		if n.Type == NodeCodeBlock {
			return nil
		}
		return n
	})
	if len(result.Children) != 2 {
		t.Errorf("expected 2 children after transform, got %d", len(result.Children))
	}
}

func TestHasMarkAndGetAttr(t *testing.T) {
	node := NewStyledText("click", Bold(), Link("https://x.com"))
	if !HasMark(node, MarkBold) {
		t.Error("expected MarkBold")
	}
	if !HasMark(node, MarkLink) {
		t.Error("expected MarkLink")
	}
	if HasMark(node, MarkItalic) {
		t.Error("should not have MarkItalic")
	}
	href := GetMarkAttr(node, MarkLink, "href")
	if href != "https://x.com" {
		t.Errorf("expected href, got %q", href)
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

// ──────────────────────────────────────────────────────────────
// Full Roundtrip: ADF → AST → Markdown → verify content
// ──────────────────────────────────────────────────────────────

func TestFullRoundtrip_ADFToMarkdown(t *testing.T) {
	adf := `{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "heading",
				"attrs": {"level": 2},
				"content": [{"type": "text", "text": "Acceptance Criteria"}]
			},
			{
				"type": "bulletList",
				"content": [
					{
						"type": "listItem",
						"content": [{
							"type": "paragraph",
							"content": [
								{"type": "text", "text": "Given "},
								{"type": "text", "text": "a user", "marks": [{"type": "strong"}]},
								{"type": "text", "text": " when they click submit"}
							]
						}]
					}
				]
			},
			{
				"type": "codeBlock",
				"attrs": {"language": "sql"},
				"content": [{"type": "text", "text": "SELECT * FROM users;"}]
			}
		]
	}`

	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}

	md := RenderMarkdown(node)

	checks := []string{
		"## Acceptance Criteria",
		"**a user**",
		"when they click submit",
		"```sql",
		"SELECT * FROM users;",
	}
	for _, c := range checks {
		if !strings.Contains(md, c) {
			t.Errorf("missing %q in:\n%s", c, md)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Edge Cases
// ──────────────────────────────────────────────────────────────

func TestParseADF_EmptyDoc(t *testing.T) {
	adf := `{"version": 1, "type": "doc", "content": []}`
	node, err := ParseADF([]byte(adf))
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 0 {
		t.Errorf("expected empty doc, got %d children", len(node.Children))
	}
	// All renderers should handle empty docs gracefully.
	if md := RenderMarkdown(node); strings.TrimSpace(md) != "" {
		t.Errorf("expected empty markdown, got %q", md)
	}
	if out := RenderANSI(node, ANSIConfig{}); out != "" {
		t.Errorf("expected empty ANSI, got %q", out)
	}
}

func TestParseADF_NilInput(t *testing.T) {
	if md := RenderMarkdown(nil); md != "" {
		t.Errorf("expected empty for nil, got %q", md)
	}
	if out := RenderANSI(nil, ANSIConfig{}); out != "" {
		t.Errorf("expected empty for nil, got %q", out)
	}
}

func TestRenderADF_NilNode(t *testing.T) {
	out, err := RenderADF(nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "doc" {
		t.Errorf("nil should produce empty doc, got %v", parsed)
	}
}
