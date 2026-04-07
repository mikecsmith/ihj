package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// ── Layout constants ────────────────────────────────────────────

const (
	// helpOverlayInsetBottom is the row offset from the bottom edge for the help panel.
	helpOverlayInsetBottom = 3
	// helpOverlayInsetRight is the column offset from the right edge for the help panel.
	helpOverlayInsetRight = 5

	// toastInsetBottom is the row offset from the bottom edge for the toast notification.
	toastInsetBottom = 3
	// toastInsetRight is the column offset from the right edge for the toast notification.
	toastInsetRight = 4
)

// ── Compositor ──────────────────────────────────────────────────

// CompositeOverlay composites a rendered overlay onto the base screen at
// a given position using the lipgloss v2 Compositor. The base sits at
// Z=0; the overlay at (left, top, Z=1) so it always draws on top.
//
// Guarantees:
//   - An empty overlay returns the base string unchanged.
//   - Cells under the overlay's bounding box show the overlay's glyphs.
//   - Cells outside that box show the base's glyphs.
func CompositeOverlay(base, overlay string, offsetX, offsetY int) string {
	if overlay == "" {
		return base
	}
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(overlay).X(offsetX).Y(offsetY).Z(1),
	).Render()
}

// ── Popup overlay ───────────────────────────────────────────────

func (m *AppModel) overlayPopup(base string) string {
	popup := m.popup.View()
	if popup == "" {
		return base
	}
	popupLines := strings.Split(popup, "\n")
	boxHeight := len(popupLines)
	boxWidth := lipgloss.Width(popupLines[0])
	offsetY := max(0, (m.height-boxHeight)/2)
	offsetX := max(0, (m.width-boxWidth)/2)
	return CompositeOverlay(base, popup, offsetX, offsetY)
}

// ── Help overlay ────────────────────────────────────────────────

// overlayHelp renders a WhichKey-style key binding panel at the bottom right.
func (m *AppModel) overlayHelp(base string) string {
	theme := m.styles.Theme()

	helpBox := m.renderHelpBox(theme)
	helpLines := strings.Split(helpBox, "\n")
	boxHeight := len(helpLines)
	boxWidth := lipgloss.Width(helpLines[0])

	offsetY := max(0, m.height-boxHeight-helpOverlayInsetBottom)
	offsetX := max(0, m.width-boxWidth-helpOverlayInsetRight)

	return CompositeOverlay(base, helpBox, offsetX, offsetY)
}

func (m *AppModel) renderHelpBox(theme *terminal.Theme) string {
	keys := m.keys
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	descStyle := lipgloss.NewStyle().Foreground(theme.Text)
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	type bindingGroup struct {
		name     string
		bindings []key.Binding
	}

	groups := []bindingGroup{
		{"Navigation", []key.Binding{keys.Up, keys.Down, keys.Home, keys.End, keys.PageUp, keys.PageDn}},
		{"Detail", []key.Binding{keys.DetailUp, keys.DetailDown, keys.Focus, keys.Tab}},
		{"Actions", keys.ActionBindings()},
		{"General", []key.Binding{keys.Cancel, keys.Quit}},
	}

	var buf strings.Builder
	for idx, group := range groups {
		if idx > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(groupStyle.Render(group.name) + "\n")
		for _, binding := range group.bindings {
			if !binding.Enabled() {
				continue
			}
			help := binding.Help()
			if help.Key == "" {
				continue
			}
			buf.WriteString("  " + keyStyle.Render(help.Key) + " " + descStyle.Render(help.Desc) + "\n")
		}
	}

	closeHint := keys.Help.Help().Key + " close"
	buf.WriteString("\n" + hintStyle.Render(closeHint))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 2).
		Render(buf.String())
}

// ── Toast overlay ───────────────────────────────────────────────

// overlayToast composites a floating notification in the bottom right corner.
func (m *AppModel) overlayToast(base string) string {
	if m.notify == "" && m.loading == "" {
		return base
	}

	theme := m.styles.Theme()
	toast := m.renderToast(theme)

	toastLines := strings.Split(toast, "\n")
	toastHeight := len(toastLines)
	toastWidth := lipgloss.Width(toastLines[0])

	offsetY := m.height - toastHeight - toastInsetBottom
	offsetX := m.width - toastWidth - toastInsetRight

	if offsetY < 0 || offsetX < 0 {
		return base
	}

	return CompositeOverlay(base, toast, offsetX, offsetY)
}

func (m *AppModel) renderToast(theme *terminal.Theme) string {
	message := m.notify
	icon := core.GlyphCircle
	iconColor := theme.Accent

	if m.loading != "" {
		message = m.loading
		icon = core.GlyphCycleArrow
		iconColor = theme.Warning
	}

	content := lipgloss.NewStyle().Foreground(iconColor).Render(icon) + " " + message

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 1).
		Render(content)
}
