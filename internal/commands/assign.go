package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mikecsmith/ihj/internal/client"
)

func Assign(app *App, issueKey string) error {
	accountID, err := resolveAccountID(app)
	if err != nil {
		return fmt.Errorf("resolving account ID: %w", err)
	}

	if err := app.Client.AssignIssue(issueKey, accountID); err != nil {
		app.UI.Notify("Jira Error", fmt.Sprintf("Failed to assign %s.", issueKey))
		return err
	}

	app.UI.Notify("Jira Updated", fmt.Sprintf("Assigned %s to you.", issueKey))
	return nil
}

func resolveAccountID(app *App) (string, error) {
	cachePath := filepath.Join(app.CacheDir, "myself.json")

	if data, err := os.ReadFile(cachePath); err == nil {
		var user client.User
		if json.Unmarshal(data, &user) == nil && user.AccountID != "" {
			return user.AccountID, nil
		}
	}

	user, err := app.Client.FetchMyself()
	if err != nil {
		return "", err
	}

	if data, err := json.Marshal(user); err == nil {
		if writeErr := os.WriteFile(cachePath, data, 0o644); writeErr != nil {
			fmt.Fprintf(app.Err, "Warning: could not cache user info: %v\n", writeErr)
		}
	}

	return user.AccountID, nil
}
