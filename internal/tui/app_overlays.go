package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// CompositeOverlay composites a rendered overlay onto the base screen at
// a given position using the lipgloss v2 Compositor. The base sits at
// Z=0; the overlay at (left, top, Z=1) so it always draws on top.
//
// Guarantees:
//   - An empty overlay returns the base string unchanged.
//   - Cells under the overlay's bounding box show the overlay's glyphs.
//   - Cells outside that box show the base's glyphs.
func CompositeOverlay(base, overlay string, top, left int) string {
	if overlay == "" {
		return base
	}
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(overlay).X(left).Y(top).Z(1),
	).Render()
}

func (m *AppModel) overlayPopup(base string) string {
	popup := m.popup.View()
	if popup == "" {
		return base
	}
	popupLines := strings.Split(popup, "\n")
	boxH := len(popupLines)
	boxW := lipgloss.Width(popupLines[0])
	top := max(0, (m.height-boxH)/2)
	left := max(0, (m.width-boxW)/2)
	return CompositeOverlay(base, popup, top, left)
}

// overlayHelp renders a WhichKey-style key binding panel at the bottom right.
func (m *AppModel) overlayHelp(base string) string {
	theme := terminal.DefaultTheme()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	descStyle := lipgloss.NewStyle().Foreground(theme.Text)
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	type group struct {
		name     string
		bindings []key.Binding
	}

	groups := []group{
		{"Navigation", []key.Binding{m.keys.Up, m.keys.Down, m.keys.Home, m.keys.End, m.keys.PageUp, m.keys.PageDn}},
		{"Detail", []key.Binding{m.keys.DetailUp, m.keys.DetailDown, m.keys.Focus, m.keys.Tab}},
		{"Actions", m.keys.ActionBindings()},
		{"General", []key.Binding{m.keys.Cancel, m.keys.Quit}},
	}

	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(groupStyle.Render(g.name) + "\n")
		for _, bind := range g.bindings {
			if !bind.Enabled() {
				continue
			}
			h := bind.Help()
			if h.Key == "" {
				continue
			}
			b.WriteString("  " + keyStyle.Render(h.Key) + " " + descStyle.Render(h.Desc) + "\n")
		}
	}

	hint := m.keys.Help.Help().Key + " close"
	b.WriteString("\n" + hintStyle.Render(hint))

	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.Muted).
		Padding(0, 2)

	box := boxStyle.Render(b.String())
	boxLines := strings.Split(box, "\n")
	boxH := len(boxLines)
	boxW := lipgloss.Width(boxLines[0])

	// Position: bottom right, right edge aligned with the detail view border.
	// Outer chrome: 1 border + 2 padding on each side = 3 per side, so 6 total inset.
	top := max(0, m.height-boxH-3)
	left := max(0, m.width-boxW-5)

	return CompositeOverlay(base, box, top, left)
}

// overlayToast composites a floating notification in the bottom right corner.
func (m *AppModel) overlayToast(base string) string {
	if m.notify == "" && m.loading == "" {
		return base
	}

	theme := terminal.DefaultTheme()

	// Determine state and colors.
	msg := m.notify
	icon := core.GlyphCircle
	color := theme.Accent

	if m.loading != "" {
		msg = m.loading
		icon = core.GlyphCycleArrow
		color = theme.Warning
	}

	// Render the sleek toast box.
	toastStr := lipgloss.NewStyle().Foreground(color).Render(icon) + " " + msg
	toast := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 1).
		Render(toastStr)

	toastLines := strings.Split(toast, "\n")
	toastH := len(toastLines)
	toastW := lipgloss.Width(toastLines[0])

	// Position: bottom right, pinned just inside the outer border padding.
	top := m.height - toastH - 3
	left := m.width - toastW - 4

	if top < 0 || left < 0 {
		return base
	}

	return CompositeOverlay(base, toast, top, left)
}
