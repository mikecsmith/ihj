package document

import (
	"strings"
	"testing"
)

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

// TestParseADF_NilInput tests that renderers handle nil nodes gracefully.
// Despite its name, this test exercises RenderMarkdown and RenderANSI, not ParseADF.
func TestParseADF_NilInput(t *testing.T) {
	if md := RenderMarkdown(nil); md != "" {
		t.Errorf("RenderMarkdown(nil) = %q; want empty", md)
	}
	if out := RenderANSI(nil, ANSIConfig{}); out != "" {
		t.Errorf("RenderANSI(nil) = %q; want empty", out)
	}
}
