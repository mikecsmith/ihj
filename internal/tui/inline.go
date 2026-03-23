package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// selectModel — inline TUI for choosing from a list of options.
// ---------------------------------------------------------------------------

type selectModel struct {
	title   string
	options []string
	cursor  int
	chosen  int // -1 = cancelled
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(m.options) - 1
		case "enter":
			m.chosen = m.cursor
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.chosen = -1
			return m, tea.Quit
		default:
			// Number keys for quick selection.
			key := msg.String()
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				idx := int(key[0]-'0') - 1
				if idx < len(m.options) {
					m.chosen = idx
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m selectModel) View() tea.View {
	theme := DefaultTheme()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Info)
	normalStyle := lipgloss.NewStyle().Foreground(theme.Text)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	var b strings.Builder
	b.WriteString("\n " + titleStyle.Render(m.title) + "\n\n")

	for i, opt := range m.options {
		prefix := "  "
		style := normalStyle
		if i == m.cursor {
			prefix = "▸ "
			style = selectedStyle
		}
		numKey := " "
		if i < 9 {
			numKey = dimStyle.Render(string(rune('1'+i))) + " "
		}
		b.WriteString(" " + prefix + numKey + style.Render(opt) + "\n")
	}

	b.WriteString("\n " + hintStyle.Render("↑↓ navigate • enter confirm • esc cancel") + "\n")
	return tea.NewView(b.String())
}

// ---------------------------------------------------------------------------
// confirmModel — inline TUI for yes/no confirmation.
// ---------------------------------------------------------------------------

type confirmModel struct {
	prompt string
	yes    bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			m.yes = true
			return m, tea.Quit
		case "n", "N", "esc", "q", "ctrl+c", "enter":
			m.yes = false
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() tea.View {
	theme := DefaultTheme()
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Text)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	return tea.NewView(fmt.Sprintf("\n %s %s\n",
		promptStyle.Render(m.prompt),
		hintStyle.Render("[y/N]"),
	))
}

// ---------------------------------------------------------------------------
// promptModel — inline TUI for single-line text input.
// ---------------------------------------------------------------------------

type promptModel struct {
	prompt   string
	input    textinput.Model
	value    string
	canceled bool
	ready    bool
}

func (m promptModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.ready {
		m.input = textinput.New()
		m.input.Placeholder = "..."
		m.input.SetWidth(50)
		m.input.Focus()
		m.ready = true
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.value = strings.TrimSpace(m.input.Value())
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m promptModel) View() tea.View {
	theme := DefaultTheme()
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	var b strings.Builder
	b.WriteString("\n " + promptStyle.Render(m.prompt) + "\n\n")
	b.WriteString(" " + m.input.View() + "\n\n")
	b.WriteString(" " + hintStyle.Render("enter submit • esc cancel") + "\n")
	return tea.NewView(b.String())
}
