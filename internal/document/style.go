package document

// StyleSet abstracts terminal styling so the document renderer isn't
// coupled to a specific styling library. The default implementation
// uses Lip Gloss, but callers can provide their own (e.g., plain text,
// HTML, or a different terminal library).
type StyleSet interface {
	// Inline text styling. Each returns the text wrapped in the
	// appropriate style. Implementations should handle reset/cleanup.
	Bold(text string) string
	Italic(text string) string
	Code(text string) string
	Strike(text string) string
	Underline(text string) string
	Dim(text string) string
	Link(text, href string) string

	// Semantic styles for document structure.
	Heading(text string, level int) string
	CodeBlockLabel(lang string) string
	CodeBlockBorder() string
	BlockquoteBorder() string
	HorizontalRule(width int) string

	// Media/attachment placeholder.
	MediaPlaceholder(alt, url string) string
}
