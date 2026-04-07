package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// ── Top-level key handler ───────────────────────────────────────

func (m AppModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.vimMode {
		return m.handleKeyVim(msg)
	}

	keys := m.keys

	// ── Global keys ──

	if key.Matches(msg, keys.Quit) {
		return m, m.quitCmd()
	}

	if key.Matches(msg, keys.Cancel) {
		return m.handleEscape()
	}

	if msg.Code == tea.KeyBackspace && m.view >= ViewDetail {
		return m.handleBackspace()
	}

	if key.Matches(msg, keys.Help) {
		m.showHelp = !m.showHelp
		return m, nil
	}

	// ── Pane focus ──

	if key.Matches(msg, keys.Focus) {
		m.enterFullscreen()
		return m, nil
	}

	if key.Matches(msg, keys.Tab) && m.view != ViewFullscreen {
		if m.view == ViewList {
			m.focusDetail()
		} else {
			m.focusList()
		}
		return m, nil
	}

	// ── Actions ──

	if model, cmd, handled := m.executeAction(m.resolveAction(msg)); handled {
		return model, cmd
	}

	// ── Navigation and child hint keys ──

	if handled := m.handleNavigation(msg); handled {
		return m, nil
	}

	// ── Search input fallthrough ──

	return m.forwardToSearch(msg)
}

// ── Escape / backspace ──────────────────────────────────────────

// handleEscape implements the cascading Esc behavior:
// exit detail view → clear search → quit.
func (m AppModel) handleEscape() (tea.Model, tea.Cmd) {
	if m.exitDetailView() {
		return m, nil
	}
	if m.list.search.Value() != "" {
		m.list.search.SetValue("")
		m.list.applyFilter()
		return m, nil
	}
	return m, m.quitCmd()
}

// handleBackspace navigates back through child history, or exits the detail view.
func (m AppModel) handleBackspace() (tea.Model, tea.Cmd) {
	if m.detail.CanGoBack() {
		m.detail.GoBack()
		m.recalcLayout()
		if issue := m.detail.Issue(); issue != nil {
			m.ui.Emit(EventBack, "id", issue.ID, "breadcrumb", m.detail.Breadcrumb())
		}
	} else {
		m.exitDetailView()
	}
	return m, nil
}

// ── Navigation ──────────────────────────────────────────────────

// handleNavigation processes cursor movement and child hint keys.
// Returns true if the key was handled.
func (m *AppModel) handleNavigation(msg tea.KeyPressMsg) bool {
	if m.view >= ViewDetail {
		return m.handleDetailNavigation(msg)
	}
	return m.handleListNavigation(msg)
}

func (m *AppModel) handleDetailNavigation(msg tea.KeyPressMsg) bool {
	keys := m.keys

	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.DetailUp):
		m.detail.ScrollUp(scrollLines)
		return true
	case key.Matches(msg, keys.Down), key.Matches(msg, keys.DetailDown):
		m.detail.ScrollDown(scrollLines)
		return true
	case key.Matches(msg, keys.PageUp):
		m.detail.ScrollUp(m.detailContentH)
		return true
	case key.Matches(msg, keys.PageDn):
		m.detail.ScrollDown(m.detailContentH)
		return true
	case key.Matches(msg, keys.Home):
		m.detail.ScrollToTop()
		return true
	case key.Matches(msg, keys.End):
		m.detail.ScrollToBottom()
		return true
	}

	// Hint keys navigate to child issues.
	return m.tryChildNavigation(msg)
}

func (m *AppModel) tryChildNavigation(msg tea.KeyPressMsg) bool {
	pressed := msg.String()
	if len([]rune(pressed)) != 1 {
		return false
	}
	childIndex := m.detail.ChildIndexForKey([]rune(pressed)[0])
	if childIndex < 0 {
		return false
	}
	m.detail.NavigateToChild(childIndex)
	m.recalcLayout()
	if issue := m.detail.Issue(); issue != nil {
		m.ui.Emit(EventNavigated, "id", issue.ID, "breadcrumb", m.detail.Breadcrumb())
	}
	return true
}

