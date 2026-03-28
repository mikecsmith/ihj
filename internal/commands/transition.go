package commands

import (
	"context"
	"fmt"

	"github.com/mikecsmith/ihj/internal/core"
)

// Transition prompts for a new status and applies the change to the issue.
func Transition(s *Session, workspaceSlug, issueKey string) error {
	ws, err := s.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return err
	}

	if !s.Provider.Capabilities().HasTransitions {
		return fmt.Errorf("provider %q does not support status transitions", ws.Provider)
	}

	statuses := ws.Statuses
	if len(statuses) == 0 {
		return fmt.Errorf("no statuses configured for workspace %q", ws.Slug)
	}

	choice, err := s.UI.Select(fmt.Sprintf("Transition: %s", issueKey), statuses)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "transition"}
	}

	newStatus := statuses[choice]
	if err := s.Provider.Update(context.TODO(), issueKey, &core.Changes{Status: &newStatus}); err != nil {
		s.UI.Notify("Error", fmt.Sprintf("Failed to move %s", issueKey))
		return err
	}

	s.UI.Notify(issueKey, fmt.Sprintf("Moved to %s", newStatus))
	return nil
}
