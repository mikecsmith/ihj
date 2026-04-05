package commands

import (
	"context"
	"fmt"

	"github.com/mikecsmith/ihj/internal/core"
)

// Transition prompts for a new status and applies the change to the issue.
func Transition(ctx context.Context, ws *WorkspaceSession, issueKey string) error {
	caps := ws.Provider.Capabilities()
	if !caps.HasTransitions && caps.StatusSource != core.StatusSourceEntity {
		return fmt.Errorf("provider %q does not support status transitions", ws.Workspace.Provider)
	}

	current, options, err := ws.Provider.TransitionsFor(ctx, issueKey)
	if err != nil {
		return err
	}
	if len(options) == 0 {
		ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("No transitions available (currently %s)", current))
		return nil
	}

	verb := "Transition"
	if caps.StatusSource == core.StatusSourceEntity && !caps.HasTransitions {
		verb = "Change state"
	}
	prompt := fmt.Sprintf("%s: %s (currently %s)", verb, issueKey, current)

	choice, err := ws.Runtime.UI.Select(prompt, options)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "transition"}
	}

	newStatus := options[choice]
	if err := ws.Provider.Update(ctx, issueKey, &core.Changes{Status: &newStatus}); err != nil {
		ws.Runtime.UI.Notify("Error", fmt.Sprintf("Failed to move %s", issueKey))
		return err
	}

	ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
	return nil
}
