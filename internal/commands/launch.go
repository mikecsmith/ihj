package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
)

// LaunchTUIData holds everything the TUI needs to start.
// Separating data fetching from TUI construction lets us test both independently.
type LaunchTUIData struct {
	App       *App
	Board     *config.BoardConfig
	Filter    string
	Views     []jira.IssueView
	FetchedAt time.Time // When data was fetched — zero value means demo mode.
}

// PrepareTUI fetches board data and builds the registry for the TUI.
func PrepareTUI(app *App, boardSlug, filterName string) (*LaunchTUIData, error) {
	board, err := app.Config.ResolveBoard(boardSlug)
	if err != nil {
		return nil, err
	}
	filter := app.Config.ResolveFilter(filterName)

	app.UI.Status(fmt.Sprintf("Loading %s (%s)...", board.Name, strings.ToUpper(filter)))

	issues, err := fetchBoardData(app, board, filter)
	if err != nil {
		return nil, fmt.Errorf("fetching board data: %w", err)
	}

	registry := jira.BuildRegistry(issues)
	jira.LinkChildren(registry)

	// Flatten to a slice for the TUI model.
	views := make([]jira.IssueView, 0, len(registry))
	for _, v := range registry {
		views = append(views, *v)
	}

	return &LaunchTUIData{
		App:       app,
		Board:     board,
		Filter:    filter,
		Views:     views,
		FetchedAt: time.Now(),
	}, nil
}

// RunTUI prepares data and delegates to the Bubble Tea launcher.
func RunTUI(app *App, boardSlug, filterName string) error {
	if app.LaunchTUI == nil {
		return fmt.Errorf("TUI not available (LaunchTUI not configured)")
	}

	data, err := PrepareTUI(app, boardSlug, filterName)
	if err != nil {
		return err
	}

	return app.LaunchTUI(data)
}
