package jira

import (
	"encoding/json"
	"fmt"

	"github.com/mikecsmith/ihj/internal/document"
)

// adfNode is the raw JSON shape that Jira's Atlassian Document Format uses.
// We parse into this throwaway struct, then convert to our own AST.
type adfNode struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Marks   []adfMark         `json:"marks,omitempty"`
	Attrs   map[string]any    `json:"attrs,omitempty"`
	Content []json.RawMessage `json:"content,omitempty"`
}

type adfMark struct {
	Type  string            `json:"type"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// parseADF converts a Jira ADF JSON blob into the internal AST.
// Accepts raw bytes.
func parseADF(data []byte) (*document.Node, error) {
	var raw adfNode
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("adf: invalid json: %w", err)
	}
	return convertADFNode(&raw)
}

func convertADFNode(raw *adfNode) (*document.Node, error) {
	children, err := convertADFChildren(raw.Content)
	if err != nil {
		return nil, err
	}

	switch raw.Type {
	case "doc":
		return document.NewDoc(children...), nil

	case "paragraph":
		return document.NewParagraph(children...), nil

	case "heading":
		level := adfAttrInt(raw.Attrs, "level", 2)
		return document.NewHeading(level, children...), nil

	case "text":
		marks := convertADFMarks(raw.Marks)
		return document.NewStyledText(raw.Text, marks...), nil

	case "hardBreak":
		return document.NewHardBreak(), nil

	case "bulletList":
		return document.NewBulletList(children...), nil

	case "orderedList":
		return document.NewOrderedList(children...), nil

	case "listItem":
		return document.NewListItem(children...), nil

	case "codeBlock":
		lang := adfAttrString(raw.Attrs, "language")
		node := document.NewCodeBlock(lang, "")
		node.Children = children // ADF provides children directly, not wrapped text.
		return node, nil

	case "blockquote":
		return document.NewBlockquote(children...), nil

	case "rule":
		return document.NewRule(), nil

	case "table":
		return document.NewTable(children...), nil

	case "tableRow":
		return document.NewTableRow(children...), nil

	case "tableHeader":
		node := document.NewTableHeader(children...)
		node.ColSpan = max(1, adfAttrInt(raw.Attrs, "colspan", 1))
		node.RowSpan = max(1, adfAttrInt(raw.Attrs, "rowspan", 1))
		return node, nil

	case "tableCell":
		node := document.NewTableCell(children...)
		node.ColSpan = max(1, adfAttrInt(raw.Attrs, "colspan", 1))
		node.RowSpan = max(1, adfAttrInt(raw.Attrs, "rowspan", 1))
		return node, nil

	case "mediaSingle", "media", "mediaInline":
		node := document.NewMedia(
			adfAttrString(raw.Attrs, "type"),
			adfAttrString(raw.Attrs, "url"),
			adfAttrString(raw.Attrs, "alt"),
		)
		node.Children = children
		return node, nil

	default:
		// Unknown node types: preserve children so content isn't silently lost.
		if len(children) > 0 {
			return document.NewParagraph(children...), nil
		}
		return nil, nil
	}
}

func convertADFChildren(raw []json.RawMessage) ([]*document.Node, error) {
	var children []*document.Node
	for _, r := range raw {
		var child adfNode
		if err := json.Unmarshal(r, &child); err != nil {
			continue
		}
		node, err := convertADFNode(&child)
		if err != nil {
			continue
		}
		if node != nil {
			children = append(children, node)
		}
	}
	return children, nil
}

func convertADFMarks(raw []adfMark) []document.Mark {
	if len(raw) == 0 {
		return nil
	}
	marks := make([]document.Mark, 0, len(raw))
	for _, m := range raw {
		mark, ok := convertADFMark(m)
		if ok {
			marks = append(marks, mark)
		}
	}
	return marks
}

func convertADFMark(m adfMark) (document.Mark, bool) {
	switch m.Type {
	case "strong":
		return document.Bold(), true
	case "em":
		return document.Italic(), true
	case "code":
		return document.Code(), true
	case "strike":
		return document.Strike(), true
	case "underline":
		return document.Underline(), true
	case "link":
		href := ""
		if m.Attrs != nil {
			href = m.Attrs["href"]
		}
		return document.Link(href), true
	case "textColor":
		color := ""
		if m.Attrs != nil {
			color = m.Attrs["color"]
		}
		return document.TextColor(color), true
	case "subsup":
		if m.Attrs != nil && m.Attrs["type"] == "sub" {
			return document.Mark{Type: document.MarkSubscript}, true
		}
		return document.Mark{Type: document.MarkSuperscript}, true
	default:
		return document.Mark{}, false
	}
}

// --- attribute helpers (prefixed with adf to avoid conflicts) ---

func adfAttrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func adfAttrInt(attrs map[string]any, key string, fallback int) int {
	if attrs == nil {
		return fallback
	}
	v, ok := attrs[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return fallback
	}
}
