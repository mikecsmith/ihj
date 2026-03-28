package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

// --- ParseADF tests ---

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
	if node.Type != document.NodeDoc {
		t.Fatalf("node.Type = %v; want NodeDoc", node.Type)
	}
	if len(node.Children) != 1 {
		t.Fatalf("len(Children) = %d; want 1", len(node.Children))
	}
	p := node.Children[0]
	if p.Type != document.NodeParagraph {
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
	if txt.Marks[0].Type != document.MarkBold {
		t.Errorf("Marks[0].Type = %v; want MarkBold", txt.Marks[0].Type)
	}
	if txt.Marks[1].Type != document.MarkLink {
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
	if h.Type != document.NodeHeading || h.Level != 3 {
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
	if list.Type != document.NodeBulletList {
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
	if cb.Type != document.NodeCodeBlock || cb.Language != "go" {
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
	if table.Type != document.NodeTable {
		t.Fatalf("expected table, got %v", table.Type)
	}
	row := table.Children[0]
	if len(row.Children) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(row.Children))
	}
	if row.Children[0].Type != document.NodeTableHeader {
		t.Errorf("row.Children[0].Type = %v; want NodeTableHeader", row.Children[0].Type)
	}
}

func TestParseADF_UnknownNodes(t *testing.T) {
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
	// Verify unknown nodes preserve their children: doc > generic > paragraph > text.
	if len(node.Children) == 0 {
		t.Fatal("doc has no children; want generic wrapper for panel")
	}
	panel := node.Children[0]
	if len(panel.Children) == 0 || len(panel.Children[0].Children) == 0 {
		t.Fatal("panel content not preserved")
	}
	if text := panel.Children[0].Children[0].Text; text != "inside panel" {
		t.Errorf("text = %q; want \"inside panel\"", text)
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
	if md := document.RenderMarkdown(node); strings.TrimSpace(md) != "" {
		t.Errorf("RenderMarkdown(empty) = %q; want empty", md)
	}
	if out := document.RenderANSI(node, document.ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(empty) = %q; want empty", out)
	}
}

// --- RenderADFValue tests ---

func TestRenderADFValue_NilNode(t *testing.T) {
	parsed := RenderADFValue(nil)
	if parsed["type"] != "doc" {
		t.Errorf("nil should produce empty doc, got %v", parsed)
	}
}

// --- Roundtrip tests ---

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

	out, err := json.Marshal(RenderADFValue(node))
	if err != nil {
		t.Fatal(err)
	}

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

	md := document.RenderMarkdown(node)

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
