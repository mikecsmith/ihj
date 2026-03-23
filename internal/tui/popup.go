package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PopupMode indicates what kind of popup is active.
type PopupMode int

const (
	PopupNone   PopupMode = iota
	PopupSelect           // Choose from a list of options.
	PopupInput            // Free-text input (comments, extract prompts).
)

// PopupResult is sent when the user confirms or cancels a popup.
type PopupResult struct {
	ID       string // Identifies which action triggered the popup.
	Index    int    // Selected index (PopupSelect), -1 if cancelled.
	Text     string // Input text (PopupInput), empty if cancelled.
	Canceled bool
}

// PopupModel is a centered floating overlay panel, styled like LazyGit.
type PopupModel struct {
	mode    PopupMode
	id      string   // Action identifier (e.g. "transition", "comment").
	title   string   // Rendered in the top border.
	options []string // For PopupSelect.
	cursor  int      // Currently highlighted option (PopupSelect).

	input textarea.Model // For PopupInput.

	width, height int // Available terminal dimensions.
	styles        *Styles
}

// NewPopupModel creates an inactive popup.
func NewPopupModel(styles *Styles) PopupModel {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 4000
	return PopupModel{
		mode:   PopupNone,
		styles: styles,
		input:  ta,
	}
}

// Active returns true if a popup is currently displayed.
func (p *PopupModel) Active() bool { return p.mode != PopupNone }

// ShowSelect opens a selection popup.
func (p *PopupModel) ShowSelect(id, title string, options []string) {
	p.mode = PopupSelect
	p.id = id
	p.title = title
	p.options = options
	p.cursor = 0
}

// ShowInput opens a text input popup.
func (p *PopupModel) ShowInput(id, title, placeholder string) {
	p.mode = PopupInput
	p.id = id
	p.title = title
	p.input.Reset()
	p.input.Placeholder = placeholder
	p.input.Focus()
}

// SetSize tells the popup how large the terminal is so it can center itself.
func (p *PopupModel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// Close dismisses the popup without producing a result.
func (p *PopupModel) Close() {
	p.mode = PopupNone
	p.input.Blur()
}

// Update handles key events when the popup is active.
// Returns (updated model, optional result msg, tea.Cmd).
func (p *PopupModel) Update(msg tea.Msg) (tea.Cmd, *PopupResult) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch p.mode {
		case PopupSelect:
			return p.updateSelect(msg)
		case PopupInput:
			return p.updateInput(msg)
		}
	}
	return nil, nil
}

func (p *PopupModel) updateSelect(msg tea.KeyPressMsg) (tea.Cmd, *PopupResult) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.options)-1 {
			p.cursor++
		}
	case "home":
		p.cursor = 0
	case "end":
		p.cursor = len(p.options) - 1
	case "enter":
		result := &PopupResult{ID: p.id, Index: p.cursor}
		p.Close()
		return nil, result
	case "esc", "q":
		result := &PopupResult{ID: p.id, Index: -1, Canceled: true}
		p.Close()
		return nil, result
	default:
		// Number keys for quick selection (1-9).
		key := msg.String()
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0]-'0') - 1
			if idx < len(p.options) {
				result := &PopupResult{ID: p.id, Index: idx}
				p.Close()
				return nil, result
			}
		}
	}
	return nil, nil
}

func (p *PopupModel) updateInput(msg tea.KeyPressMsg) (tea.Cmd, *PopupResult) {
	switch msg.String() {
	case "esc":
		result := &PopupResult{ID: p.id, Canceled: true}
		p.Close()
		return nil, result
	case "ctrl+s":
		text := strings.TrimSpace(p.input.Value())
		result := &PopupResult{ID: p.id, Text: text, Canceled: text == ""}
		p.Close()
		return nil, result
	default:
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd, nil
	}
}

// View renders the popup as a centered overlay. The caller composites this
// on top of the main TUI content.
func (p *PopupModel) View() string {
	if p.mode == PopupNone {
		return ""
	}

	theme := DefaultTheme()

	// Popup dimensions — adapt to content and terminal size.
	popupW := max(min(60, p.width-8), 30)

	var body string
	switch p.mode {
	case PopupSelect:
		body = p.renderSelect(theme)
	case PopupInput:
		body = p.renderInput(popupW, theme)
	}

	// Border style.
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.Accent).
		Padding(1, 2).
		Width(popupW)

	box := boxStyle.Render(body)

	// Center the box in the terminal.
	boxH := lipgloss.Height(box)
	boxW := lipgloss.Width(box)

	padTop := max(0, (p.height-boxH)/2)
	padLeft := max(0, (p.width-boxW)/2)

	// Build the centered overlay.
	var b strings.Builder
	for range padTop {
		b.WriteString("\n")
	}
	lines := strings.Split(box, "\n")
	leftPad := strings.Repeat(" ", padLeft)
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(leftPad + line)
	}
	return b.String()
}

func (p *PopupModel) renderSelect(theme *Theme) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Info)
	normalStyle := lipgloss.NewStyle().Foreground(theme.Text)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title) + "\n\n")

	for i, opt := range p.options {
		prefix := "  "
		style := normalStyle
		if i == p.cursor {
			prefix = "▸ "
			style = selectedStyle
		}

		numKey := dimStyle.Render(strings.Repeat(" ", 0))
		if i < 9 {
			numKey = dimStyle.Render(string(rune('1'+i))) + " "
		}
		b.WriteString(prefix + numKey + style.Render(opt) + "\n")
	}

	b.WriteString("\n" + hintStyle.Render("↑↓ navigate • enter confirm • esc cancel"))
	return b.String()
}

func (p *PopupModel) renderInput(width int, theme *Theme) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	innerW := max(
		// account for border + padding
		width-6, 20)
	p.input.SetWidth(innerW)
	p.input.SetHeight(8)

	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title) + "\n\n")
	b.WriteString(p.input.View() + "\n\n")
	b.WriteString(hintStyle.Render("ctrl+s submit • esc cancel"))
	return b.String()
}
