package commands

import (
	"fmt"
	"strings"
)

func Comment(s *Session, issueKey string) error {
	raw, err := s.UI.EditText("", fmt.Sprintf("j_comment_%s_", issueKey), 1, "")
	if err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}

	body := strings.TrimSpace(raw)
	if body == "" {
		return &CancelledError{Operation: "comment"}
	}

	if err := s.Provider.Comment(nil, issueKey, body); err != nil {
		s.UI.Notify("Error", fmt.Sprintf("Failed to add comment to %s", issueKey))
		return err
	}

	s.UI.Notify("Comment", fmt.Sprintf("Added comment to %s", issueKey))
	return nil
}
