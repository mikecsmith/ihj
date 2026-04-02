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

	switch m.inputMode {
	case ModeSearch:
		return m.handleVimSearch(msg)
	case ModeCommand:
		return m.handleVimCommand(msg)
	default:
		return m.handleVimNormal(msg)
	}
}

// handleVimNormal handles keys in vim normal mode.
func (m AppModel) handleVimNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Mode switches.
	if key.Matches(msg, m.keys.Search) {
		m.inputMode = ModeSearch
		cmd := m.list.search.Focus()
		return m, cmd
	}
	if key.Matches(msg, m.keys.Command) {
		m.inputMode = ModeCommand
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
	if msg.Code == tea.KeyBackspace && (m.focused || m.detailFocused) {
		if m.detail.CanGoBack() {
			m.detail.GoBack()
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

	// Enter: enter focus mode.
	if key.Matches(msg, m.keys.Focus) {
		m.focused = true
		m.detailFocused = true
		m.recalcLayout()
		m.ui.Emit("focus:entered")
		return m, nil
	}

	// Tab: toggle pane focus.
	if key.Matches(msg, m.keys.Tab) && !m.focused {
		m.detailFocused = !m.detailFocused
		if m.detailFocused {
			m.ui.Emit("pane:detail")
		} else {
			m.ui.Emit("pane:list")
		}
		return m, nil
	}

	// Actions — resolved via the KeyMap (single-char keys in vim mode).
	if action := m.resolveAction(msg); action != ActionNone {
		if model, cmd, handled := m.executeAction(action); handled {
			return model, cmd
		}
	}

	// Navigation — when detail is focused, scroll detail instead of list.
	if m.detailFocused || m.focused {
		switch {
		case key.Matches(msg, m.keys.Up):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.detail.ScrollDown(1)
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.detail.ScrollUp(m.previewContentH)
			return m, nil
		case key.Matches(msg, m.keys.PageDn):
			m.detail.ScrollDown(m.previewContentH)
			return m, nil
		case key.Matches(msg, m.keys.Home):
			m.detail.ScrollToTop()
			return m, nil
		case key.Matches(msg, m.keys.End):
			m.detail.ScrollToBottom()
			return m, nil
		case key.Matches(msg, m.keys.PreviewUp):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewDown):
			m.detail.ScrollDown(1)
			return m, nil
		}
	} else {
		switch {
		case key.Matches(msg, m.keys.Down):
			if m.list.cursor < len(m.list.filtered)-1 {
				m.list.cursor++
				m.syncDetail()
			}
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
			return m, nil
		case key.Matches(msg, m.keys.Home):
			m.list.cursor = 0
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.End):
			m.list.cursor = max(0, len(m.list.filtered)-1)
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.list.cursor = max(0, m.list.cursor-m.list.visibleRows())
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PageDn):
			m.list.cursor = min(len(m.list.filtered)-1, m.list.cursor+m.list.visibleRows())
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PreviewUp):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewDown):
			m.detail.ScrollDown(1)
			return m, nil
		}
	}

	// Hint keys navigate to child issues when detail pane is active.
	if m.detailFocused || m.focused {
		if s := msg.String(); len([]rune(s)) == 1 {
			if idx := m.detail.ChildIndexForKey([]rune(s)[0]); idx >= 0 {
				m.detail.NavigateToChild(idx)
				iss := m.detail.Issue()
				if iss != nil {
					m.ui.Emit("navigated", "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
				}
				return m, nil
			}
		}
	}

	return m, nil
}

// handleVimSearch handles keys in vim search mode (entered via /).
func (m AppModel) handleVimSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Esc or Enter exits search mode (back to normal), keeping the filter.
	if key.Matches(msg, m.keys.Cancel) || key.Matches(msg, m.keys.Focus) {
		m.inputMode = ModeNormal
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
		m.inputMode = ModeNormal
		m.cmdBuf = ""
		return m, nil
	}

	// Enter executes the command.
	if key.Matches(msg, m.keys.Focus) {
		cmd := m.cmdBuf
		m.cmdBuf = ""
		m.inputMode = ModeNormal
		return m.executeVimCommand(cmd)
	}

	// Backspace.
	if msg.Code == tea.KeyBackspace {
		if len(m.cmdBuf) > 0 {
			m.cmdBuf = m.cmdBuf[:len(m.cmdBuf)-1]
		} else {
			// Backspace on empty buffer exits command mode.
			m.inputMode = ModeNormal
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

	switch m.inputMode {
	case ModeSearch:
		bar := s.ActionKey.Render("/") + s.ActionDesc.Render(" search") +
			s.ActionDesc.Render(" | ") +
			s.ActionKey.Render("Enter/Esc") + s.ActionDesc.Render(" done")
		return lipgloss.NewStyle().MaxWidth(width).Render(bar)

	case ModeCommand:
		prompt := s.ActionKey.Render(":") + s.ActionDesc.Render(m.cmdBuf)
		return lipgloss.NewStyle().MaxWidth(width).Render(prompt)
	}

	// Normal mode: mode indicator + action bindings (with truncation).
	modeTag := s.ActionKey.Render("NORMAL") + s.ActionDesc.Render(" | ")
	modeTagW := lipgloss.Width(modeTag)
	m.help.SetWidth(width - modeTagW)

	return modeTag + m.help.ShortHelpView(m.keys.ShortHelp())
}
