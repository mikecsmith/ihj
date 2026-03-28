package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// FieldDiff represents a single field-level difference shown to the user
// during review (e.g. in the apply command's ReviewDiff dialog).
type FieldDiff struct {
	Field string
	Old   string
	New   string
}

// UI abstracts all user interaction so commands never touch stdin/stdout
// directly. Bubble Tea implements this for the real TUI. Tests provide
// a mock. Headless/CI provides a flag-driven stub.
type UI interface {
	// Select presents a list of options and returns the chosen index.
	// Returns -1 if the user cancels.
	Select(title string, options []string) (int, error)

	// Confirm asks a yes/no question. Returns true for yes.
	Confirm(prompt string) (bool, error)

	// EditText opens the user's editor with initial content and returns
	// the edited result. prefix is used for the temp file name.
	EditText(initial, prefix string, cursorLine int, searchPattern string) (string, error)

	// Notify displays a message to the user (toast, inline, etc).
	Notify(title, message string)

	// CopyToClipboard copies text to the system clipboard.
	CopyToClipboard(text string) error

	// PromptText asks for a single line of text input.
	PromptText(prompt string) (string, error)

	// Status shows a transient progress message (spinner in TUI, stderr in terminal).
	Status(message string)

	// ReviewDiff presents a set of changes to the user and asks them to select
	// an action from the options list. Returns the index of the chosen option,
	// or -1 if cancelled.
	ReviewDiff(title string, changes []FieldDiff, options []string) (int, error)
}

// LaunchTUIData holds everything the TUI needs to start.
// Separating data fetching from TUI construction lets us test both independently.
type LaunchTUIData struct {
	Session   *Session
	Workspace *core.Workspace
	Filter    string
	Items     []*core.WorkItem
	FetchedAt time.Time // When data was fetched — zero value means demo mode.
}

// PrepareTUI fetches board data and builds the registry for the TUI.
func PrepareTUI(s *Session, workspaceSlug, filterName string) (*LaunchTUIData, error) {
	ws, err := s.Config.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return nil, err
	}
	filter := s.Config.ResolveFilter(filterName)

	s.UI.Status(fmt.Sprintf("Loading %s (%s)...", ws.Name, strings.ToUpper(filter)))

	items, err := s.Provider.Search(context.TODO(), filter, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching board data: %w", err)
	}

	return &LaunchTUIData{
		Session:   s,
		Workspace: ws,
		Filter:    filter,
		Items:     items,
		FetchedAt: time.Now(),
	}, nil
}

// RunTUI prepares data and delegates to the Bubble Tea launcher.
func RunTUI(s *Session, workspaceSlug, filterName string) error {
	if s.LaunchTUI == nil {
		return fmt.Errorf("TUI not available (LaunchTUI not configured)")
	}

	data, err := PrepareTUI(s, workspaceSlug, filterName)
	if err != nil {
		return err
	}

	return s.LaunchTUI(data)
}
