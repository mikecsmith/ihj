package document

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ANSIConfig controls rendering behavior.
type ANSIConfig struct {
	// WrapWidth is the maximum line width for paragraph text wrapping.
	// If zero, no wrapping is applied.
	WrapWidth int

	// Styles provides all text styling. If nil, uses a plain-text
	// fallback that applies no formatting.
	Styles StyleSet
}

// RenderANSI converts the AST into a styled string for terminal display.
// The styling is fully delegated to the StyleSet in the config, so
// "ANSI" is a slight misnomer — it renders to whatever the StyleSet produces.
func RenderANSI(node *Node, cfg ANSIConfig) string {
	if node == nil {
		return ""
	}
	s := cfg.Styles
	if s == nil {
		s = PlainStyles{}
	}
	r := &ansiRenderer{cfg: cfg, styles: s}
	var buf strings.Builder
	r.renderNode(&buf, node, 0)
	return buf.String()
}

type ansiRenderer struct {
	cfg    ANSIConfig
	styles StyleSet
}

func (r *ansiRenderer) renderNode(buf *strings.Builder, node *Node, listDepth int) {
	s := r.styles

	switch node.Type {
	case NodeDoc:
		for _, child := range node.Children {
			r.renderNode(buf, child, listDepth)
		}

	case NodeParagraph:
		raw := r.renderInline(node.Children)
		if strings.TrimSpace(raw) == "" {
			return
		}
		if r.cfg.WrapWidth > 0 {
			raw = wrapText(raw, r.cfg.WrapWidth)
		}
		buf.WriteString(raw)
		buf.WriteString("\n\n")

	case NodeHeading:
		inner := r.renderInline(node.Children)
		text := strings.TrimSpace(inner)
		buf.WriteString("\n")
		buf.WriteString(s.Heading(text, node.Level))
		buf.WriteString("\n\n")

	case NodeText:
		buf.WriteString(r.renderTextNode(node))

	case NodeHardBreak:
		buf.WriteString("\n")

	case NodeBulletList:
		for _, item := range node.Children {
			r.renderListItem(buf, item, "•", listDepth)
		}
		if listDepth == 0 {
			buf.WriteString("\n")
		}

	case NodeOrderedList:
		for i, item := range node.Children {
			prefix := fmt.Sprintf("%d.", i+1)
			r.renderListItem(buf, item, prefix, listDepth)
		}
		if listDepth == 0 {
			buf.WriteString("\n")
		}

	case NodeListItem:
		for _, child := range node.Children {
			r.renderNode(buf, child, listDepth)
		}

	case NodeCodeBlock:
		lang := strings.ToUpper(node.Language)
		if lang == "" {
			lang = "CODE"
		}
		border := s.CodeBlockBorder()

		buf.WriteString("\n")
		buf.WriteString(s.CodeBlockLabel(lang))
		buf.WriteString("\n")

		code := collectText(node.Children)
		for _, line := range strings.Split(code, "\n") {
			line = strings.ReplaceAll(line, "\t", "    ")
			buf.WriteString(s.Dim(border))
			buf.WriteString(" ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")

	case NodeBlockquote:
		var inner strings.Builder
		for _, child := range node.Children {
			r.renderNode(&inner, child, listDepth)
		}
		border := s.BlockquoteBorder()
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			buf.WriteString(border)
			buf.WriteString(" ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")

	case NodeTable:
		r.renderTable(buf, node)

	case NodeRule:
		width := 40
		if r.cfg.WrapWidth > 0 {
			width = r.cfg.WrapWidth
		}
		buf.WriteString(s.HorizontalRule(width))
		buf.WriteString("\n\n")

	case NodeMedia:
		alt := node.Alt
		if alt == "" {
			alt = "attachment"
		}
		buf.WriteString(s.MediaPlaceholder(alt, node.URL))
		buf.WriteString("\n\n")
	}
}

func (r *ansiRenderer) renderListItem(buf *strings.Builder, item *Node, bullet string, depth int) {
	indent := strings.Repeat("  ", depth)
	buf.WriteString(indent)
	buf.WriteString(bullet)
	buf.WriteString(" ")

	for i, child := range item.Children {
		switch child.Type {
		case NodeParagraph:
			inline := r.renderInline(child.Children)
			if r.cfg.WrapWidth > 0 {
				prefixLen := utf8.RuneCountInString(indent) + utf8.RuneCountInString(bullet) + 1
				inline = wrapText(inline, r.cfg.WrapWidth-prefixLen)
			}
			if i == 0 {
				buf.WriteString(strings.TrimSpace(inline))
				buf.WriteString("\n")
			} else {
				contIndent := indent + strings.Repeat(" ", utf8.RuneCountInString(bullet)+1)
				buf.WriteString(contIndent)
				buf.WriteString(strings.TrimSpace(inline))
				buf.WriteString("\n")
			}
		case NodeBulletList, NodeOrderedList:
			r.renderNode(buf, child, depth+1)
		default:
			r.renderNode(buf, child, depth)
		}
	}
}

func (r *ansiRenderer) renderInline(children []*Node) string {
	var buf strings.Builder
	for _, child := range children {
		switch child.Type {
		case NodeText:
			buf.WriteString(r.renderTextNode(child))
		case NodeHardBreak:
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func (r *ansiRenderer) renderTextNode(node *Node) string {
	text := node.Text
	if len(node.Marks) == 0 {
		return text
	}

	s := r.styles

	// Apply marks inside-out: code → strike → italic → bold → link.
	for _, m := range node.Marks {
		switch m.Type {
		case MarkCode:
			text = s.Code(text)
		case MarkStrike:
			text = s.Strike(text)
		}
	}
	for _, m := range node.Marks {
		switch m.Type {
		case MarkItalic:
			text = s.Italic(text)
		case MarkUnderline:
			text = s.Underline(text)
		}
	}
	for _, m := range node.Marks {
		if m.Type == MarkBold {
			text = s.Bold(text)
		}
	}
	for _, m := range node.Marks {
		if m.Type == MarkLink {
			href := ""
			if m.Attrs != nil {
				href = m.Attrs["href"]
			}
			text = s.Link(text, href)
		}
	}

	return text
}

func (r *ansiRenderer) renderTable(buf *strings.Builder, table *Node) {
	if len(table.Children) == 0 {
		return
	}

	s := r.styles

	// Collect cell text as a 2D grid.
	var rows [][]string
	maxCols := 0

	for _, row := range table.Children {
		var cells []string
		for _, cell := range row.Children {
			text := r.renderInline(flattenCellContent(cell))
			cells = append(cells, strings.TrimSpace(text))
		}
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
		rows = append(rows, cells)
	}

	// Calculate column widths.
	colWidths := make([]int, maxCols)
	for _, row := range rows {
		for i, cell := range row {
			vl := visibleLength(cell)
			if vl > colWidths[i] {
				colWidths[i] = vl
			}
		}
	}

	// Render.
	for rowIdx, row := range rows {
		for i := 0; i < maxCols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			pad := colWidths[i] - visibleLength(cell)
			buf.WriteString(cell)
			buf.WriteString(strings.Repeat(" ", pad))
			if i < maxCols-1 {
				buf.WriteString(s.Dim(" │ "))
			}
		}
		buf.WriteString("\n")

		// Separator after header row.
		if rowIdx == 0 {
			for i := 0; i < maxCols; i++ {
				buf.WriteString(s.Dim(strings.Repeat("─", colWidths[i])))
				if i < maxCols-1 {
					buf.WriteString(s.Dim("─┼─"))
				}
			}
			buf.WriteString("\n")
		}
	}
	buf.WriteString("\n")
}

// --- Text wrapping (style-agnostic) ---

// wrapText wraps text at the given width while being aware that ANSI/styled
// escape sequences don't contribute to visible width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			result.WriteString("\n")
			continue
		}
		result.WriteString(wrapSingleLine(line, width))
		result.WriteString("\n")
	}
	s := result.String()
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return s
}

