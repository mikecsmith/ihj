package document

import (
	"fmt"
	"strings"
)

// PlainStyles is a no-op StyleSet that renders plain text with no formatting.
// Used as the fallback when no StyleSet is provided, and useful for
// generating search indexes, plain-text exports, or testing.
type PlainStyles struct{}

var _ StyleSet = PlainStyles{}

func (PlainStyles) Bold(text string) string              { return text }
func (PlainStyles) Italic(text string) string             { return text }
func (PlainStyles) Code(text string) string               { return "`" + text + "`" }
func (PlainStyles) Strike(text string) string             { return "~" + text + "~" }
func (PlainStyles) Underline(text string) string          { return text }
func (PlainStyles) Dim(text string) string                { return text }
func (PlainStyles) Link(text, href string) string         { return fmt.Sprintf("%s (%s)", text, href) }
func (PlainStyles) Heading(text string, _ int) string     { return strings.ToUpper(text) }
func (PlainStyles) CodeBlockLabel(lang string) string     { return fmt.Sprintf("── %s ──", lang) }
func (PlainStyles) CodeBlockBorder() string               { return "│" }
func (PlainStyles) BlockquoteBorder() string              { return "│" }
func (PlainStyles) HorizontalRule(width int) string       { return strings.Repeat("─", width) }
func (PlainStyles) MediaPlaceholder(alt, url string) string {
	return fmt.Sprintf("[%s: %s]", alt, url)
}
