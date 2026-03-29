package commands

import (
	"context"
	"fmt"
	"strings"
)

// Comment collects a comment from the user and posts it to the issue.
func Comment(ws *WorkspaceSession, issueKey string) error {
	raw, err := ws.Runtime.UI.InputText(fmt.Sprintf("Comment on %s", issueKey), "")
	if err != nil {
		return err
	}

	body := strings.TrimSpace(raw)
	if body == "" {
		return &CancelledError{Operation: "comment"}
	}

	if err := ws.Provider.Comment(context.TODO(), issueKey, body); err != nil {
		ws.Runtime.UI.Notify("Error", fmt.Sprintf("Failed to add comment to %s", issueKey))
		return err
	}

	ws.Runtime.UI.Notify("Comment", fmt.Sprintf("Added comment to %s", issueKey))
	return nil
}