func wrapSingleLine(line string, width int) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var buf strings.Builder
	visLen := 0
	for i, word := range words {
		wLen := visibleLength(word)
		if i > 0 && visLen+1+wLen > width {
			buf.WriteString("\n")
			buf.WriteString(word)
			visLen = wLen
		} else {
			if i > 0 {
				buf.WriteString(" ")
				visLen++
			}
			buf.WriteString(word)
			visLen += wLen
		}
	}
	return buf.String()
}

// visibleLength returns the display width of a string, ignoring ANSI escapes.
// Handles both CSI sequences (\033[...m) and OSC sequences (\033]...\a).
func visibleLength(s string) int {
	length := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == '\033' && i+1 < len(runes) {
			next := runes[i+1]
			if next == ']' {
				// OSC sequence: skip until BEL (\a) or ST (\033\\).
				i += 2
				for i < len(runes) {
					if runes[i] == '\a' {
						i++
						break
					}
					if runes[i] == '\033' && i+1 < len(runes) && runes[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
				continue
			}
			if next == '[' {
				// CSI sequence: skip until a letter terminates it.
				i += 2
				for i < len(runes) {
					c := runes[i]
					i++
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						break
					}
				}
				continue
			}
		}
		length++
		i++
	}
	return length
}

// collectText concatenates raw text from children, used for code blocks.
func collectText(children []*Node) string {
	var buf strings.Builder
	for _, c := range children {
		if c.Type == NodeText {
			buf.WriteString(c.Text)
		}
	}
	return buf.String()
}
