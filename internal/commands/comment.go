package commands

import (
	"fmt"
	"strings"
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

	if err := app.Provider.Comment(nil, issueKey, body); err != nil {
		app.UI.Notify("Error", fmt.Sprintf("Failed to add comment to %s", issueKey))
		return err
	}

	app.UI.Notify("Comment", fmt.Sprintf("Added comment to %s", issueKey))
	return nil
}
