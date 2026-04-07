package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// ── Layout constants ────────────────────────────────────────────

const (
	// minTopBorderSpace is the minimum horizontal space needed to render
	// the title inside the top border before falling back to a plain line.
	minTopBorderSpace = 4

	// topBorderTitleInset is the number of horizontal characters consumed
	// by the gap between the border corner and the title on each side.
	topBorderTitleInset = 2
)

// ── Main view ───────────────────────────────────────────────────

// View renders the main application view for ihj.
func (m AppModel) View() tea.View {
	if !m.ready {
		view := tea.NewView("\n  Loading...")
		view.AltScreen = true
		view.MouseMode = tea.MouseModeCellMotion
		return view
	}

	screen := m.renderScreen()

	if m.popup.Active() {
		screen = m.overlayPopup(screen)
	}
	if m.showHelp {
		screen = m.overlayHelp(screen)
	}
	screen = m.overlayToast(screen)

	view := tea.NewView(screen)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

// ── Screen composition ──────────────────────────────────────────

func (m AppModel) renderScreen() string {
	theme := m.styles.Theme()

	detailBox := m.renderDetailBox(theme)
	footer := m.renderFooter(m.innerW)
	divider := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render(strings.Repeat(core.GlyphHorizLine, m.innerW-detailBorderH))

	body := m.composeBody(detailBox, divider, footer)

	titleContent := fmt.Sprintf(" %s "+core.GlyphVertLine+" %s (%s) ",
		m.ws.Name, strings.ToUpper(m.filter), m.cacheAgeString())

	outerBorder := lipgloss.RoundedBorder()
	outerWidth := m.width - outerBorderH
	topBorder := m.buildTopBorder(outerWidth, outerBorder, titleContent)
	inner := lipgloss.NewStyle().
		Border(outerBorder).
		BorderForeground(theme.Muted).
		Padding(0, 2).
		PaddingTop(1).
		Width(outerWidth).
		BorderTop(false).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Render(body)

	return lipgloss.JoinVertical(lipgloss.Left, topBorder, inner)
}

func (m AppModel) renderDetailBox(theme *terminal.Theme) string {
	detailContent := m.detail.View()

	borderColor := theme.Muted
	if m.view >= ViewDetail {
		borderColor = theme.Accent
	}

	if m.view >= ViewDetail && m.detail.CanGoBack() {
		detailContent += "\n" + m.renderBreadcrumbBar()
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 2).
		Width(m.innerW - detailBorderH).
		Height(m.detailContentH).
		MaxHeight(m.detailTotalH).
		Render(detailContent)
}

func (m AppModel) composeBody(detailBox, divider, footer string) string {
	hasFooter := footer != ""

	if m.view == ViewFullscreen {
		parts := []string{detailBox}
		if hasFooter {
			if m.showHelpBar {
				parts = append(parts, divider)
			}
			parts = append(parts, footer)
		}
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	parts := []string{
		detailBox,
		m.list.SearchBarView(),
		divider,
		m.list.View(),
	}
	if hasFooter {
		if m.showHelpBar {
			parts = append(parts, divider)
		}
		parts = append(parts, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ── Top border ──────────────────────────────────────────────────

func (m *AppModel) buildTopBorder(width int, border lipgloss.Border, title string) string {
	theme := m.styles.Theme()
	horizStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	titleStyled := m.styles.StatusBarKey.Render(title)
	titleWidth := lipgloss.Width(titleStyled)

	topLeft := horizStyle.Render(border.TopLeft)
	topRight := horizStyle.Render(border.TopRight)
	horizChar := border.Top

	available := width - titleWidth
	if available < minTopBorderSpace {
		return horizStyle.Render(strings.Repeat(horizChar, max(0, width+topBorderTitleInset)))
	}

	leftSegment := max(1, available/2-1)
	rightSegment := max(1, available-leftSegment-topBorderTitleInset)

	return topLeft +
		horizStyle.Render(strings.Repeat(horizChar, leftSegment)) +
		titleStyled +
		horizStyle.Render(strings.Repeat(horizChar, rightSegment)) +
		topRight
}

// ── Footer ──────────────────────────────────────────────────────

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

// ── Breadcrumb ──────────────────────────────────────────────────

// renderBreadcrumbBar renders the pinned breadcrumb line for the detail pane.
// Shows the navigation path (ancestor → … → current) with contextual key hints.
func (m *AppModel) renderBreadcrumbBar() string {
	issue := m.detail.Issue()
	if issue == nil || !m.detail.CanGoBack() {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Faint(true)
	separator := dimStyle.Render(" " + core.GlyphArrow + " ")

	breadcrumbPath := m.detail.Breadcrumb()
	segments := strings.Split(breadcrumbPath, " "+core.GlyphArrow+" ")

	styledSegments := make([]string, 0, len(segments))
	for idx, segment := range segments {
		if idx == len(segments)-1 {
			styledSegments = append(styledSegments, lipgloss.NewStyle().Bold(true).Render(segment))
		} else {
			styledSegments = append(styledSegments, dimStyle.Render(segment))
		}
	}

	breadcrumb := strings.Join(styledSegments, separator)
	hint := dimStyle.Render("  " + core.GlyphBackspace + " " + core.GlyphEscape)
	return breadcrumb + hint
}
