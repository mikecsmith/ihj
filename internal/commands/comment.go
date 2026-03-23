package commands

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/document"
)

func Comment(app *App, issueKey string) error {
	raw, err := app.UI.EditText("", fmt.Sprintf("j_comment_%s_", issueKey), 1, "")
	if err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}

	body := strings.TrimSpace(raw)
	if body == "" {
		return &CancelledError{Operation: "comment"}
	}

	ast, err := document.ParseMarkdownString(body)
	if err != nil {
		return fmt.Errorf("parsing markdown: %w", err)
	}
	adfBody := document.RenderADFValue(ast)

	if err := app.Client.AddComment(issueKey, adfBody); err != nil {
		app.UI.Notify("Jira Error", fmt.Sprintf("Failed to add comment to %s", issueKey))
		return err
	}

	app.UI.Notify("Jira Comment", fmt.Sprintf("Added comment to %s", issueKey))
	return nil
}
