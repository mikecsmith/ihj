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

// issueCommand runs fn against the selected issue. Returns handled=false
// if iss is nil (no issue selected). This collapses the repeated
// nil-guard + ID-capture + runCommand pattern used by most action branches.
func (m AppModel) issueCommand(iss *core.WorkItem, fn func(id string) error) (tea.Model, tea.Cmd, bool) {
	if iss == nil {
		return m, nil, false
	}
	id := iss.ID
	return m, m.runCommand(func() error { return fn(id) }), true
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
		return m.issueCommand(iss, func(id string) error {
			return commands.Comment(m.ctx, m.wsSess, id)
		})

	case ActionExtract:
		return m.issueCommand(iss, func(id string) error {
			return commands.Extract(m.ctx, m.wsSess, id, commands.ExtractOptions{Copy: true})
		})

	case ActionTransition:
		return m.issueCommand(iss, func(id string) error {
			return commands.Transition(m.ctx, m.wsSess, id)
		})

	case ActionAssign:
		return m.issueCommand(iss, func(id string) error {
			return commands.Assign(m.ctx, m.wsSess, id)
		})

	case ActionEdit:
		return m.issueCommand(iss, func(id string) error {
			return commands.Edit(m.ctx, m.wsSess, id, nil)
		})

	case ActionBranch:
		return m.issueCommand(iss, func(id string) error {
			return commands.Branch(m.ctx, m.wsSess, id)
		})

	case ActionOpen:
		if iss == nil {
			return m, nil, false
		}
		url := m.ws.BrowseURL(iss.ID)
		if url == "" {
			m.setNotify("No browse URL configured")
			return m, nil, true
		}
		issKey := iss.ID
		cmd := func() tea.Msg {
			if err := commands.OpenInBrowser(url); err != nil {
				return notifyMsg{title: "Open failed", message: err.Error()}
			}
			return notifyMsg{title: "Opened", message: issKey}
		}
		return m, cmd, true

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

		filterNames := []string{core.GlyphCircle + " " + m.filter}
		for _, name := range others {
			filterNames = append(filterNames, "  "+name)
		}
		m.popup.ShowSelect("filter", "Switch Filter", filterNames)
		m.ui.Emit(EventPopupSelect, "title", "Switch Filter")
		return m, nil, true

	case ActionRefresh:
		m.loading = "Refreshing..."
		return m, m.fetchData(m.filter, fetchOpts{}), true

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
