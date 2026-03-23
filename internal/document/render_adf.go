package document

import "encoding/json"

// RenderADF converts the internal AST back into Jira ADF JSON bytes.
// This produces a valid ADF document that can be POSTed to the Jira API.
func RenderADF(node *Node) ([]byte, error) {
	out := renderADFNode(node)
	if out == nil {
		out = map[string]any{
			"version": 1,
			"type":    "doc",
			"content": []any{},
		}
	}
	return json.Marshal(out)
}

// RenderADFValue returns the ADF as a map[string]any suitable for embedding
// directly into a larger JSON payload (e.g. issue creation body).
func RenderADFValue(node *Node) map[string]any {
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

func renderADFNode(node *Node) map[string]any {
	if node == nil {
		return nil
	}

	switch node.Type {
	case NodeDoc:
		return map[string]any{
			"version": 1,
			"type":    "doc",
			"content": renderADFChildren(node.Children),
		}

	case NodeParagraph:
		return map[string]any{
			"type":    "paragraph",
			"content": renderADFChildren(node.Children),
		}

	case NodeHeading:
		return map[string]any{
			"type":    "heading",
			"attrs":   map[string]any{"level": node.Level},
			"content": renderADFChildren(node.Children),
		}

	case NodeText:
		out := map[string]any{
			"type": "text",
			"text": node.Text,
		}
		if marks := renderADFMarks(node.Marks); len(marks) > 0 {
			out["marks"] = marks
		}
		return out

	case NodeHardBreak:
		return map[string]any{"type": "hardBreak"}

	case NodeBulletList:
		return map[string]any{
			"type":    "bulletList",
			"content": renderADFChildren(node.Children),
		}

	case NodeOrderedList:
		return map[string]any{
			"type":    "orderedList",
			"content": renderADFChildren(node.Children),
		}

	case NodeListItem:
		return map[string]any{
			"type":    "listItem",
			"content": renderADFChildren(node.Children),
		}

	case NodeCodeBlock:
		out := map[string]any{
			"type":    "codeBlock",
			"content": renderADFChildren(node.Children),
		}
		if node.Language != "" {
			out["attrs"] = map[string]any{"language": node.Language}
		}
		return out

	case NodeBlockquote:
		return map[string]any{
			"type":    "blockquote",
			"content": renderADFChildren(node.Children),
		}

	case NodeRule:
		return map[string]any{"type": "rule"}

	case NodeTable:
		return map[string]any{
			"type":    "table",
			"attrs":   map[string]any{"isNumberColumnEnabled": false, "layout": "default"},
			"content": renderADFChildren(node.Children),
		}

	case NodeTableRow:
		return map[string]any{
			"type":    "tableRow",
			"content": renderADFChildren(node.Children),
		}

	case NodeTableHeader:
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

	case NodeTableCell:
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

	case NodeMedia:
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

func renderADFChildren(children []*Node) []any {
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

func renderADFMarks(marks []Mark) []any {
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

func renderADFMark(m Mark) map[string]any {
	switch m.Type {
	case MarkBold:
		return map[string]any{"type": "strong"}
	case MarkItalic:
		return map[string]any{"type": "em"}
	case MarkCode:
		return map[string]any{"type": "code"}
	case MarkStrike:
		return map[string]any{"type": "strike"}
	case MarkUnderline:
		return map[string]any{"type": "underline"}
	case MarkLink:
		out := map[string]any{"type": "link"}
		if m.Attrs != nil {
			out["attrs"] = map[string]any{"href": m.Attrs["href"]}
		}
		return out
	case MarkTextColor:
		out := map[string]any{"type": "textColor"}
		if m.Attrs != nil {
			out["attrs"] = map[string]any{"color": m.Attrs["color"]}
		}
		return out
	case MarkSuperscript:
		return map[string]any{"type": "subsup", "attrs": map[string]any{"type": "sup"}}
	case MarkSubscript:
		return map[string]any{"type": "subsup", "attrs": map[string]any{"type": "sub"}}
	default:
		return nil
	}
}
