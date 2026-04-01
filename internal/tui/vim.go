package tui

import (
	"strings"

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

	// Esc in normal mode: pop child navigation, clear search, or quit.
	if key.Matches(msg, m.keys.Cancel) {
		if m.detail.CanGoBack() {
			m.detail.GoBack()
			return m, nil
		}
		if m.list.search.Value() != "" {
			m.list.search.SetValue("")
			m.list.applyFilter()
			return m, nil
		}
		return m, tea.Quit
	}

	// Actions — resolved via the KeyMap (single-char keys in vim mode).
	if action := m.resolveAction(msg); action != ActionNone {
		if model, cmd, handled := m.executeAction(action); handled {
			return model, cmd
		}
	}

	// Navigation.
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

	// Preview scroll.
	case key.Matches(msg, m.keys.PreviewUp):
		m.detail.ScrollUp(1)
		return m, nil
	case key.Matches(msg, m.keys.PreviewDown):
		m.detail.ScrollDown(1)
		return m, nil

	// Enter navigates into child issues.
	case key.Matches(msg, m.keys.EnterChild):
		if iss := m.detail.Issue(); iss != nil && len(iss.Children) > 0 {
			m.detail.NavigateToChild(0)
		}
		return m, nil
	}

	// Number keys 1-9 navigate to nth child issue.
	if msg.Code >= '1' && msg.Code <= '9' && msg.Mod == 0 {
		idx := int(msg.Code-'0') - 1
		if m.detail.NavigateToChild(idx) {
			return m, nil
		}
	}

	return m, nil
}

// handleVimSearch handles keys in vim search mode (entered via /).
func (m AppModel) handleVimSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Esc or Enter exits search mode (back to normal), keeping the filter.
	if key.Matches(msg, m.keys.Cancel) || key.Matches(msg, m.keys.EnterChild) {
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
	if key.Matches(msg, m.keys.EnterChild) {
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

	// Normal mode: show bindings from the KeyMap.
	var parts []string
	for _, k := range m.keys.ActionBindings() {
		if k.Enabled() {
			parts = append(parts, s.ActionKey.Render(k.Help().Key)+" "+s.ActionDesc.Render(k.Help().Desc))
		}
	}

	search := m.keys.Search.Help()
	command := m.keys.Command.Help()
	bar := s.ActionKey.Render("NORMAL") + s.ActionDesc.Render(" | ") +
		strings.Join(parts, s.ActionDesc.Render(" | ")) +
		s.ActionDesc.Render(" | ") +
		s.ActionKey.Render(search.Key) + s.ActionDesc.Render(search.Desc) +
		s.ActionDesc.Render(" | ") +
		s.ActionKey.Render(command.Key) + s.ActionDesc.Render(command.Desc)

	return lipgloss.NewStyle().MaxWidth(width).Render(bar)
}
