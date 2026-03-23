// Package commands implements the CLI interface and core business logic for ihj.
//
// It defines the root Cobra command and its various subcommands (e.g., create,
// edit, transition, branch), orchestrating interactions between the Jira API
// client, the local configuration, and the user interface (both headless and
// interactive TUI modes).
package commands

import (
	"errors"
	"io"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/ui"
)

// App holds all dependencies for command execution.
type App struct {
	Config   *config.Config
	Client   client.API
	UI       ui.UI
	CacheDir string
	Out      io.Writer
	Err      io.Writer

	// LaunchTUI is set by main.go to the Bubble Tea launcher.
	// This avoids the commands package importing bubbletea directly.
	LaunchTUI func(data *LaunchTUIData) error
}

// CancelledError indicates the user intentionally cancelled an operation.
// The CLI should exit cleanly (code 0) rather than printing an error.
type CancelledError struct {
	Operation string
}

func (e *CancelledError) Error() string {
	return e.Operation + " cancelled"
}

// IsCancelled checks whether an error is a user cancellation.
func IsCancelled(err error) bool {
	var ce *CancelledError
	return errors.As(err, &ce)
}

// fetchBoardData loads issues from cache or fetches fresh.
func fetchBoardData(app *App, board *config.BoardConfig, filter string) ([]client.Issue, error) {
	cached, err := jira.LoadCache(app.CacheDir, board.Slug, filter)
	if err == nil {
		return cached.Issues, nil
	}

	return FetchBoardDataFresh(app, board, filter)
}

// FetchBoardDataFresh always fetches from the API (skipping cache), then saves.
// Exported so the TUI can call it for background refresh and filter switching.
func FetchBoardDataFresh(app *App, board *config.BoardConfig, filter string) ([]client.Issue, error) {
	jql, err := config.BuildJQL(board, filter, app.Config.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	issues, err := jira.FetchAllIssues(app.Client, jql, app.Config.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	if saveErr := jira.SaveCache(app.CacheDir, board.Slug, filter, issues); saveErr != nil {
		app.UI.Notify("Warning", "Could not save cache: "+saveErr.Error())
	}
	return issues, nil
}
