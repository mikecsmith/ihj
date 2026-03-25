package commands

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// ParseComment parses markdown text and returns the ADF payload for the Jira API,
// along with the parsed AST (for local display). Used by both CLI and TUI.
func ParseComment(text string) (adf map[string]any, ast *document.Node, err error) {
	ast, err = document.ParseMarkdownString(text)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing comment: %w", err)
	}
	return jira.RenderADFValue(ast), ast, nil
}

func Comment(app *App, issueKey string) error {
	raw, err := app.UI.EditText("", fmt.Sprintf("j_comment_%s_", issueKey), 1, "")
	if err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}

	body := strings.TrimSpace(raw)
	if body == "" {
		return &CancelledError{Operation: "comment"}
	}

	adfBody, _, err := ParseComment(body)
	if err != nil {
		return err
	}

	if err := app.Client.AddComment(issueKey, adfBody); err != nil {
		app.UI.Notify("Jira Error", fmt.Sprintf("Failed to add comment to %s", issueKey))
		return err
	}

	app.UI.Notify("Jira Comment", fmt.Sprintf("Added comment to %s", issueKey))
	return nil
}
