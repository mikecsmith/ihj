package document

import (
	"fmt"
	"strings"
)

// ANSIStyles is a StyleSet that uses raw ANSI escape codes.
// This provides basic terminal formatting without depending on Lip Gloss,
// useful for CLI output and testing.
type ANSIStyles struct{}

var _ StyleSet = ANSIStyles{}

func (ANSIStyles) Bold(text string) string      { return "\033[1m" + text + "\033[0m" }
func (ANSIStyles) Italic(text string) string    { return "\033[3m" + text + "\033[0m" }
func (ANSIStyles) Code(text string) string      { return "\033[100m\033[36m" + text + "\033[0m" }
func (ANSIStyles) Strike(text string) string    { return "\033[2m~" + text + "~\033[0m" }
func (ANSIStyles) Underline(text string) string { return "\033[4m" + text + "\033[0m" }
func (ANSIStyles) Dim(text string) string       { return "\033[2m" + text + "\033[0m" }

func (ANSIStyles) Link(text, href string) string {
	return fmt.Sprintf("\033]8;;%s\a\033[34m\033[4m%s\033[24m\033[0m\033]8;;\a", href, text)
}

func (ANSIStyles) Heading(text string, _ int) string {
	return "\033[36m\033[1m" + strings.ToUpper(text) + "\033[0m"
}

func (ANSIStyles) CodeBlockLabel(lang string) string {
	return fmt.Sprintf("\033[100m\033[1m   %s \033[0m", lang)
}

func (ANSIStyles) CodeBlockBorder() string         { return "┃" }
func (ANSIStyles) BlockquoteBorder() string        { return "│" }
func (ANSIStyles) HorizontalRule(width int) string { return strings.Repeat("─", width) }

func (ANSIStyles) MediaPlaceholder(alt, url string) string {
	return fmt.Sprintf("[%s: %s]", alt, url)
}
