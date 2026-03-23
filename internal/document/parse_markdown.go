package document

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ParseMarkdown converts a Markdown string into the internal AST.
// This uses goldmark for robust parsing (nested lists, complex inline
// marks, etc.) rather than hand-rolled regex.
func ParseMarkdown(source []byte) (*Node, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // Enables tables, strikethrough, autolinks, task lists.
		),
	)
	reader := text.NewReader(source)
	gmDoc := md.Parser().Parse(reader)

	doc := &Node{Type: NodeDoc}
	convertGoldmarkChildren(doc, gmDoc, source)
	return doc, nil
}

// ParseMarkdownString is a convenience wrapper.
func ParseMarkdownString(s string) (*Node, error) {
	return ParseMarkdown([]byte(s))
}

func convertGoldmarkChildren(parent *Node, gmNode ast.Node, source []byte) {
	for child := gmNode.FirstChild(); child != nil; child = child.NextSibling() {
		converted := convertGoldmarkNode(child, source)
		if converted != nil {
			parent.Children = append(parent.Children, converted)
		}
	}
}

func convertGoldmarkNode(gmNode ast.Node, source []byte) *Node {
	switch n := gmNode.(type) {
	case *ast.Document:
		doc := &Node{Type: NodeDoc}
		convertGoldmarkChildren(doc, n, source)
		return doc

	case *ast.Paragraph:
		p := &Node{Type: NodeParagraph}
		convertGoldmarkInline(p, n, source)
		return p

	case *ast.Heading:
		h := &Node{Type: NodeHeading, Level: n.Level}
		convertGoldmarkInline(h, n, source)
		return h

	case *ast.ThematicBreak:
		return NewRule()

	case *ast.CodeBlock:
		code := extractGoldmarkText(n, source)
		return NewCodeBlock("", code)

	case *ast.FencedCodeBlock:
		lang := ""
		if n.Info != nil {
			lang = string(n.Info.Segment.Value(source))
		}
		code := extractGoldmarkText(n, source)
		return NewCodeBlock(lang, code)

	case *ast.Blockquote:
		bq := &Node{Type: NodeBlockquote}
		convertGoldmarkChildren(bq, n, source)
		return bq

	case *ast.List:
		var list *Node
		if n.IsOrdered() {
			list = &Node{Type: NodeOrderedList}
		} else {
			list = &Node{Type: NodeBulletList}
		}
		convertGoldmarkChildren(list, n, source)
		return list

	case *ast.ListItem:
		item := &Node{Type: NodeListItem}
		convertGoldmarkChildren(item, n, source)
		return item

	case *ast.HTMLBlock:
		// Preserve raw HTML as a code block so it's not silently dropped.
		html := extractGoldmarkText(n, source)
		if html != "" {
			return NewCodeBlock("html", html)
		}
		return nil

	case *east.Table:
		return convertGoldmarkTable(n, source)

	default:
		// For any unrecognized block nodes, try to convert children.
		if gmNode.HasChildren() {
			p := &Node{Type: NodeParagraph}
			convertGoldmarkInline(p, gmNode, source)
			if len(p.Children) > 0 {
				return p
			}
		}
		return nil
	}
}

// convertGoldmarkInline walks inline children and converts them to AST nodes.
func convertGoldmarkInline(parent *Node, gmNode ast.Node, source []byte) {
	for child := gmNode.FirstChild(); child != nil; child = child.NextSibling() {
		nodes := convertGoldmarkInlineNode(child, source, nil)
		parent.Children = append(parent.Children, nodes...)
	}
}

