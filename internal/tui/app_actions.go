package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// ── Quit ────────────────────────────────────────────────────────

// quitCmd signals the UI bridge to unblock any pending interactive prompts
// and then returns the Bubble Tea quit command. All quit paths route through
// here so background goroutines waiting on Select/Confirm/InputText don't
// leak when the app exits with a pending prompt.
func (m AppModel) quitCmd() tea.Cmd {
	m.ui.Shutdown()
	return tea.Quit
}

// ── Issue targeting ─────────────────────────────────────────────

// targetIssue returns the issue that actions should operate on.
// When the detail pane has navigated into a child, it returns
// that child — otherwise the list's selected parent.
func (m AppModel) targetIssue() *core.WorkItem {
	issue := m.detail.Issue()
	if issue == nil {
		issue = m.list.SelectedIssue()
	}
	return issue
}

// issueCommand runs commandFn against the currently targeted issue.
// Returns handled=false if no issue is selected. This collapses the
// repeated nil-guard + ID-capture + runCommand pattern used by most
// action branches.
func (m AppModel) issueCommand(commandFn func(issueID string) error) (tea.Model, tea.Cmd, bool) {
	issue := m.targetIssue()
	if issue == nil {
		return m, nil, false
	}
	issueID := issue.ID
	return m, m.runCommand(func() error { return commandFn(issueID) }), true
}

// ── Command lifecycle ───────────────────────────────────────────

// runCommand launches commandFn in a goroutine via tea.Cmd. The result is
// sent back as commandCompleteMsg, which triggers a data reload.
func (m *AppModel) runCommand(commandFn func() error) tea.Cmd {
	m.commandRunning = true
	return func() tea.Msg {
		err := commandFn()
		return commandCompleteMsg{err: err}
	}
}

// ── Action dispatch ─────────────────────────────────────────────

// executeAction performs an action. Returns handled=false only for ActionNone.
func (m AppModel) executeAction(action Action) (tea.Model, tea.Cmd, bool) {
	if action == ActionNone {
		return m, nil, false
	}

	// Suppress actions while a command is running.
	if m.commandRunning {
		return m, nil, false
	}

	switch action {
	case ActionComment:
		return m.issueCommand(func(issueID string) error {
			return commands.Comment(m.ctx, m.wsSess, issueID)
		})

	case ActionExtract:
		return m.issueCommand(func(issueID string) error {
			return commands.Extract(m.ctx, m.wsSess, issueID, commands.ExtractOptions{
				Copy:   true,
				Filter: m.filter,
			})
		})

	case ActionTransition:
		return m.issueCommand(func(issueID string) error {
			return commands.Transition(m.ctx, m.wsSess, issueID)
		})

	case ActionAssign:
		return m.issueCommand(func(issueID string) error {
			return commands.Assign(m.ctx, m.wsSess, issueID)
		})

	case ActionEdit:
		return m.issueCommand(func(issueID string) error {
			return commands.Edit(m.ctx, m.wsSess, issueID, nil)
		})

	case ActionBranch:
		return m.issueCommand(func(issueID string) error {
			return commands.Branch(m.ctx, m.wsSess, issueID)
		})

	case ActionOpen:
		return m.executeOpen()

	case ActionFilter:
		return m.executeFilterSwitch()

	case ActionRefresh:
		m.loading = "Refreshing..."
		return m, m.fetchData(m.filter, fetchOpts{}), true

	case ActionNew:
		return m, m.runCommand(func() error {
			return commands.Create(m.ctx, m.wsSess, nil)
		}), true

	case ActionWorkspace:
		return m.executeWorkspaceSwitch()
	}

	return m, nil, false
}

// ── Action implementations ──────────────────────────────────────

func (m AppModel) executeOpen() (tea.Model, tea.Cmd, bool) {
	issue := m.targetIssue()
	if issue == nil {
		return m, nil, false
	}
	browseURL := m.ws.BrowseURL(issue.ID)
	if browseURL == "" {
		m.setNotify("No browse URL configured")
		return m, nil, true
	}
	issueKey := issue.ID
	cmd := func() tea.Msg {
		if err := commands.OpenInBrowser(browseURL); err != nil {
			return notifyMsg{title: "Open failed", message: err.Error()}
		}
		return notifyMsg{title: "Opened", message: issueKey}
	}
	return m, cmd, true
}

func (m AppModel) executeFilterSwitch() (tea.Model, tea.Cmd, bool) {
	var otherFilters []string
	for filterName := range m.ws.Filters {
		if filterName != m.filter {
			otherFilters = append(otherFilters, filterName)
		}
	}
	if len(otherFilters) == 0 {
		m.setNotify("Only one filter available")
		return m, nil, true
	}
	sort.Strings(otherFilters)

	m.popup.ShowSelect("filter", "Switch Filter", otherFilters)
	m.ui.Emit(EventPopupSelect, "title", "Switch Filter")
	return m, nil, true
}

func (m AppModel) executeWorkspaceSwitch() (tea.Model, tea.Cmd, bool) {
	workspaces := m.runtime.Workspaces
	var otherSlugs []string
	for wsSlug := range workspaces {
		if wsSlug != m.ws.Slug {
			otherSlugs = append(otherSlugs, wsSlug)
		}
	}
	if len(otherSlugs) == 0 {
		m.setNotify("Only one workspace configured")
		return m, nil, true
	}
	sort.Strings(otherSlugs)

	displayNames := make([]string, len(otherSlugs))
	for idx, wsSlug := range otherSlugs {
		displayNames[idx] = workspaceLabel(workspaces[wsSlug])
	}

	m.popup.ShowSelectWithActive("workspace", "Switch Workspace", displayNames, otherSlugs, noActiveItem)
	m.ui.Emit(EventPopupSelect, "title", "Switch Workspace")
	return m, nil, true
}

// workspaceLabel returns a human-readable label for a workspace,
// including the server alias when configured.
func workspaceLabel(workspace *core.Workspace) string {
	label := workspace.Name
	if label == "" {
		label = workspace.Slug
	}
	if workspace.ServerAlias != "" {
		label += " (" + workspace.ServerAlias + ")"
	}
	return label
}
