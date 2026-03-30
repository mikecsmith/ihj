package commands

import (
	"context"
	"fmt"

	"github.com/mikecsmith/ihj/internal/core"
)

// Transition prompts for a new status and applies the change to the issue.
func Transition(ctx context.Context, ws *WorkspaceSession, issueKey string) error {
	if !ws.Provider.Capabilities().HasTransitions {
		return fmt.Errorf("provider %q does not support status transitions", ws.Workspace.Provider)
	}

	statuses := ws.Workspace.Statuses
	if len(statuses) == 0 {
		return fmt.Errorf("no statuses configured for workspace %q", ws.Workspace.Slug)
	}

	choice, err := ws.Runtime.UI.Select(fmt.Sprintf("Transition: %s", issueKey), statuses)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "transition"}
	}

	newStatus := statuses[choice]
	if err := ws.Provider.Update(ctx, issueKey, &core.Changes{Status: &newStatus}); err != nil {
		ws.Runtime.UI.Notify("Error", fmt.Sprintf("Failed to move %s", issueKey))
		return err
	}

	ws.Runtime.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
	return nil
}
