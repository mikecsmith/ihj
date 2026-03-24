package document

import (
	"testing"
)

func TestPlainText(t *testing.T) {
	doc := NewDoc(
		NewParagraph(
			NewText("Hello "),
			NewStyledText("bold", Bold()),
			NewText(" world"),
		),
		NewHeading(2, NewText("Title")),
	)
	text := PlainText(doc)
	if text != "Hello bold world Title" {
		t.Errorf("PlainText() = %q; want \"Hello bold world Title\"", text)
	}
}

func TestTruncate(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("one")),
		NewParagraph(NewText("two")),
		NewParagraph(NewText("three")),
	)
	truncated := Truncate(doc, 2)
	if len(truncated.Children) != 2 {
		t.Errorf("Truncate(doc, 2) children = %d; want 2", len(truncated.Children))
	}
	// Original should be unchanged.
	if len(doc.Children) != 3 {
		t.Error("truncate mutated original")
	}
}

func TestWalk(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("a"), NewText("b")),
		NewParagraph(NewText("c")),
	)
	var texts []string
	Walk(doc, func(n *Node) bool {
		if n.Type == NodeText {
			texts = append(texts, n.Text)
		}
		return true
	})
	if len(texts) != 3 || texts[0] != "a" || texts[1] != "b" || texts[2] != "c" {
		t.Errorf("Walk() collected texts = %v; want [a b c]", texts)
	}
}

func TestWalk_SkipSubtree(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("visible")),
		NewCodeBlock("go", "hidden"),
	)
	var texts []string
	Walk(doc, func(n *Node) bool {
		if n.Type == NodeCodeBlock {
			return false // Skip code block subtree.
		}
		if n.Type == NodeText {
			texts = append(texts, n.Text)
		}
		return true
	})
	if len(texts) != 1 || texts[0] != "visible" {
		t.Errorf("Walk() with skip collected texts = %v; want [visible]", texts)
	}
}

func TestTransform_RemoveNodes(t *testing.T) {
	doc := NewDoc(
		NewParagraph(NewText("keep")),
		NewCodeBlock("go", "remove me"),
		NewParagraph(NewText("also keep")),
	)
	result := Transform(doc, func(n *Node) *Node {
		if n.Type == NodeCodeBlock {
			return nil
		}
		return n
	})
	if len(result.Children) != 2 {
		t.Errorf("Transform() children = %d; want 2", len(result.Children))
	}
}

func TestHasMarkAndGetAttr(t *testing.T) {
	node := NewStyledText("click", Bold(), Link("https://x.com"))
	if !HasMark(node, MarkBold) {
		t.Errorf("HasMark(node, MarkBold) = false; want true")
	}
	if !HasMark(node, MarkLink) {
		t.Errorf("HasMark(node, MarkLink) = false; want true")
	}
	if HasMark(node, MarkItalic) {
		t.Errorf("HasMark(node, MarkItalic) = true; want false")
	}
	href := GetMarkAttr(node, MarkLink, "href")
	if href != "https://x.com" {
		t.Errorf("GetMarkAttr(node, MarkLink, \"href\") = %q; want \"https://x.com\"", href)
	}
}
