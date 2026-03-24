package document

import (
	"strings"
	"testing"
)

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
