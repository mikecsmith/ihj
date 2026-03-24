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
		t.Fatalf("node.Type = %v; want NodeDoc", node.Type)
	}
	if len(node.Children) != 1 {
		t.Fatalf("len(Children) = %d; want 1", len(node.Children))
	}
	p := node.Children[0]
	if p.Type != NodeParagraph {
		t.Fatalf("Children[0].Type = %v; want NodeParagraph", p.Type)
	}
	txt := p.Children[0]
	if txt.Text != "Hello world" {
		t.Fatalf("Text = %q; want \"Hello world\"", txt.Text)
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
		t.Errorf("Marks[0].Type = %v; want MarkBold", txt.Marks[0].Type)
	}
	if txt.Marks[1].Type != MarkLink {
		t.Errorf("Marks[1].Type = %v; want MarkLink", txt.Marks[1].Type)
	}
	if txt.Marks[1].Attrs["href"] != "https://example.com" {
		t.Errorf("Marks[1].Attrs[\"href\"] = %v; want \"https://example.com\"", txt.Marks[1].Attrs["href"])
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
		t.Errorf("Children[0] = {Type: %v, Level: %d}; want {NodeHeading, 3}", h.Type, h.Level)
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
		t.Errorf("Children[0] = {Type: %v, Language: %q}; want {NodeCodeBlock, \"go\"}", cb.Type, cb.Language)
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
		t.Errorf("row.Children[0].Type = %v; want NodeTableHeader", row.Children[0].Type)
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
		t.Errorf("PlainText() = %q; want containing \"inside panel\"", text)
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
		t.Errorf("Children[1].Level = %d; want 2 after roundtrip", node2.Children[1].Level)
	}
	if node2.Children[2].Language != "python" {
		t.Errorf("Children[2].Language = %q; want \"python\" after roundtrip", node2.Children[2].Language)
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
		t.Errorf("RenderMarkdown() = %q; want \"Hello world\"", md)
	}
}

func TestRenderMarkdown_Heading(t *testing.T) {
	doc := NewDoc(
		NewHeading(2, NewText("Title")),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "## Title") {
		t.Errorf("RenderMarkdown() = %q; want containing \"## Title\"", md)
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
		t.Errorf("RenderMarkdown() = %q; want containing \"- one\" and \"- two\"", md)
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
		t.Errorf("RenderMarkdown() = %q; want containing \"1. first\" and \"2. second\"", md)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	doc := NewDoc(
		NewCodeBlock("go", "func main() {}"),
	)
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "```go") || !strings.Contains(md, "func main() {}") {
		t.Errorf("RenderMarkdown() = %q; want containing code fence and content", md)
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
	doc := NewDoc(
		NewBlockquote(NewParagraph(NewText("quoted text"))),
	)
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

// ──────────────────────────────────────────────────────────────
// ANSI Rendering
// ──────────────────────────────────────────────────────────────

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
		t.Errorf("PlainText() = %q; want \"Hello bold world Title\"", text)
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
		t.Errorf("Truncate(doc, 2) children = %d; want 2", len(truncated.Children))
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
		t.Errorf("Walk() collected texts = %v; want [a b c]", texts)
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
		t.Errorf("Walk() with skip collected texts = %v; want [visible]", texts)
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
		t.Errorf("Transform() children = %d; want 2", len(result.Children))
	}
}

func TestHasMarkAndGetAttr(t *testing.T) {
	node := NewStyledText("click", Bold(), Link("https://x.com"))
	if !HasMark(node, MarkBold) {
		t.Errorf("HasMark(node, MarkBold) = false; want true")
	}
	if !HasMark(node, MarkLink) {
		t.Errorf("HasMark(node, MarkLink) = false; want true")
	}
	if HasMark(node, MarkItalic) {
		t.Errorf("HasMark(node, MarkItalic) = true; want false")
	}
	href := GetMarkAttr(node, MarkLink, "href")
	if href != "https://x.com" {
		t.Errorf("GetMarkAttr(node, MarkLink, \"href\") = %q; want \"https://x.com\"", href)
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
		t.Errorf("len(Children) = %d; want 0 for empty doc", len(node.Children))
	}
	// All renderers should handle empty docs gracefully.
	if md := RenderMarkdown(node); strings.TrimSpace(md) != "" {
		t.Errorf("RenderMarkdown(empty) = %q; want empty", md)
	}
	if out := RenderANSI(node, ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(empty) = %q; want empty", out)
	}
}

func TestParseADF_NilInput(t *testing.T) {
	if md := RenderMarkdown(nil); md != "" {
		t.Errorf("RenderMarkdown(nil) = %q; want empty", md)
	}
	if out := RenderANSI(nil, ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(nil) = %q; want empty", out)
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

// ──────────────────────────────────────────────────────────────
// Regression: NodeDoc must NOT insert extra blank lines
// ──────────────────────────────────────────────────────────────

func TestRenderMarkdown_NoExtraBlankLines(t *testing.T) {
	p := func(text string) *Node {
		return &Node{Type: NodeParagraph, Children: []*Node{{Type: NodeText, Text: text}}}
	}

	tests := []struct {
		name string
		doc  *Node
		want string
	}{
		{
			"two paragraphs",
			&Node{Type: NodeDoc, Children: []*Node{p("First"), p("Second")}},
			"First\n\nSecond\n",
		},
		{
			"heading then paragraph",
			&Node{Type: NodeDoc, Children: []*Node{
				{Type: NodeHeading, Level: 1, Children: []*Node{{Type: NodeText, Text: "Title"}}},
				p("Body"),
			}},
			"# Title\n\nBody\n",
		},
		{
			"paragraph then list",
			&Node{Type: NodeDoc, Children: []*Node{
				p("Intro"),
				{Type: NodeBulletList, Children: []*Node{
					{Type: NodeListItem, Children: []*Node{p("A")}},
					{Type: NodeListItem, Children: []*Node{p("B")}},
				}},
			}},
			"",
		},
		{
			"paragraph then code block",
			&Node{Type: NodeDoc, Children: []*Node{
				p("Before"),
				{Type: NodeCodeBlock, Language: "go", Children: []*Node{{Type: NodeText, Text: "x := 1\n"}}},
			}},
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
