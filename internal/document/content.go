package document

// ContentRenderer converts between a provider's native content format
// and the document AST used internally by ihj.
//
// Each backend implements this interface:
//   - Jira: converts between ADF (Atlassian Document Format) and *Node
//   - GitHub/Trello: converts between markdown strings and *Node
//
// For markdown-native backends, the implementation delegates to
// RenderMarkdown/ParseMarkdown from this package.
type ContentRenderer interface {
	// ParseContent converts from the backend's native description format
	// into a document AST. For Jira, raw is ADF JSON (map[string]any).
	// For markdown backends, raw is a string.
	ParseContent(raw any) (*Node, error)

	// RenderContent converts a document AST into the backend's native
	// description format. For Jira, returns an ADF map[string]any.
	// For markdown backends, returns a string.
	RenderContent(node *Node) (any, error)
}
