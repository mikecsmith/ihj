package commands

import (
	"fmt"
)

func Assign(s *Session, issueKey string) error {
	if err := s.Provider.Assign(nil, issueKey); err != nil {
		s.UI.Notify("Error", fmt.Sprintf("Failed to assign %s.", issueKey))
		return err
	}

	s.UI.Notify("Assigned", fmt.Sprintf("Assigned %s to you.", issueKey))
	return nil
}
