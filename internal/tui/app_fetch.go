package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// fetchOpts controls the behaviour of fetchData.
type fetchOpts struct {
	silent  bool // suppress the "Loaded N issues" notification
	startup bool // errors are fatal (e.g. auth failure on initial refresh)
}

// fetchData fetches fresh data from the API for a given filter.
// All fetch paths (startup, refresh, post-command reload) route through
// this single function with different opts.
func (m *AppModel) fetchData(filter string, opts fetchOpts) tea.Cmd {
	provider := m.wsSess.Provider
	return func() tea.Msg {
		items, err := provider.Search(m.ctx, filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err, silent: opts.silent, startup: opts.startup}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
			silent:    opts.silent,
		}
	}
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
