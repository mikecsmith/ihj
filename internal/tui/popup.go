package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// ── Layout constants ────────────────────────────────────────────

const (
	popupMaxWidth       = 80 // Maximum popup panel width.
	popupMinWidth       = 30 // Minimum popup panel width.
	popupHorizontalPad  = 8  // Breathing room subtracted from terminal width.
	popupBorderPadding  = 6  // Border + padding consumed by the box frame.
	popupMinInnerWidth  = 20 // Floor for the text-input inner width.
	popupInputHeight    = 15 // Visible rows in the text-input area.
	popupInputCharLimit = 4000

	selectViewportMargin  = 10 // Rows reserved for title, hints, and box chrome.
	selectMinVisibleItems = 5  // Minimum items shown even in a tiny terminal.
)

// ── Types ───────────────────────────────────────────────────────

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
	Value    string // The exact string selected from the options list.
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
	styles        *terminal.Styles
	keys          terminal.KeyMap
}

// ── Lifecycle ───────────────────────────────────────────────────

// NewPopupModel creates an inactive popup.
func NewPopupModel(styles *terminal.Styles, keys terminal.KeyMap) PopupModel {
	textArea := textarea.New()
	textArea.ShowLineNumbers = false
	textArea.CharLimit = popupInputCharLimit
	return PopupModel{
		mode:   PopupNone,
		styles: styles,
		keys:   keys,
		input:  textArea,
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
func (p *PopupModel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Close dismisses the popup without producing a result.
func (p *PopupModel) Close() {
	p.mode = PopupNone
	p.input.Blur()
}

// ── Update handlers ─────────────────────────────────────────────

// Update handles key events when the popup is active.
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
	keys := p.keys

	switch {
	case key.Matches(msg, keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Matches(msg, keys.Down):
		if p.cursor < len(p.options)-1 {
			p.cursor++
		}
	case key.Matches(msg, keys.Home):
		p.cursor = 0
	case key.Matches(msg, keys.End):
		p.cursor = len(p.options) - 1
	case key.Matches(msg, keys.Submit), key.Matches(msg, keys.Focus):
		result := &PopupResult{ID: p.id, Index: p.cursor, Value: p.options[p.cursor]}
		p.Close()
		return nil, result
	case key.Matches(msg, keys.Cancel), key.Matches(msg, keys.Quit):
		result := &PopupResult{ID: p.id, Index: -1, Canceled: true}
		p.Close()
		return nil, result
	default:
		return p.tryHintKeySelect(msg)
	}
	return nil, nil
}

// tryHintKeySelect checks whether the key press matches a hint shortcut
// (1-9, 0, a-z) and returns the corresponding selection result.
func (p *PopupModel) tryHintKeySelect(msg tea.KeyPressMsg) (tea.Cmd, *PopupResult) {
	pressed := msg.String()
	if len([]rune(pressed)) != 1 {
		return nil, nil
	}
	idx := p.hintIndex([]rune(pressed)[0])
	if idx < 0 || idx >= len(p.options) {
		return nil, nil
	}
	result := &PopupResult{ID: p.id, Index: idx, Value: p.options[idx]}
	p.Close()
	return nil, result
}

// hintIndex returns the option index that would be selected by pressing
// the given rune, or -1 when the rune isn't a hint key.
func (p *PopupModel) hintIndex(r rune) int {
	for idx, hint := range p.keys.HintKeys() {
		if hint == r {
			return idx
		}
	}
	return -1
}

func (p *PopupModel) updateInput(msg tea.KeyPressMsg) (tea.Cmd, *PopupResult) {
	keys := p.keys

	switch {
	case key.Matches(msg, keys.Cancel), key.Matches(msg, keys.Quit):
		result := &PopupResult{ID: p.id, Canceled: true}
		p.Close()
		return nil, result
	case key.Matches(msg, keys.Submit):
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

// ── Rendering ───────────────────────────────────────────────────

// View renders the popup as a centered overlay. The caller composites this
// on top of the main TUI content.
func (p *PopupModel) View() string {
	if p.mode == PopupNone {
		return ""
	}

	theme := p.styles.Theme()
	popupWidth := p.width - popupHorizontalPad
	if popupWidth > popupMaxWidth {
		popupWidth = popupMaxWidth
	}
	if popupWidth < popupMinWidth {
		popupWidth = popupMinWidth
	}

	var body string
	switch p.mode {
	case PopupSelect:
		body = p.renderSelect()
	case PopupInput:
		body = p.renderInput(popupWidth)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Accent).
		Padding(1, 2).
		Width(popupWidth)

	return boxStyle.Render(body)
}

func (p *PopupModel) renderSelect() string {
	theme := p.styles.Theme()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Info)
	normalStyle := lipgloss.NewStyle().Foreground(theme.Text)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	var buf strings.Builder
	buf.WriteString(titleStyle.Render(p.title) + "\n\n")

	// Calculate a safe sliding window so the popup never exceeds terminal height.
	maxVisible := max(p.height-selectViewportMargin, selectMinVisibleItems)
	start, end := CalculateWindow(p.cursor, len(p.options), maxVisible)

	if start > 0 {
		buf.WriteString(dimStyle.Render("  "+core.GlyphArrowUp+"  ...") + "\n")
	}

	hints := p.keys.HintKeys()
	for idx := start; idx < end; idx++ {
		option := p.options[idx]
		prefix := "  "
		style := normalStyle
		if idx == p.cursor {
			prefix = core.GlyphTriangle + " "
			style = selectedStyle
		}

		shortcut := dimStyle.Render("  ")
		if idx < len(hints) {
			shortcut = dimStyle.Render(string(hints[idx])) + " "
		}
		buf.WriteString(prefix + shortcut + style.Render(option) + "\n")
	}

	if end < len(p.options) {
		buf.WriteString(dimStyle.Render("  "+core.GlyphArrowDown+"  ...") + "\n")
	}

	buf.WriteString("\n" + hintStyle.Render(
		core.GlyphArrowUp+core.GlyphArrowDown+" Navigate "+
			core.GlyphDot+" Enter Confirm "+
			core.GlyphDot+" Esc Cancel"))
	return buf.String()
}

func (p *PopupModel) renderInput(popupWidth int) string {
	theme := p.styles.Theme()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	innerWidth := max(popupWidth-popupBorderPadding, popupMinInnerWidth)
	p.input.SetWidth(innerWidth)
	p.input.SetHeight(popupInputHeight)

	var buf strings.Builder
	buf.WriteString(titleStyle.Render(p.title) + "\n\n")
	buf.WriteString(p.input.View() + "\n\n")

	keys := p.keys
	hint := fmt.Sprintf("%s %s "+core.GlyphDot+" %s %s",
		keys.Submit.Help().Key, keys.Submit.Help().Desc,
		keys.Cancel.Help().Key, keys.Cancel.Help().Desc,
	)
	buf.WriteString(hintStyle.Render(hint))
	return buf.String()
}
