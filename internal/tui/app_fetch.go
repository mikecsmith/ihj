package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

func (m *AppModel) cacheAgeString() string {
	if m.fetchedAt.IsZero() {
		return core.GlyphInfinity // Demo mode.
	}
	elapsed := time.Since(m.fetchedAt).Truncate(time.Second)
	if elapsed < time.Minute {
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
}

// switchWorkspace creates a new session via the factory and fetches data.
func (m *AppModel) switchWorkspace(slug string) tea.Cmd {
	m.loading = "Switching to " + slug + "..."
	factory := m.factory
	ctx := m.ctx
	return func() tea.Msg {
		wsSess, err := factory(slug)
		if err != nil {
			return workspaceSwitchedMsg{slug: slug, err: err}
		}
		filter := commands.ResolveFilter("")
		items, searchErr := wsSess.Provider.Search(ctx, filter, true)
		if searchErr != nil {
			return workspaceSwitchedMsg{slug: slug, err: searchErr}
		}
		return workspaceSwitchedMsg{
			slug:      slug,
			wsSess:    wsSess,
			items:     items,
			fetchedAt: time.Now(),
		}
	}
}

// switchFilter loads data for the new filter. Uses stale cache immediately
// if available, then always fetches fresh data in the background.
func (m *AppModel) switchFilter(filter string) tea.Cmd {
	m.loading = "Loading " + strings.ToUpper(filter) + "..."
	return m.fetchFreshData(filter)
}

// fetchFreshData fetches fresh data from the API for a given filter.
func (m *AppModel) fetchFreshData(filter string) tea.Cmd {
	return m.fetchData(filter, false)
}

// fetchStartupData fetches fresh data on startup. Errors are treated as fatal
// (e.g. auth failures) and cause the TUI to exit.
func (m *AppModel) fetchStartupData(filter string) tea.Cmd {
	provider := m.wsSess.Provider
	return func() tea.Msg {
		items, err := provider.Search(m.ctx, filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err, startup: true}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
			silent:    true,
		}
	}
}

// fetchFreshDataSilent fetches fresh data without showing a notification.
// Used for background reloads after commands complete.
func (m *AppModel) fetchFreshDataSilent(filter string) tea.Cmd {
	return m.fetchData(filter, true)
}

func (m *AppModel) fetchData(filter string, silent bool) tea.Cmd {
	provider := m.wsSess.Provider
	return func() tea.Msg {
		items, err := provider.Search(m.ctx, filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err, silent: silent}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
			silent:    silent,
		}
	}
}
