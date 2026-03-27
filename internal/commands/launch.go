package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

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
