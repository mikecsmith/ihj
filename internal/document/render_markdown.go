package document

import (
	"fmt"
	"strings"
)

// RenderMarkdown converts the AST into a Markdown string suitable for
// writing to an editor buffer. This is the path used when opening an
// issue for editing: ADF → AST → Markdown → editor → Markdown → AST → ADF.
func RenderMarkdown(node *Node) string {
	if node == nil {
		return ""
	}
	var buf strings.Builder
	renderMarkdownNode(&buf, node, 0)
	return strings.TrimRight(buf.String(), "\n") + "\n"
}

func renderMarkdownNode(buf *strings.Builder, node *Node, depth int) {
	switch node.Type {
	case NodeDoc:
		for _, child := range node.Children {
			renderMarkdownNode(buf, child, depth)
		}

	case NodeParagraph:
		renderMarkdownInline(buf, node.Children)
		buf.WriteString("\n\n")

	case NodeHeading:
		buf.WriteString(strings.Repeat("#", node.Level))
		buf.WriteString(" ")
		renderMarkdownInline(buf, node.Children)
		buf.WriteString("\n\n")

	case NodeText:
		renderMarkdownText(buf, node)

	case NodeHardBreak:
		buf.WriteString("  \n")

	case NodeBulletList:
		for _, item := range node.Children {
			renderMarkdownListItem(buf, item, "- ", depth)
		}
		if depth == 0 {
			buf.WriteString("\n")
		}

	case NodeOrderedList:
		for i, item := range node.Children {
			prefix := fmt.Sprintf("%d. ", i+1)
			renderMarkdownListItem(buf, item, prefix, depth)
		}
		if depth == 0 {
			buf.WriteString("\n")
		}

	case NodeListItem:
		// Handled by renderMarkdownListItem. Direct calls fall through.
		for _, child := range node.Children {
			renderMarkdownNode(buf, child, depth)
		}

	case NodeCodeBlock:
		buf.WriteString("```")
		buf.WriteString(node.Language)
		buf.WriteString("\n")
		for _, child := range node.Children {
			if child.Type == NodeText {
				buf.WriteString(child.Text)
			}
		}
		// Ensure code block ends with a newline before the fence.
		s := buf.String()
		if len(s) > 0 && s[len(s)-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")

	case NodeBlockquote:
		// Render children to a temporary buffer, then prefix each line with "> ".
		var inner strings.Builder
		for _, child := range node.Children {
			renderMarkdownNode(&inner, child, depth)
		}
		for line := range strings.SplitSeq(strings.TrimRight(inner.String(), "\n"), "\n") {
			buf.WriteString("> ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")

	case NodeTable:
		renderMarkdownTable(buf, node)

	case NodeRule:
		buf.WriteString("---\n\n")

	case NodeMedia:
		alt := node.Alt
		if alt == "" {
			alt = "media"
		}
		if node.URL != "" {
			fmt.Fprintf(buf, "![%s](%s)\n\n", alt, node.URL)
		} else {
			fmt.Fprintf(buf, "<!-- media: %s (no URL) -->\n\n", alt)
		}
	}
}

func renderMarkdownListItem(buf *strings.Builder, item *Node, prefix string, depth int) {
	indent := strings.Repeat("  ", depth)
	buf.WriteString(indent)
	buf.WriteString(prefix)

	for i, child := range item.Children {
		switch child.Type {
		case NodeParagraph:
			if i == 0 {
				// First paragraph: inline on the same line as the bullet.
				renderMarkdownInline(buf, child.Children)
				buf.WriteString("\n")
			} else {
				// Subsequent paragraphs: indented continuation.
				contIndent := indent + strings.Repeat(" ", len(prefix))
				buf.WriteString(contIndent)
				renderMarkdownInline(buf, child.Children)
				buf.WriteString("\n")
			}
		case NodeBulletList, NodeOrderedList:
			// Nested list: recurse with increased depth.
			renderMarkdownNode(buf, child, depth+1)
		default:
			renderMarkdownNode(buf, child, depth)
		}
	}
}

func renderMarkdownInline(buf *strings.Builder, children []*Node) {
	for _, child := range children {
		switch child.Type {
		case NodeText:
			renderMarkdownText(buf, child)
		case NodeHardBreak:
			buf.WriteString("  \n")
		}
	}
}

func renderMarkdownText(buf *strings.Builder, node *Node) {
	text := node.Text

	// Determine wrapping from marks. Order matters for nesting:
	// link is outermost, then bold/italic/strike, code is innermost.
	var linkHref string
	hasBold := false
	hasItalic := false
	hasCode := false
	hasStrike := false

	for _, m := range node.Marks {
		switch m.Type {
		case MarkBold:
			hasBold = true
		case MarkItalic:
			hasItalic = true
		case MarkCode:
			hasCode = true
		case MarkStrike:
			hasStrike = true
		case MarkLink:
			if m.Attrs != nil {
				linkHref = m.Attrs["href"]
			}
		}
	}

	// Apply innermost marks first, outermost last.
	if hasCode {
		text = "`" + text + "`"
	}
	if hasStrike {
		text = "~~" + text + "~~"
	}
	if hasItalic {
		text = "*" + text + "*"
	}
	if hasBold {
		text = "**" + text + "**"
	}
	if linkHref != "" {
		text = "[" + text + "](" + linkHref + ")"
	}

	buf.WriteString(text)
}

func renderMarkdownTable(buf *strings.Builder, table *Node) {
	if len(table.Children) == 0 {
		return
	}

	// First pass: determine column count from the first row.
	firstRow := table.Children[0]
	colCount := 0
	for _, cell := range firstRow.Children {
		span := max(cell.ColSpan, 1)
		colCount += span
	}

	// Render each row.
	for rowIdx, row := range table.Children {
		buf.WriteString("|")
		for _, cell := range row.Children {
			buf.WriteString(" ")
			renderMarkdownInline(buf, flattenCellContent(cell))
			buf.WriteString(" |")
		}
		buf.WriteString("\n")

		// After the first row (header), emit the separator line.
		if rowIdx == 0 {
			buf.WriteString("|")
			for i := 0; i < colCount; i++ {
				buf.WriteString(" --- |")
			}
			buf.WriteString("\n")
		}
	}
	buf.WriteString("\n")
}

// flattenCellContent extracts inline nodes from a table cell, which may
// contain paragraphs wrapping the actual text content.
func flattenCellContent(cell *Node) []*Node {
	var inlines []*Node
	for _, child := range cell.Children {
		switch child.Type {
		case NodeParagraph:
			inlines = append(inlines, child.Children...)
		case NodeText, NodeHardBreak:
			inlines = append(inlines, child)
		}
	}
	return inlines
}