// convertGoldmarkInlineNode converts a single inline goldmark node, propagating
// inherited marks for nested emphasis (e.g., bold inside italic inside a link).
func convertGoldmarkInlineNode(gmNode ast.Node, source []byte, inherited []Mark) []*Node {
	switch n := gmNode.(type) {
	case *ast.Text:
		t := string(n.Segment.Value(source))
		var result []*Node

		if len(inherited) > 0 {
			result = append(result, &Node{Type: NodeText, Text: t, Marks: copyMarks(inherited)})
		} else {
			result = append(result, NewText(t))
		}

		// Goldmark represents hard line breaks as a flag on the text node.
		if n.HardLineBreak() {
			result = append(result, NewHardBreak())
		} else if n.SoftLineBreak() {
			// Treat soft breaks as spaces to match typical rendering.
			result = append(result, NewText(" "))
		}

		return result

	case *ast.String:
		t := string(n.Value)
		if len(inherited) > 0 {
			return []*Node{{Type: NodeText, Text: t, Marks: copyMarks(inherited)}}
		}
		return []*Node{NewText(t)}

	case *ast.CodeSpan:
		t := extractInlineText(n, source)
		marks := append(copyMarks(inherited), Code())
		return []*Node{{Type: NodeText, Text: t, Marks: marks}}

	case *ast.Emphasis:
		var mark Mark
		if n.Level == 2 {
			mark = Bold()
		} else {
			mark = Italic()
		}
		newMarks := append(copyMarks(inherited), mark)

		var result []*Node
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			result = append(result, convertGoldmarkInlineNode(child, source, newMarks)...)
		}
		return result

	case *ast.Link:
		href := string(n.Destination)
		newMarks := append(copyMarks(inherited), Link(href))

		var result []*Node
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			result = append(result, convertGoldmarkInlineNode(child, source, newMarks)...)
		}
		return result

	case *ast.AutoLink:
		url := string(n.URL(source))
		marks := append(copyMarks(inherited), Link(url))
		return []*Node{{Type: NodeText, Text: url, Marks: marks}}

	case *ast.Image:
		alt := extractInlineText(n, source)
		return []*Node{NewMedia("image", string(n.Destination), alt)}

	case *east.Strikethrough:
		newMarks := append(copyMarks(inherited), Strike())
		var result []*Node
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			result = append(result, convertGoldmarkInlineNode(child, source, newMarks)...)
		}
		return result

	case *ast.RawHTML:
		// Inline HTML — preserve as plain text.
		var buf bytes.Buffer
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			buf.Write(seg.Value(source))
		}
		return []*Node{NewText(buf.String())}

	default:
		// Unknown inline node — recurse into children.
		var result []*Node
		for child := gmNode.FirstChild(); child != nil; child = child.NextSibling() {
			result = append(result, convertGoldmarkInlineNode(child, source, inherited)...)
		}
		if len(result) == 0 {
			// Last resort: try to extract any raw text segments.
			t := extractInlineText(gmNode, source)
			if t != "" {
				if len(inherited) > 0 {
					return []*Node{{Type: NodeText, Text: t, Marks: copyMarks(inherited)}}
				}
				return []*Node{NewText(t)}
			}
		}
		return result
	}
}

// extractGoldmarkText collects raw text lines from a block node (code blocks, HTML blocks).
func extractGoldmarkText(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	// Trim trailing newline that goldmark includes.
	result := buf.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result
}

// extractInlineText extracts plain text from inline node children.
func extractInlineText(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		} else if s, ok := child.(*ast.String); ok {
			buf.Write(s.Value)
		}
	}
	return buf.String()
}

func copyMarks(marks []Mark) []Mark {
	if len(marks) == 0 {
		return nil
	}
	cp := make([]Mark, len(marks))
	copy(cp, marks)
	return cp
}

// convertGoldmarkTable converts a GFM table AST node into our internal table nodes.
func convertGoldmarkTable(table *east.Table, source []byte) *Node {
	result := &Node{Type: NodeTable}

	// Walk rows: first is the header (TableHeader), rest are body rows.
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		row := &Node{Type: NodeTableRow}

		for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
			var cellNode *Node
			if _, ok := cell.(*east.TableCell); ok {
				cellNode = &Node{Type: NodeTableCell, ColSpan: 1, RowSpan: 1}
			} else {
				cellNode = &Node{Type: NodeTableHeader, ColSpan: 1, RowSpan: 1}
			}

			// Cell content is inline — wrap in a paragraph so flattenCellContent works.
			p := &Node{Type: NodeParagraph}
			convertGoldmarkInline(p, cell, source)
			cellNode.Children = append(cellNode.Children, p)

			row.Children = append(row.Children, cellNode)
		}

		result.Children = append(result.Children, row)
	}

	return result
}
