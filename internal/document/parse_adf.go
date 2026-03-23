package document

import (
	"encoding/json"
	"fmt"
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

// ParseADF converts a Jira ADF JSON blob into the internal AST.
// Accepts either raw bytes or a pre-decoded map.
func ParseADF(data []byte) (*Node, error) {
	var raw adfNode
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("adf: invalid json: %w", err)
	}
	return convertADFNode(&raw)
}

// ParseADFValue converts an already-decoded ADF value (e.g. from a larger
// JSON response that was unmarshaled into map[string]any) into the AST.
func ParseADFValue(v any) (*Node, error) {
	// Re-marshal then parse. Not the fastest path, but correct and simple.
	// The ADF blobs we deal with are small (single issue descriptions).
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("adf: cannot re-marshal value: %w", err)
	}
	return ParseADF(b)
}

func convertADFNode(raw *adfNode) (*Node, error) {
	children, err := convertADFChildren(raw.Content)
	if err != nil {
		return nil, err
	}

	switch raw.Type {
	case "doc":
		return &Node{Type: NodeDoc, Children: children}, nil

	case "paragraph":
		return &Node{Type: NodeParagraph, Children: children}, nil

	case "heading":
		level := attrInt(raw.Attrs, "level", 2)
		return &Node{Type: NodeHeading, Level: level, Children: children}, nil

	case "text":
		marks := convertADFMarks(raw.Marks)
		return &Node{Type: NodeText, Text: raw.Text, Marks: marks}, nil

	case "hardBreak":
		return NewHardBreak(), nil

	case "bulletList":
		return &Node{Type: NodeBulletList, Children: children}, nil

	case "orderedList":
		return &Node{Type: NodeOrderedList, Children: children}, nil

	case "listItem":
		return &Node{Type: NodeListItem, Children: children}, nil

	case "codeBlock":
		lang := attrString(raw.Attrs, "language")
		return &Node{Type: NodeCodeBlock, Language: lang, Children: children}, nil

	case "blockquote":
		return &Node{Type: NodeBlockquote, Children: children}, nil

	case "rule":
		return NewRule(), nil

	case "table":
		return &Node{Type: NodeTable, Children: children}, nil

	case "tableRow":
		return &Node{Type: NodeTableRow, Children: children}, nil

	case "tableHeader":
		return &Node{
			Type:     NodeTableHeader,
			ColSpan:  max(1, attrInt(raw.Attrs, "colspan", 1)),
			RowSpan:  max(1, attrInt(raw.Attrs, "rowspan", 1)),
			Children: children,
		}, nil

	case "tableCell":
		return &Node{
			Type:     NodeTableCell,
			ColSpan:  max(1, attrInt(raw.Attrs, "colspan", 1)),
			RowSpan:  max(1, attrInt(raw.Attrs, "rowspan", 1)),
			Children: children,
		}, nil

	case "mediaSingle", "media", "mediaInline":
		return &Node{
			Type:      NodeMedia,
			MediaType: attrString(raw.Attrs, "type"),
			URL:       attrString(raw.Attrs, "url"),
			Alt:       attrString(raw.Attrs, "alt"),
			Children:  children,
		}, nil

	default:
		// Unknown node types: preserve children so content isn't silently lost.
		// This handles Jira extensions (panels, expand, status, etc.) by
		// passing through their text content even if we can't style them.
		if len(children) > 0 {
			return &Node{Type: NodeParagraph, Children: children}, nil
		}
		return nil, nil
	}
}

func convertADFChildren(raw []json.RawMessage) ([]*Node, error) {
	var children []*Node
	for _, r := range raw {
		var child adfNode
		if err := json.Unmarshal(r, &child); err != nil {
			continue // Skip malformed children rather than failing the whole doc.
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

func convertADFMarks(raw []adfMark) []Mark {
	if len(raw) == 0 {
		return nil
	}
	marks := make([]Mark, 0, len(raw))
	for _, m := range raw {
		mark, ok := convertADFMark(m)
		if ok {
			marks = append(marks, mark)
		}
	}
	return marks
}

func convertADFMark(m adfMark) (Mark, bool) {
	switch m.Type {
	case "strong":
		return Bold(), true
	case "em":
		return Italic(), true
	case "code":
		return Code(), true
	case "strike":
		return Strike(), true
	case "underline":
		return Underline(), true
	case "link":
		href := ""
		if m.Attrs != nil {
			href = m.Attrs["href"]
		}
		return Link(href), true
	case "textColor":
		color := ""
		if m.Attrs != nil {
			color = m.Attrs["color"]
		}
		return TextColor(color), true
	case "subsup":
		if m.Attrs != nil && m.Attrs["type"] == "sub" {
			return Mark{Type: MarkSubscript}, true
		}
		return Mark{Type: MarkSuperscript}, true
	default:
		return Mark{}, false
	}
}

// --- attribute helpers ---

func attrString(attrs map[string]any, key string) string {
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

func attrInt(attrs map[string]any, key string, fallback int) int {
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
