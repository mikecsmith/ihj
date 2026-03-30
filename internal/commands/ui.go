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

	// InputText collects free-form text from the user (comments, prompts).
	// prompt describes what is being collected. initial provides optional
	// pre-filled content. Returns empty string if the user cancels.
	InputText(prompt, initial string) (string, error)

	// EditDocument opens a structured document editor for YAML frontmatter
	// content. initial is the full document. prefix identifies the temp file.
	// Each UI implementation decides how to present the editor (e.g. $EDITOR
	// for CLI, form popup for desktop).
	EditDocument(initial, prefix string) (string, error)

	// Notify displays a message to the user (toast, inline, etc).
	Notify(title, message string)

	// CopyToClipboard copies text to the system clipboard.
	CopyToClipboard(text string) error

	// PromptText asks for a single line of text input.
	PromptText(prompt string) (string, error)

	// PromptSecret asks for a single line of sensitive input (e.g., tokens).
	// The input is masked with asterisks in the terminal.
	PromptSecret(prompt string) (string, error)

	// Status shows a transient progress message (spinner in TUI, stderr in terminal).
	Status(message string)

	// ReviewDiff presents a set of changes to the user and asks them to select
	// an action from the options list. Returns the index of the chosen option,
	// or -1 if cancelled.
	ReviewDiff(title string, changes []FieldDiff, options []string) (int, error)
}

// UILauncher starts the full-screen interactive UI. It is implemented by the
// tui package and injected into Runtime at startup, breaking what would
// otherwise be a circular dependency between commands and tui.
type UILauncher interface {
	LaunchUI(data *LaunchUIData) error
}

// LaunchUIData holds everything the full-screen UI needs to start.
// Separating data fetching from UI construction lets us test both independently.
type LaunchUIData struct {
	Runtime   *Runtime
	Session   *WorkspaceSession
	Factory   WorkspaceSessionFactory
	Workspace *core.Workspace
	Filter    string
	Items     []*core.WorkItem
	FetchedAt time.Time // When data was fetched — zero value means demo mode.
}

// prepareUI resolves a workspace via the factory, fetches data, and builds the launch payload.
func prepareUI(rt *Runtime, factory WorkspaceSessionFactory, workspaceSlug, filterName string) (*LaunchUIData, error) {
	ws, err := rt.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return nil, err
	}

	wsSess, err := factory(ws.Slug)
	if err != nil {
		return nil, err
	}

	filter := ResolveFilter(filterName)

	rt.UI.Status(fmt.Sprintf("Loading %s (%s)...", ws.Name, strings.ToUpper(filter)))

	items, err := wsSess.Provider.Search(context.TODO(), filter, false)
	if err != nil {
		return nil, fmt.Errorf("fetching board data: %w", err)
	}

	return &LaunchUIData{
		Runtime:   rt,
		Session:   wsSess,
		Factory:   factory,
		Workspace: ws,
		Filter:    filter,
		Items:     items,
		FetchedAt: time.Now(),
	}, nil
}

// RunUI prepares data and delegates to the full-screen UI launcher.
func RunUI(rt *Runtime, factory WorkspaceSessionFactory, workspaceSlug, filterName string) error {
	if rt.Launcher == nil {
		return fmt.Errorf("UI not available (Launcher not configured on Runtime)")
	}

	data, err := prepareUI(rt, factory, workspaceSlug, filterName)
	if err != nil {
		return err
	}

	return rt.Launcher.LaunchUI(data)
}
