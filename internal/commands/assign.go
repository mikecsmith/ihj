package commands

import (
	"context"
	"fmt"
)

// Assign assigns the given issue to the current authenticated user.
func Assign(ctx context.Context, ws *WorkspaceSession, issueKey string) error {
	if err := ws.Provider.Assign(ctx, issueKey); err != nil {
		ws.Runtime.UI.Notify("Error", fmt.Sprintf("Failed to assign %s.", issueKey))
		return err
	}

	ws.Runtime.UI.Notify("Assigned", fmt.Sprintf("Assigned %s to you.", issueKey))
	return nil
}
