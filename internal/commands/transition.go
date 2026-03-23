package commands

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/jira"
)

func Transition(app *App, issueKey string) error {
	prefix := strings.ToUpper(strings.SplitN(issueKey, "-", 2)[0])
	var allowed []string
	for _, b := range app.Config.Boards {
		if strings.EqualFold(b.ProjectKey, prefix) {
			allowed = b.Transitions
			break
		}
	}

	transitions, err := app.Client.FetchTransitions(issueKey)
	if err != nil {
		return fmt.Errorf("fetching transitions: %w", err)
	}

	filtered := jira.FilterTransitions(transitions, allowed)
	if len(filtered) == 0 {
		return fmt.Errorf("no available transitions for %s", issueKey)
	}

	names := make([]string, len(filtered))
	for i, t := range filtered {
		names[i] = t.Name
	}

	choice, err := app.UI.Select(fmt.Sprintf("Transition: %s", issueKey), names)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "transition"}
	}

	if err := app.Client.DoTransition(issueKey, filtered[choice].ID); err != nil {
		app.UI.Notify("Error", fmt.Sprintf("Failed to move %s", issueKey))
		return err
	}

	app.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", names[choice]))
	return nil
}
