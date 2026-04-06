package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// quitCmd signals the UI bridge to unblock any pending interactive prompts
// and then returns the Bubble Tea quit command. All quit paths route through
// here so background goroutines waiting on Select/Confirm/InputText don't
// leak when the app exits with a pending prompt.
func (m AppModel) quitCmd() tea.Cmd {
	m.ui.Shutdown()
	return tea.Quit
}

// executeAction performs an action. Returns handled=false only for ActionNone.
func (m AppModel) executeAction(action Action) (tea.Model, tea.Cmd, bool) {
	if action == ActionNone {
		return m, nil, false
	}

	// Suppress actions while a command is running.
	if m.commandRunning {
		return m, nil, false
	}

	iss := m.list.SelectedIssue()

	switch action {
	case ActionComment:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Comment(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionExtract:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Extract(m.ctx, m.wsSess, issKey, commands.ExtractOptions{Copy: true})
			}), true
		}

	case ActionTransition:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Transition(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionAssign:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Assign(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionEdit:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Edit(m.ctx, m.wsSess, issKey, nil)
			}), true
		}

	case ActionOpen:
		if iss != nil {
			url := m.ws.BrowseURL(iss.ID)
			if url == "" {
				m.setNotify("No browse URL configured")
				return m, nil, true
			}
			issKey := iss.ID
			// Run OpenInBrowser as a tea.Cmd so its error (if any) flows
			// back through the TEA event loop as a notifyMsg. Previously
			// this was fired in a bare goroutine with errcheck disabled,
			// which silently swallowed failures on headless machines.
			cmd := func() tea.Msg {
				if err := commands.OpenInBrowser(url); err != nil {
					return notifyMsg{title: "Open failed", message: err.Error()}
				}
				return notifyMsg{title: "Opened", message: issKey}
			}
			return m, cmd, true
		}

	case ActionBranch:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Branch(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionFilter:
		var others []string
		for name := range m.ws.Filters {
			if name != m.filter {
				others = append(others, name)
			}
		}
		if len(others) == 0 {
			m.setNotify("Only one filter available")
			return m, nil, true
		}
		sort.Strings(others)

		// Current filter first with bullet indicator, then the rest.
		filterNames := []string{core.GlyphCircle + " " + m.filter}
		for _, name := range others {
			filterNames = append(filterNames, "  "+name)
		}
		m.popup.ShowSelect("filter", "Switch Filter", filterNames)
		m.ui.Emit(EventPopupSelect, "title", "Switch Filter")
		return m, nil, true

	case ActionRefresh:
		m.loading = "Refreshing..."
		return m, m.fetchFreshData(m.filter), true

	case ActionNew:
		return m, m.runCommand(func() error {
			return commands.Create(m.ctx, m.wsSess, nil)
		}), true

	case ActionWorkspace:
		slugs := make([]string, 0, len(m.runtime.Workspaces))
		for slug := range m.runtime.Workspaces {
			slugs = append(slugs, slug)
		}
		if len(slugs) <= 1 {
			m.setNotify("Only one workspace configured")
			return m, nil, true
		}
		sort.Strings(slugs)

		// Current workspace first with bullet indicator, then the rest.
		wsLabel := func(ws *core.Workspace) string {
			label := ws.Name
			if label == "" {
				label = ws.Slug
			}
			if ws.ServerAlias != "" {
				label += " (" + ws.ServerAlias + ")"
			}
			return label
		}
		names := []string{core.GlyphCircle + " " + wsLabel(m.ws)}
		for _, slug := range slugs {
			if slug == m.ws.Slug {
				continue
			}
			names = append(names, "  "+wsLabel(m.runtime.Workspaces[slug]))
		}
		m.popup.ShowSelect("workspace", "Switch Workspace", names)
		m.ui.Emit(EventPopupSelect, "title", "Switch Workspace")
		return m, nil, true
	}

	return m, nil, false
}

// runCommand launches fn in a goroutine via tea.Cmd. The result is sent
// back as commandCompleteMsg, which triggers a data reload.
func (m *AppModel) runCommand(fn func() error) tea.Cmd {
	m.commandRunning = true
	return func() tea.Msg {
		err := fn()
		return commandCompleteMsg{err: err}
	}
}
