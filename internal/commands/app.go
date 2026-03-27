// Package commands implements the core business logic for ihj.
//
// It orchestrates interactions between the backend provider, the local
// configuration, and the user interface (both headless and interactive
// TUI modes). The Cobra command tree lives in cmd/ihj/.
package commands

import (
	"errors"
	"io"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/storage"
	"github.com/mikecsmith/ihj/internal/ui"
)

// App holds all dependencies for command execution.
type App struct {
	Config   *storage.AppConfig
	Client   jira.API      // Direct Jira client — used by commands not yet migrated to Provider.
	Provider core.Provider // Backend-agnostic provider interface.
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

// FetchBoardDataFresh always fetches from the API (skipping cache), then saves.
// Exported so the TUI can call it for background refresh and filter switching.
func FetchBoardDataFresh(app *App, ws *core.Workspace, filter string) ([]jira.Issue, error) {
	jiraCfg, _ := ws.ProviderConfig.(*jira.Config)

	jql, err := jira.BuildJQL(ws, jiraCfg, filter)
	if err != nil {
		return nil, err
	}

	issues, err := jira.FetchAllIssues(app.Client, jql, jiraCfg.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	if saveErr := jira.SaveCache(app.CacheDir, ws.Slug, filter, issues); saveErr != nil {
		app.UI.Notify("Warning", "Could not save cache: "+saveErr.Error())
	}
	return issues, nil
}
