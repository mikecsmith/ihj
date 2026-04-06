package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// View renders the main application view for ihj
func (m AppModel) View() tea.View {
	if !m.ready {
		v := tea.NewView("\n  Loading...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	s := m.styles
	theme := terminal.DefaultTheme()
	outerBorderH := 2
	detailBorderH := 2

	detailContent := m.detail.View()

	// Border color indicates pane focus.
	detailBorderColor := theme.Muted
	if m.view >= ViewDetail {
		detailBorderColor = theme.Accent
	}

	// Breadcrumb bar: pinned at bottom of detail when navigated into children.
	if (m.view >= ViewDetail) && m.detail.CanGoBack() {
		detailContent += "\n" + m.renderBreadcrumbBar()
	}

	detailBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(detailBorderColor).
		Padding(0, 2).
		Width(m.innerW - detailBorderH).
		Height(m.detailContentH).
		MaxHeight(m.detailTotalH).
		Render(detailContent)

	var body string
	divider := lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat(core.GlyphHorizLine, m.innerW-detailBorderH))
	footer := m.renderFooter(m.innerW)
	hasBottomBar := footer != ""

	if m.view == ViewFullscreen {
		parts := []string{detailBox}
		if hasBottomBar {
			if m.showHelpBar {
				parts = append(parts, divider)
			}
			parts = append(parts, footer)
		}
		body = lipgloss.JoinVertical(lipgloss.Left, parts...)
	} else {
		parts := []string{
			detailBox,
			m.list.SearchBarView(),
			divider,
			m.list.View(),
		}
		if hasBottomBar {
			if m.showHelpBar {
				parts = append(parts, divider)
			}
			parts = append(parts, footer)
		}
		body = lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	cacheAge := m.cacheAgeString()
	titleContent := fmt.Sprintf(" %s "+core.GlyphVertLine+" %s (%s) ",
		m.ws.Name, strings.ToUpper(m.filter), cacheAge)

	outerBorder := lipgloss.RoundedBorder()
	outerStyle := lipgloss.NewStyle().
		Border(outerBorder).
		BorderForeground(theme.Muted).
		Padding(0, 2).
		PaddingTop(1).
		Width(m.width - outerBorderH).
		BorderTop(false).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true)

	topBorder := m.buildTopBorder(m.width-outerBorderH, outerBorder, titleContent, s)
	inner := outerStyle.Render(body)

	screen := lipgloss.JoinVertical(lipgloss.Left, topBorder, inner)

	if m.popup.Active() {
		screen = m.overlayPopup(screen)
	}

	if m.showHelp {
		screen = m.overlayHelp(screen)
	}

	screen = m.overlayToast(screen)

	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *AppModel) buildTopBorder(width int, border lipgloss.Border, title string, s *terminal.Styles) string {
	theme := terminal.DefaultTheme()
	borderFg := theme.Muted
	horizStyle := lipgloss.NewStyle().Foreground(borderFg)

	titleStyled := s.StatusBarKey.Render(title)
	titleW := lipgloss.Width(titleStyled)

	tl := horizStyle.Render(border.TopLeft)
	tr := horizStyle.Render(border.TopRight)
	horiz := border.Top

	// Center the title in the top border line.
	available := width - titleW
	if available < 4 {
		return horizStyle.Render(strings.Repeat(horiz, max(0, width+2)))
	}

	leftSeg := max(1, available/2-1)
	rightSeg := max(1, available-leftSeg-2)

	return tl +
		horizStyle.Render(strings.Repeat(horiz, leftSeg)) +
		titleStyled +
		horizStyle.Render(strings.Repeat(horiz, rightSeg)) +
		tr
}

// renderFooter renders the bottom bar: key bindings (default mode),
// mode indicator + bindings (vim mode), or just the mode tag (vim with
// help bar hidden). Returns "" when there's nothing to show.
func (m *AppModel) renderFooter(width int) string {
	if m.vimMode {
		return m.renderVimFooter(width)
	}
	if m.showHelpBar {
		return m.help.ShortHelpView(m.keys.ShortHelp())
	}
	return ""
}

// renderBreadcrumbBar renders the pinned breadcrumb line for the detail pane.
// Shows the navigation path with contextual key hints.
func (m *AppModel) renderBreadcrumbBar() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	iss := m.detail.Issue()
	if iss == nil {
		return ""
	}

	if !m.detail.CanGoBack() {
		return ""
	}

	// Show full path: ancestor → ancestor → current  ⌫ ␛
	crumbParts := make([]string, 0, 4)
	bc := m.detail.Breadcrumb()
	ids := strings.Split(bc, " "+core.GlyphArrow+" ")
	for i, id := range ids {
		if i == len(ids)-1 {
			crumbParts = append(crumbParts, lipgloss.NewStyle().Bold(true).Render(id))
		} else {
			crumbParts = append(crumbParts, dimStyle.Render(id))
		}
	}
	sep := dimStyle.Render(" " + core.GlyphArrow + " ")
	breadcrumb := strings.Join(crumbParts, sep)
	hint := dimStyle.Render("  " + core.GlyphBackspace + " " + core.GlyphEscape)
	return breadcrumb + hint
}
