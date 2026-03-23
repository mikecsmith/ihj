package document

// Walk traverses the tree depth-first, calling fn on every node.
// If fn returns false, the subtree rooted at that node is skipped.
func Walk(node *Node, fn func(*Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for _, child := range node.Children {
		Walk(child, fn)
	}
}

// Transform applies fn to every node in the tree depth-first,
// replacing each node with fn's return value. If fn returns nil,
// the node is removed from its parent. Children are transformed
// before their parent, so fn receives already-transformed subtrees.
func Transform(node *Node, fn func(*Node) *Node) *Node {
	if node == nil {
		return nil
	}

	// Transform children first (post-order).
	var kept []*Node
	for _, child := range node.Children {
		result := Transform(child, fn)
		if result != nil {
			kept = append(kept, result)
		}
	}
	node.Children = kept

	return fn(node)
}

// PlainText extracts all text content from the tree, discarding
// all formatting. Useful for building search indexes or generating
// branch names.
func PlainText(node *Node) string {
	if node == nil {
		return ""
	}

	if node.Type == NodeText {
		return node.Text
	}

	var buf []byte
	needsSpace := false

	for _, child := range node.Children {
		text := PlainText(child)
		if text == "" {
			continue
		}

		// Insert whitespace between block-level siblings.
		if needsSpace && isBlock(child.Type) {
			buf = append(buf, ' ')
		}
		buf = append(buf, text...)
		needsSpace = true
	}

	return string(buf)
}

// Truncate returns a new tree containing at most maxBlocks top-level
// block children of a Doc node. Non-Doc nodes pass through unchanged.
// This is useful for compact previews ("first 2 paragraphs").
func Truncate(node *Node, maxBlocks int) *Node {
	if node == nil || node.Type != NodeDoc || len(node.Children) <= maxBlocks {
		return node
	}

	truncated := &Node{
		Type:     NodeDoc,
		Children: make([]*Node, maxBlocks),
	}
	copy(truncated.Children, node.Children[:maxBlocks])
	return truncated
}

// HasMark reports whether a text node carries a specific mark type.
func HasMark(node *Node, mt MarkType) bool {
	for _, m := range node.Marks {
		if m.Type == mt {
			return true
		}
	}
	return false
}

// GetMarkAttr returns the attribute value for a given mark type and key,
// or empty string if not found. Commonly used to extract href from links.
func GetMarkAttr(node *Node, mt MarkType, key string) string {
	for _, m := range node.Marks {
		if m.Type == mt {
			if m.Attrs != nil {
				return m.Attrs[key]
			}
			return ""
		}
	}
	return ""
}

func isBlock(nt NodeType) bool {
	switch nt {
	case NodeParagraph, NodeHeading, NodeBulletList, NodeOrderedList,
		NodeListItem, NodeCodeBlock, NodeBlockquote, NodeTable, NodeRule:
		return true
	}
	return false
}
