package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

func (m AppModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.vimMode {
		return m.handleKeyVim(msg)
	}

	// Global keys.
	if key.Matches(msg, m.keys.Quit) {
		return m, m.quitCmd()
	}
	if key.Matches(msg, m.keys.Cancel) {
		// Esc: exit detail view → clear search → quit.
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

	// Backspace: navigate back through child history, or exit detail view.
	if msg.Code == tea.KeyBackspace && m.view >= ViewDetail {
		if m.detail.CanGoBack() {
			m.detail.GoBack()
			m.recalcLayout()
			iss := m.detail.Issue()
			if iss != nil {
				m.ui.Emit(EventBack, "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
			}
		} else {
			m.exitDetailView()
		}
		return m, nil
	}

	// Toggle full help.
	if key.Matches(msg, m.keys.Help) {
		m.showHelp = !m.showHelp
		return m, nil
	}

	// Enter: enter fullscreen mode (detail pane fills screen).
	if key.Matches(msg, m.keys.Focus) {
		m.enterFullscreen()
		return m, nil
	}

	// Tab: toggle pane focus (only in split layout).
	if key.Matches(msg, m.keys.Tab) && m.view != ViewFullscreen {
		if m.view == ViewList {
			m.focusDetail()
		} else {
			m.focusList()
		}
		return m, nil
	}

	// Actions (resolved via KeyMap — don't interfere with search).
	if model, cmd, handled := m.executeAction(m.resolveAction(msg)); handled {
		return model, cmd
	}

	// Navigation and child hint keys.
	if handled := m.handleNavigation(msg); handled {
		return m, nil
	}

	// Everything else goes to search input.
	var cmd tea.Cmd
	prevQuery := m.list.search.Value()
	m.list.search, cmd = m.list.search.Update(msg)
	if m.list.search.Value() != prevQuery {
		m.list.applyFilter()
		m.syncDetail()
	}
	return m, cmd
}

// handleNavigation processes cursor movement and child hint keys.
// Returns true if the key was handled.
func (m *AppModel) handleNavigation(msg tea.KeyPressMsg) bool {
	if m.view >= ViewDetail {
		// Detail-focused: arrow keys scroll the detail pane.
		switch {
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.DetailUp):
			m.detail.ScrollUp(1)
			return true
		case key.Matches(msg, m.keys.Down), key.Matches(msg, m.keys.DetailDown):
			m.detail.ScrollDown(1)
			return true
		case key.Matches(msg, m.keys.PageUp):
			m.detail.ScrollUp(m.detailContentH)
			return true
		case key.Matches(msg, m.keys.PageDn):
			m.detail.ScrollDown(m.detailContentH)
			return true
		case key.Matches(msg, m.keys.Home):
			m.detail.ScrollToTop()
			return true
		case key.Matches(msg, m.keys.End):
			m.detail.ScrollToBottom()
			return true
		}

		// Hint keys navigate to child issues.
		if s := msg.String(); len([]rune(s)) == 1 {
			if idx := m.detail.ChildIndexForKey([]rune(s)[0]); idx >= 0 {
				m.detail.NavigateToChild(idx)
				m.recalcLayout()
				iss := m.detail.Issue()
				if iss != nil {
					m.ui.Emit(EventNavigated, "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
				}
				return true
			}
		}
	} else {
		// List-focused: arrow keys move the cursor.
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
			return true
		case key.Matches(msg, m.keys.Down):
			if m.list.cursor < len(m.list.filtered)-1 {
				m.list.cursor++
				m.syncDetail()
			}
			return true
		case key.Matches(msg, m.keys.Home):
			m.list.cursor = 0
			m.syncDetail()
			return true
		case key.Matches(msg, m.keys.End):
			m.list.cursor = max(0, len(m.list.filtered)-1)
			m.syncDetail()
			return true
		case key.Matches(msg, m.keys.PageUp):
			m.list.cursor = max(0, m.list.cursor-m.list.visibleRows())
			m.syncDetail()
			return true
		case key.Matches(msg, m.keys.PageDn):
			m.list.cursor = min(len(m.list.filtered)-1, m.list.cursor+m.list.visibleRows())
			m.syncDetail()
			return true
		case key.Matches(msg, m.keys.DetailUp):
			m.detail.ScrollUp(1)
			return true
		case key.Matches(msg, m.keys.DetailDown):
			m.detail.ScrollDown(1)
			return true
		}
	}
	return false
}

// resolveAction maps a key press to an Action using the default (alt-key) bindings.
func (m *AppModel) resolveAction(msg tea.KeyPressMsg) Action {
	switch {
	case key.Matches(msg, m.keys.Refresh):
		return ActionRefresh
	case key.Matches(msg, m.keys.Filter):
		return ActionFilter
	case key.Matches(msg, m.keys.Assign):
		return ActionAssign
	case key.Matches(msg, m.keys.Transition):
		return ActionTransition
	case key.Matches(msg, m.keys.Open):
		return ActionOpen
	case key.Matches(msg, m.keys.Edit):
		return ActionEdit
	case key.Matches(msg, m.keys.Comment):
		return ActionComment
	case key.Matches(msg, m.keys.Branch):
		return ActionBranch
	case key.Matches(msg, m.keys.Extract):
		return ActionExtract
	case key.Matches(msg, m.keys.New):
		return ActionNew
	case key.Matches(msg, m.keys.Workspace):
		return ActionWorkspace
	default:
		return ActionNone
	}
}

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
		if result.Value != "" {
			// Strip the bullet/spacing prefix added for display.
			selected := strings.TrimPrefix(result.Value, core.GlyphCircle+" ")
			selected = strings.TrimPrefix(selected, "  ")
			if selected == m.filter {
				m.setNotify("Already on filter: " + selected)
			} else {
				m.loading = "Loading " + strings.ToUpper(selected) + "..."
				return m, m.fetchData(selected, fetchOpts{})
			}
		}

	case "workspace":
		if result.Value != "" {
			// Strip the bullet/spacing prefix, then resolve slug from name.
			name := strings.TrimPrefix(result.Value, core.GlyphCircle+" ")
			name = strings.TrimPrefix(name, "  ")
			slug := m.resolveWorkspaceSlug(name)
			if slug == m.ws.Slug {
				m.setNotify("Already on workspace: " + name)
			} else {
				return m, m.switchWorkspace(slug)
			}
		}
	}

	return m, nil
}

// resolveBridgePopup handles popup results that originate from the UI bridge
// (Select/Confirm/InputText). These resolve a channel-based prompt so a
// background command goroutine can continue. Returns handled=true if the
// result was a bridge popup.
func (m AppModel) resolveBridgePopup(result *PopupResult) (tea.Model, tea.Cmd, bool) {
	switch result.ID {
	case "bridge-select":
		idx := result.Index
		if result.Canceled {
			idx = -1
		}
		m.ui.resolveSelect(idx)
		return m, nil, true

	case "bridge-confirm":
		yes := !result.Canceled && result.Index == 0
		m.ui.resolveConfirm(yes)
		return m, nil, true

	case "bridge-input":
		m.ui.resolveInput(result.Text, result.Canceled)
		return m, nil, true
	}

	return m, nil, false
}
