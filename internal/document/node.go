// Package document defines an intermediate AST for rich text interchange
// between Jira ADF, Markdown, and ANSI terminal output.
//
// The tree is deliberately simple: a Doc contains block-level nodes,
// block-level nodes contain inline nodes (or other blocks for lists),
// and inline Text nodes carry marks for styling. This mirrors ADF's
// own structure closely enough that ADF↔AST conversion is near-lossless,
// while remaining format-agnostic so renderers don't need to know about
// each other.
package document

// NodeType identifies the kind of AST node.
type NodeType int

const (
	NodeDoc NodeType = iota
	NodeParagraph
	NodeHeading
	NodeText
	NodeHardBreak
	NodeBulletList
	NodeOrderedList
	NodeListItem
	NodeCodeBlock
	NodeBlockquote
	NodeTable
	NodeTableRow
	NodeTableHeader
	NodeTableCell
	NodeRule // Horizontal rule / thematic break
	NodeMedia
)

// MarkType identifies inline styling applied to text spans.
type MarkType int

const (
	MarkBold MarkType = iota
	MarkItalic
	MarkCode
	MarkStrike
	MarkLink
	MarkUnderline
	MarkSuperscript
	MarkSubscript
	MarkTextColor
)

// Mark represents a single inline decoration on a Text node.
// Attrs carries mark-specific metadata (e.g. href for links,
// color for text color). Marks compose — a text span can carry
// multiple simultaneous marks, matching ADF's model exactly.
type Mark struct {
	Type  MarkType
	Attrs map[string]string
}

// Node is the single recursive type that forms the entire document tree.
// Not every field is meaningful for every NodeType — this is intentional.
// A flat struct avoids the interface-dispatch overhead and type-assertion
// ceremony that would come with per-type structs, while keeping the tree
// walkable with a simple switch on Type.
type Node struct {
	Type     NodeType
	Children []*Node

	// --- Text-specific fields ---
	// Only meaningful when Type == NodeText.
	Text  string
	Marks []Mark

	// --- Heading-specific fields ---
	// Level 1-6, matching both Markdown and ADF.
	Level int

	// --- CodeBlock-specific fields ---
	Language string

	// --- Table cell fields ---
	// ColSpan/RowSpan default to 1 if unset.
	ColSpan int
	RowSpan int

	// --- Media fields ---
	// MediaType could be "image", "file", etc.
	// URL is the src/href. Alt is the alt text.
	MediaType string
	URL       string
	Alt       string
}

func NewDoc(children ...*Node) *Node         { return &Node{Type: NodeDoc, Children: children} }
func NewParagraph(children ...*Node) *Node   { return &Node{Type: NodeParagraph, Children: children} }
func NewBulletList(children ...*Node) *Node  { return &Node{Type: NodeBulletList, Children: children} }
func NewOrderedList(children ...*Node) *Node { return &Node{Type: NodeOrderedList, Children: children} }
func NewListItem(children ...*Node) *Node    { return &Node{Type: NodeListItem, Children: children} }
func NewBlockquote(children ...*Node) *Node  { return &Node{Type: NodeBlockquote, Children: children} }
func NewTable(children ...*Node) *Node       { return &Node{Type: NodeTable, Children: children} }
func NewTableRow(children ...*Node) *Node    { return &Node{Type: NodeTableRow, Children: children} }
func NewHardBreak() *Node                    { return &Node{Type: NodeHardBreak} }
func NewRule() *Node                         { return &Node{Type: NodeRule} }

func NewHeading(level int, children ...*Node) *Node {
	return &Node{Type: NodeHeading, Level: level, Children: children}
}

func NewText(text string) *Node {
	return &Node{Type: NodeText, Text: text}
}

func NewStyledText(text string, marks ...Mark) *Node {
	return &Node{Type: NodeText, Text: text, Marks: marks}
}

// NewCodeBlock creates a fenced code block. It wraps the code text
// as a child text node, matching the structure parsers produce.
func NewCodeBlock(language, code string) *Node {
	return &Node{
		Type:     NodeCodeBlock,
		Language: language,
		Children: []*Node{NewText(code)},
	}
}

func NewTableHeader(children ...*Node) *Node {
	return &Node{Type: NodeTableHeader, ColSpan: 1, RowSpan: 1, Children: children}
}

func NewTableCell(children ...*Node) *Node {
	return &Node{Type: NodeTableCell, ColSpan: 1, RowSpan: 1, Children: children}
}

func NewMedia(mediaType, url, alt string) *Node {
	return &Node{Type: NodeMedia, MediaType: mediaType, URL: url, Alt: alt}
}

func Bold() Mark      { return Mark{Type: MarkBold} }
func Italic() Mark    { return Mark{Type: MarkItalic} }
func Code() Mark      { return Mark{Type: MarkCode} }
func Strike() Mark    { return Mark{Type: MarkStrike} }
func Underline() Mark { return Mark{Type: MarkUnderline} }

func Link(href string) Mark {
	return Mark{Type: MarkLink, Attrs: map[string]string{"href": href}}
}

func TextColor(color string) Mark {
	return Mark{Type: MarkTextColor, Attrs: map[string]string{"color": color}}
}
