package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// handleKeyVim routes key presses through the vim modal system.
func (m AppModel) handleKeyVim(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits regardless of mode.
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.capture {
	case CaptureSearch:
		return m.handleVimSearch(msg)
	case CaptureCommand:
		return m.handleVimCommand(msg)
	default:
		return m.handleVimNormal(msg)
	}
}

// handleVimNormal handles keys in vim normal mode.
func (m AppModel) handleVimNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Mode switches.
	if key.Matches(msg, m.keys.Search) {
		m.capture = CaptureSearch
		cmd := m.list.search.Focus()
		return m, cmd
	}
	if key.Matches(msg, m.keys.Command) {
		m.capture = CaptureCommand
		m.cmdBuf = ""
		return m, nil
	}

	// Esc: exit detail view → clear search. Does NOT quit — use :q.
	if key.Matches(msg, m.keys.Cancel) {
		if m.exitDetailView() {
			return m, nil
		}
		if m.list.search.Value() != "" {
			m.list.search.SetValue("")
			m.list.applyFilter()
			return m, nil
		}
		return m, nil
	}

	// Backspace: navigate back through child history when detail is focused.
	if msg.Code == tea.KeyBackspace && m.view >= ViewDetail {
		if m.detail.CanGoBack() {
			m.detail.GoBack()
			m.recalcLayout()
			iss := m.detail.Issue()
			if iss != nil {
				m.ui.Emit("back", "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
			}
		} else {
			m.exitDetailView()
		}
		return m, nil
	}

	// Toggle full help.
	if msg.String() == "?" {
		m.showHelp = !m.showHelp
		return m, nil
	}

	// Enter: enter fullscreen mode.
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

	// Actions — resolved via the KeyMap (single-char keys in vim mode).
	if action := m.resolveAction(msg); action != ActionNone {
		if model, cmd, handled := m.executeAction(action); handled {
			return model, cmd
		}
	}

	// Navigation and child hint keys (shared with default mode).
	if handled := m.handleNavigation(msg); handled {
		return m, nil
	}

	return m, nil
}

// handleVimSearch handles keys in vim search mode (entered via /).
func (m AppModel) handleVimSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Esc or Enter exits search mode (back to normal), keeping the filter.
	if key.Matches(msg, m.keys.Cancel) || key.Matches(msg, m.keys.Focus) {
		m.capture = CaptureNone
		m.list.search.Blur()
		return m, nil
	}

	// Everything else goes to the search input.
	var cmd tea.Cmd
	prevQuery := m.list.search.Value()
	m.list.search, cmd = m.list.search.Update(msg)
	if m.list.search.Value() != prevQuery {
		m.list.applyFilter()
		m.syncDetail()
	}
	return m, cmd
}

// handleVimCommand handles keys in vim command mode (entered via :).
func (m AppModel) handleVimCommand(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Esc cancels command mode.
	if key.Matches(msg, m.keys.Cancel) {
		m.capture = CaptureNone
		m.cmdBuf = ""
		return m, nil
	}

	// Enter executes the command.
	if key.Matches(msg, m.keys.Focus) {
		cmd := m.cmdBuf
		m.cmdBuf = ""
		m.capture = CaptureNone
		return m.executeVimCommand(cmd)
	}

	// Backspace.
	if msg.Code == tea.KeyBackspace {
		if len(m.cmdBuf) > 0 {
			m.cmdBuf = m.cmdBuf[:len(m.cmdBuf)-1]
		} else {
			// Backspace on empty buffer exits command mode.
			m.capture = CaptureNone
		}
		return m, nil
	}

	// Accumulate printable characters.
	if msg.Text != "" {
		m.cmdBuf += msg.Text
	}

	return m, nil
}

// executeVimCommand runs a ":" command.
func (m AppModel) executeVimCommand(cmd string) (tea.Model, tea.Cmd) {
	switch cmd {
	case "q", "quit":
		return m, tea.Quit
	case "h", "help":
		m.showHelp = !m.showHelp
		return m, nil
	default:
		m.setNotify("Unknown command: :" + cmd)
		return m, nil
	}
}

// renderVimHelpBar renders mode-aware help for vim mode.
func (m *AppModel) renderVimHelpBar(width int) string {
	s := m.styles

	switch m.capture {
	case CaptureSearch:
		bar := s.ActionKey.Render("/") + s.ActionDesc.Render(" search") +
			s.ActionDesc.Render(" | ") +
			s.ActionKey.Render("Enter/Esc") + s.ActionDesc.Render(" done")
		return lipgloss.NewStyle().MaxWidth(width).Render(bar)

	case CaptureCommand:
		prompt := s.ActionKey.Render(":") + s.ActionDesc.Render(m.cmdBuf)
		return lipgloss.NewStyle().MaxWidth(width).Render(prompt)
	}

	// Normal mode: mode indicator + action bindings (with truncation).
	modeTag := s.ActionKey.Render("NORMAL") + s.ActionDesc.Render(" | ")
	modeTagW := lipgloss.Width(modeTag)
	m.help.SetWidth(width - modeTagW)

	return modeTag + m.help.ShortHelpView(m.keys.ShortHelp())
}