func (m *AppModel) handleListNavigation(msg tea.KeyPressMsg) bool {
	keys := m.keys

	switch {
	case key.Matches(msg, keys.Up):
		if m.list.cursor > 0 {
			m.list.cursor--
			m.syncDetail()
		}
		return true
	case key.Matches(msg, keys.Down):
		if m.list.cursor < len(m.list.filtered)-1 {
			m.list.cursor++
			m.syncDetail()
		}
		return true
	case key.Matches(msg, keys.Home):
		m.list.cursor = 0
		m.syncDetail()
		return true
	case key.Matches(msg, keys.End):
		m.list.cursor = max(0, len(m.list.filtered)-1)
		m.syncDetail()
		return true
	case key.Matches(msg, keys.PageUp):
		m.list.cursor = max(0, m.list.cursor-m.list.visibleRows())
		m.syncDetail()
		return true
	case key.Matches(msg, keys.PageDn):
		m.list.cursor = min(len(m.list.filtered)-1, m.list.cursor+m.list.visibleRows())
		m.syncDetail()
		return true
	case key.Matches(msg, keys.DetailUp):
		m.detail.ScrollUp(scrollLines)
		return true
	case key.Matches(msg, keys.DetailDown):
		m.detail.ScrollDown(scrollLines)
		return true
	}
	return false
}

// ── Search ──────────────────────────────────────────────────────

func (m AppModel) forwardToSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	previousQuery := m.list.search.Value()
	var cmd tea.Cmd
	m.list.search, cmd = m.list.search.Update(msg)
	if m.list.search.Value() != previousQuery {
		m.list.applyFilter()
		m.syncDetail()
	}
	return m, cmd
}

// ── Action resolution ───────────────────────────────────────────

// resolveAction maps a key press to an Action using the default (alt-key) bindings.
func (m *AppModel) resolveAction(msg tea.KeyPressMsg) Action {
	keys := m.keys

	switch {
	case key.Matches(msg, keys.Refresh):
		return ActionRefresh
	case key.Matches(msg, keys.Filter):
		return ActionFilter
	case key.Matches(msg, keys.Assign):
		return ActionAssign
	case key.Matches(msg, keys.Transition):
		return ActionTransition
	case key.Matches(msg, keys.Open):
		return ActionOpen
	case key.Matches(msg, keys.Edit):
		return ActionEdit
	case key.Matches(msg, keys.Comment):
		return ActionComment
	case key.Matches(msg, keys.Branch):
		return ActionBranch
	case key.Matches(msg, keys.Extract):
		return ActionExtract
	case key.Matches(msg, keys.New):
		return ActionNew
	case key.Matches(msg, keys.Workspace):
		return ActionWorkspace
	default:
		return ActionNone
	}
}

// ── Popup results ───────────────────────────────────────────────

func (m AppModel) handlePopupResult(result *PopupResult) (tea.Model, tea.Cmd) {
	// Bridge popups resolve a channel-based prompt for a background command.
	if model, cmd, handled := m.resolveBridgePopup(result); handled {
		return model, cmd
	}

	// TUI-only popups (filter/workspace switcher, etc.).
	if result.Canceled {
		m.setNotify("Cancelled")
		return m, nil
	}

	switch result.ID {
	case "filter":
		return m.handleFilterSelection(result.Value)
	case "workspace":
		return m.handleWorkspaceSelection(result.Value)
	}

	return m, nil
}

func (m AppModel) handleFilterSelection(filterName string) (tea.Model, tea.Cmd) {
	if filterName == "" {
		return m, nil
	}
	if filterName == m.filter {
		m.setNotify("Already on filter: " + filterName)
		return m, nil
	}
	m.loading = "Loading " + strings.ToUpper(filterName) + "..."
	return m, m.fetchData(filterName, fetchOpts{})
}

func (m AppModel) handleWorkspaceSelection(workspaceSlug string) (tea.Model, tea.Cmd) {
	if workspaceSlug == "" {
		return m, nil
	}
	if workspaceSlug == m.ws.Slug {
		m.setNotify("Already on this workspace")
		return m, nil
	}
	return m, m.switchWorkspace(workspaceSlug)
}

// ── Bridge popup resolution ─────────────────────────────────────

// resolveBridgePopup handles popup results that originate from the UI bridge
// (Select/Confirm/InputText). These resolve a channel-based prompt so a
// background command goroutine can continue. Returns handled=true if the
// result was a bridge popup.
func (m AppModel) resolveBridgePopup(result *PopupResult) (tea.Model, tea.Cmd, bool) {
	switch result.ID {
	case "bridge-select":
		selectedIndex := result.Index
		if result.Canceled {
			selectedIndex = -1
		}
		m.ui.resolveSelect(selectedIndex)
		return m, nil, true

	case "bridge-confirm":
		confirmed := !result.Canceled && result.Index == 0
		m.ui.resolveConfirm(confirmed)
		return m, nil, true

	case "bridge-input":
		m.ui.resolveInput(result.Text, result.Canceled)
		return m, nil, true
	}

	return m, nil, false
}
