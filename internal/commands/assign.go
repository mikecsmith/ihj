package commands

import (
	"context"
	"fmt"
)

// Assign assigns the given issue to the current authenticated user.
func Assign(s *Session, issueKey string) error {
	if err := s.Provider.Assign(context.TODO(), issueKey); err != nil {
		s.UI.Notify("Error", fmt.Sprintf("Failed to assign %s.", issueKey))
		return err
	}

	s.UI.Notify("Assigned", fmt.Sprintf("Assigned %s to you.", issueKey))
	return nil
}
