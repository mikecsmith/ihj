package jira

import (
	"github.com/mikecsmith/ihj/internal/document"
)

// renderADFValue returns the ADF as a map[string]any suitable for embedding
// directly into a larger JSON payload (e.g. issue creation body).
func renderADFValue(node *document.Node) map[string]any {
	out := renderADFNode(node)
	if out == nil {
		return map[string]any{
			"version": 1,
			"type":    "doc",
			"content": []any{},
		}
	}
	return out
}

func renderADFNode(node *document.Node) map[string]any {
	if node == nil {
		return nil
	}

	switch node.Type {
	case document.NodeDoc:
		return map[string]any{
			"version": 1,
			"type":    "doc",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeParagraph:
		return map[string]any{
			"type":    "paragraph",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeHeading:
		return map[string]any{
			"type":    "heading",
			"attrs":   map[string]any{"level": node.Level},
			"content": renderADFChildren(node.Children),
		}

	case document.NodeText:
		out := map[string]any{
			"type": "text",
			"text": node.Text,
		}
		if marks := renderADFMarks(node.Marks); len(marks) > 0 {
			out["marks"] = marks
		}
		return out

	case document.NodeHardBreak:
		return map[string]any{"type": "hardBreak"}

	case document.NodeBulletList:
		return map[string]any{
			"type":    "bulletList",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeOrderedList:
		return map[string]any{
			"type":    "orderedList",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeListItem:
		content := renderADFChildren(node.Children)
		if len(content) == 0 {
			// ADF requires listItem to contain at least one block node.
			content = []any{map[string]any{"type": "paragraph", "content": []any{}}}
		}
		// ADF has no native checkbox support. Prepend [ ]/[x] as text to
		// the first paragraph so the checkbox state isn't silently lost.
		if node.CheckState != nil && len(content) > 0 {
			prefix := "[ ] "
			if *node.CheckState {
				prefix = "[x] "
			}
			if para, ok := content[0].(map[string]any); ok && para["type"] == "paragraph" {
				prefixNode := map[string]any{"type": "text", "text": prefix}
				existing, _ := para["content"].([]any)
				para["content"] = append([]any{prefixNode}, existing...)
			}
		}
		return map[string]any{
			"type":    "listItem",
			"content": content,
		}

	case document.NodeCodeBlock:
		out := map[string]any{
			"type":    "codeBlock",
			"content": renderADFChildren(node.Children),
		}
		if node.Language != "" {
			out["attrs"] = map[string]any{"language": node.Language}
		}
		return out

	case document.NodeBlockquote:
		return map[string]any{
			"type":    "blockquote",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeRule:
		return map[string]any{"type": "rule"}

	case document.NodeTable:
		return map[string]any{
			"type":    "table",
			"attrs":   map[string]any{"isNumberColumnEnabled": false, "layout": "default"},
			"content": renderADFChildren(node.Children),
		}

	case document.NodeTableRow:
		return map[string]any{
			"type":    "tableRow",
			"content": renderADFChildren(node.Children),
		}

	case document.NodeTableHeader:
		out := map[string]any{
			"type":    "tableHeader",
			"content": renderADFChildren(node.Children),
		}
		if node.ColSpan > 1 || node.RowSpan > 1 {
			out["attrs"] = map[string]any{
				"colspan": node.ColSpan,
				"rowspan": node.RowSpan,
			}
		}
		return out

	case document.NodeTableCell:
		out := map[string]any{
			"type":    "tableCell",
			"content": renderADFChildren(node.Children),
		}
		if node.ColSpan > 1 || node.RowSpan > 1 {
			out["attrs"] = map[string]any{
				"colspan": node.ColSpan,
				"rowspan": node.RowSpan,
			}
		}
		return out

	case document.NodeMedia:
		return map[string]any{
			"type": "mediaSingle",
			"content": []any{
				map[string]any{
					"type": "media",
					"attrs": map[string]any{
						"type": node.MediaType,
						"url":  node.URL,
						"alt":  node.Alt,
					},
				},
			},
		}

	default:
		return nil
	}
}

func renderADFChildren(children []*document.Node) []any {
	if len(children) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(children))
	for _, child := range children {
		rendered := renderADFNode(child)
		if rendered != nil {
			out = append(out, rendered)
		}
	}
	return out
}

func renderADFMarks(marks []document.Mark) []any {
	if len(marks) == 0 {
		return nil
	}
	out := make([]any, 0, len(marks))
	for _, m := range marks {
		rendered := renderADFMark(m)
		if rendered != nil {
			out = append(out, rendered)
		}
	}
	return out
}

func renderADFMark(m document.Mark) map[string]any {
	switch m.Type {
	case document.MarkBold:
		return map[string]any{"type": "strong"}
	case document.MarkItalic:
		return map[string]any{"type": "em"}
	case document.MarkCode:
		return map[string]any{"type": "code"}
	case document.MarkStrike:
		return map[string]any{"type": "strike"}
	case document.MarkUnderline:
		return map[string]any{"type": "underline"}
	case document.MarkLink:
		out := map[string]any{"type": "link"}
		if m.Attrs != nil {
			out["attrs"] = map[string]any{"href": m.Attrs["href"]}
		}
		return out
	case document.MarkTextColor:
		out := map[string]any{"type": "textColor"}
		if m.Attrs != nil {
			out["attrs"] = map[string]any{"color": m.Attrs["color"]}
		}
		return out
	case document.MarkSuperscript:
		return map[string]any{"type": "subsup", "attrs": map[string]any{"type": "sup"}}
	case document.MarkSubscript:
		return map[string]any{"type": "subsup", "attrs": map[string]any{"type": "sub"}}
	default:
		return nil
	}
}
