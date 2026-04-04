package document_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/document"
)

func TestAST_MarkdownRoundTrip_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		// Empty structures
		{"empty doc", ""},
		{"whitespace only", "   \n\n  "},
		{"empty heading", "# \n"},
		{"empty blockquote", "> \n"},
		{"empty code block", "```\n```\n"},
		{"empty code block with lang", "```go\n```\n"},

		// Nested lists
		{"nested bullet", "- a\n  - b\n    - c\n"},
		{"nested ordered", "1. a\n   1. b\n"},
		{"mixed nested", "- a\n  1. b\n  2. c\n"},
		// Note: "- a\n  - \n    - c\n" (deeply nested with empty middle item)
		// is unstable — goldmark parses the empty item's nested children as
		// the first child of the ListItem (not preceded by a paragraph), which
		// causes the prefix and nested indent to merge on render. This is a
		// known limitation of the markdown round-trip for unusual nesting.

		// Inline marks
		{"bold", "**bold**\n"},
		{"italic", "*italic*\n"},
		{"code", "`code`\n"},
		{"strikethrough", "~~strike~~\n"},
		{"nested marks", "***bold italic***\n"},
		{"link", "[text](http://example.com)\n"},
		{"bold in link", "[**bold link**](http://example.com)\n"},
		{"code in list", "- `code` item\n"},

		// Complex structures
		{"heading then list", "# Title\n\n- a\n- b\n"},
		{"paragraph then code", "text\n\n```\ncode\n```\n"},
		{"blockquote with list", "> - item\n"},
		{"multi-paragraph blockquote", "> first\n>\n> second\n"},
		{"hr between content", "above\n\n---\n\nbelow\n"},

		// Tables
		{"simple table", "| a | b |\n| --- | --- |\n| 1 | 2 |\n"},
		{"table with marks", "| **bold** | *italic* |\n| --- | --- |\n| `code` | text |\n"},

		// Tricky whitespace
		{"trailing spaces", "text   \n"},
		{"multiple paragraphs", "first\n\nsecond\n\nthird\n"},
		{"list with blank between", "- a\n\n- b\n"},

		// Task lists / checkboxes (ADF has no native support — preserved as text)
		{"unchecked checkbox", "- [ ] todo\n"},
		{"checked checkbox", "- [x] done\n"},
		{"mixed checklist", "- [x] done\n- [ ] todo\n- [ ] another\n"},
		{"nested checklist", "- [ ] parent\n   - [ ] child\n"},
		{"checklist after heading", "# Tasks\n\n- [ ] first\n- [x] second\n"},

		// Special characters
		{"literal asterisks in code", "`**not bold**`\n"},
		{"backslash", "text with \\* escaped\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := document.ParseMarkdownString(tt.md)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			out := document.RenderMarkdown(node)

			// Parse again and render — must be stable
			node2, err := document.ParseMarkdownString(out)
			if err != nil {
				t.Fatalf("re-parse error: %v", err)
			}
			out2 := document.RenderMarkdown(node2)

			if out != out2 {
				t.Errorf("round-trip unstable:\n  first:  %q\n  second: %q", out, out2)
			}
		})
	}
}
