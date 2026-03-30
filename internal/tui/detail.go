package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// DetailModel is the preview pane (top of screen).
type DetailModel struct {
	issue    *core.WorkItem
	viewport viewport.Model
	styles   *terminal.Styles
	keys     terminal.KeyMap
	teamName string
	width    int
	height   int

	// Navigation — allows drilling into child issues and back.
	history  []*core.WorkItem          // Stack of previously viewed issues.
	registry map[string]*core.WorkItem // Full issue registry for child lookup.

	// Sorted children for the current issue (for number-key navigation).
	sortedChildren []*core.WorkItem
}

// NewDetailModel creates the detail pane.
func NewDetailModel(styles *terminal.Styles, registry map[string]*core.WorkItem, teamName string, keys terminal.KeyMap) DetailModel {
	return DetailModel{
		viewport: viewport.New(),
		styles:   styles,
		keys:     keys,
		registry: registry,
		teamName: teamName,
	}
}

// SetIssue updates the displayed issue and re-renders content.
// Clears the navigation history (fresh selection from the list).
func (m *DetailModel) SetIssue(issue *core.WorkItem) {
	if issue == nil || (m.issue != nil && m.issue.ID == issue.ID && len(m.history) == 0) {
		return
	}
	m.issue = issue
	m.history = nil // Clear history — this is a new list selection.
	m.rebuildContent()
	m.viewport.GotoTop()
}

// NavigateTo pushes the current issue onto the history stack and shows a new one.
func (m *DetailModel) NavigateTo(issue *core.WorkItem) {
	if issue == nil || m.issue == nil {
		return
	}
	m.history = append(m.history, m.issue)
	m.issue = issue
	m.rebuildContent()
	m.viewport.GotoTop()
}

// NavigateToChild navigates to the nth child (0-indexed). Returns true if successful.
func (m *DetailModel) NavigateToChild(idx int) bool {
	if idx < 0 || idx >= len(m.sortedChildren) {
		return false
	}
	m.NavigateTo(m.sortedChildren[idx])
	return true
}

// GoBack pops the history stack to return to the previous issue.
func (m *DetailModel) GoBack() {
	if len(m.history) == 0 {
		return
	}
	m.issue = m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.rebuildContent()
	m.viewport.GotoTop()
}

// CanGoBack returns true if there's history to pop.
func (m *DetailModel) CanGoBack() bool {
	return len(m.history) > 0
}

// Breadcrumb returns a display string showing the navigation path.
func (m *DetailModel) Breadcrumb() string {
	if len(m.history) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m.history)+1)
	for _, h := range m.history {
		parts = append(parts, h.ID)
	}
	if m.issue != nil {
		parts = append(parts, m.issue.ID)
	}
	return strings.Join(parts, " → ")
}

// ScrollUp scrolls the preview viewport up.
func (m *DetailModel) ScrollUp(lines int) {
	m.viewport.ScrollUp(lines)
}

// ScrollDown scrolls the preview viewport down.
func (m *DetailModel) ScrollDown(lines int) {
	m.viewport.ScrollDown(lines)
}

// SetSize updates dimensions. Only rebuilds content if dimensions changed.
func (m *DetailModel) SetSize(w, h int) {
	if m.width == w && m.height == h {
		return
	}
	m.width = w
	m.height = h
	m.viewport.SetWidth(w)
	m.viewport.SetHeight(h)
	if m.issue != nil {
		m.rebuildContent()
	}
}

// Issue returns the currently displayed issue.
func (m *DetailModel) Issue() *core.WorkItem { return m.issue }

func (m DetailModel) Init() tea.Cmd { return nil }

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DetailModel) View() string {
	if m.issue == nil {
		return m.renderEmpty()
	}
	return m.viewport.View()
}

func (m *DetailModel) rebuildContent() {
	if m.issue == nil {
		return
	}

	s := m.styles
	iss := m.issue
	contentWidth := m.width - 2
	if contentWidth < 20 {
		contentWidth = 60
	}
	wrapWidth := min(90, contentWidth)

	var b strings.Builder

	var parts []string

	if m.teamName != "" {
		teamStr := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(" " + strings.ToUpper(m.teamName))
		parts = append(parts, teamStr)
	}

	keyStr := lipgloss.NewStyle().Bold(true).Render(iss.ID)
	parts = append(parts, keyStr)

	typeColor := s.TypeColor(iss.Type)
	typeStr := lipgloss.NewStyle().Foreground(typeColor).Render(" " + strings.ToUpper(iss.Type))
	parts = append(parts, typeStr)

	statusIcon, statusColor := s.StatusStyle(iss.Status)
	statusStr := lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon + " " + strings.ToUpper(iss.Status))
	parts = append(parts, statusStr)

	priority := iss.StringField("priority")
	prioStr := s.PriorityIcon(priority) + " " + strings.ToUpper(priority)
	parts = append(parts, prioStr)

	// Cleanly join all present parts with the faint chevron
	bc := lipgloss.NewStyle().Faint(true).Render(" ❯ ")
	identLine := strings.Join(parts, bc)

	b.WriteString(identLine + "\n")

	pad := func(text string, width int) string {
		w := lipgloss.Width(text)
		if w > width {
			runes := []rune(text)
			if len(runes) > width-3 {
				return string(runes[:width-3]) + "..."
			}
		}
		return text + strings.Repeat(" ", max(0, width-w))
	}

	assigneeVal := iss.DisplayStringField("assignee")
	if assigneeVal == "" {
		assigneeVal = "—"
	}
	assignee := pad(assigneeVal, 22)
	reporterVal := iss.DisplayStringField("reporter")
	if reporterVal == "" {
		reporterVal = "—"
	}
	reporter := pad(reporterVal, 22)

	// Row 1: Assignee (Cyan) & Created (Dim)
	lblAssignee := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Info).Render(" Assignee:   ")
	lblCreated := lipgloss.NewStyle().Faint(true).Render(" Created: ")
	b.WriteString(lblAssignee + s.DetailValue.Render(assignee) + " " + lblCreated + s.DetailValue.Render(iss.StringField("created")) + "\n")

	// Row 2: Reporter (Dim) & Updated (Dim)
	lblReporter := lipgloss.NewStyle().Faint(true).Render(" Reporter:   ")
	lblUpdated := lipgloss.NewStyle().Faint(true).Render(" Updated: ")
	b.WriteString(lblReporter + s.DetailValue.Render(reporter) + " " + lblUpdated + s.DetailValue.Render(iss.StringField("updated")) + "\n")

	// Components (Blue)
	if components := iss.StringField("components"); components != "" {
		lblComponents := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Accent).Render(" Components: ")
		b.WriteString(lblComponents + s.DetailValue.Render(components) + "\n")
	}

	// Labels (Magenta)
	if labels := iss.StringField("labels"); labels != "" {
		lblLabels := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().TypeEpic).Render(" Labels:     ")
		b.WriteString(lblLabels + s.DetailValue.Render(labels) + "\n")
	}

	// Parent (Dim)
	if iss.ParentID != "" {
		lblParent := lipgloss.NewStyle().Faint(true).Render("󰄶 Parent:     ")
		b.WriteString(lblParent + lipgloss.NewStyle().Bold(true).Render(iss.ParentID) + "\n")
	}

	// Back navigation hint
	if len(m.history) > 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  ← Esc to go back") + "\n")
	}

	divider := s.DetailDivider.Render(strings.Repeat("─", min(contentWidth, 64)))
	b.WriteString("\n" + divider + "\n")
	b.WriteString(s.DetailHeader.Render(strings.ToUpper(iss.Summary)) + "\n\n")

	// Description (rendered from AST).
	noDesc := lipgloss.NewStyle().Faint(true).Italic(true).Render("No description.") + "\n"
	if iss.Description != nil {
		desc := strings.TrimSpace(document.RenderANSI(iss.Description, document.ANSIConfig{
			WrapWidth: wrapWidth,
			Style:     s.ContentStyle,
		}))
		if desc != "" {
			b.WriteString(desc + "\n")
		} else {
			b.WriteString(noDesc)
		}
	} else {
		b.WriteString(noDesc)
	}

	// Child issues (sorted by key for stable ordering).
	m.sortedChildren = nil
	if len(iss.Children) > 0 {
		b.WriteString("\n" + divider + "\n")
		b.WriteString(s.ChildSection.Render("󰙔 CHILD ISSUES") + "\n")

		sortedChildren := make([]*core.WorkItem, len(iss.Children))
		copy(sortedChildren, iss.Children)
		sort.Slice(sortedChildren, func(i, j int) bool {
			return sortedChildren[i].ID < sortedChildren[j].ID
		})
		m.sortedChildren = sortedChildren

		// Measure columns to pad relative to the longest value.
		maxID, maxType, maxStatus := 0, 0, 0
		for _, child := range sortedChildren {
			if w := len([]rune(child.ID)); w > maxID {
				maxID = w
			}
			t := child.Type
			if len(t) > 10 {
				t = t[:10]
			}
			if w := len([]rune(t)); w > maxType {
				maxType = w
			}
			st := child.Status
			if len(st) > 14 {
				st = st[:14]
			}
			if w := len([]rune(st)); w > maxStatus {
				maxStatus = w
			}
		}

		idFmt := fmt.Sprintf("%%-%ds", maxID+1)
		typeFmt := fmt.Sprintf("%%-%ds", maxType+1)
		// Status column includes the icon char + space before the name.
		statusFmt := fmt.Sprintf("%%s %%-%ds", maxStatus+1)

		for idx, child := range sortedChildren {
			icon, clr := s.StatusStyle(child.Status)
			statusStyle := lipgloss.NewStyle().Foreground(clr)
			typeClr := s.TypeColor(child.Type)

			childStatus := child.Status
			if len(childStatus) > 14 {
				childStatus = childStatus[:14]
			}

			numHint := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("[%d]", idx+1))

			childType := child.Type
			if len(childType) > 10 {
				childType = childType[:10]
			}

			prio := s.PriorityIcon(child.StringField("priority"))

			line := "  " + s.TreeGlyph.Render("↳") + " " +
				lipgloss.NewStyle().Foreground(typeClr).Bold(true).Render(fmt.Sprintf(idFmt, child.ID)) + " " +
				prio + " " +
				lipgloss.NewStyle().Foreground(typeClr).Render(fmt.Sprintf(typeFmt, childType)) + " " +
				statusStyle.Render(fmt.Sprintf(statusFmt, icon, childStatus)) + " " +
				child.Summary + " " + numHint
			b.WriteString(line + "\n")
		}
	}

	// Comments.
	if len(iss.Comments) > 0 {
		b.WriteString("\n" + divider + "\n")
		b.WriteString(s.CommentSection.Render("󱠁 LATEST COMMENTS") + "\n\n")

		commentSep := lipgloss.NewStyle().Faint(true).Render("───")
		for i, c := range iss.Comments {
			if i > 0 {
				b.WriteString("\n" + commentSep + "\n")
			}
			header := s.CommentAuthor.Render(c.Author) + "  " +
				s.CommentDate.Render("• "+c.Created)
			b.WriteString(header + "\n")
			if c.Body != nil {
				body := document.RenderANSI(c.Body, document.ANSIConfig{
					WrapWidth: wrapWidth,
					Style:     s.ContentStyle,
				})
				b.WriteString(strings.Trim(body, "\n") + "\n")
			}
		}
	}

	m.viewport.SetContent(b.String())
}

func (m *DetailModel) renderEmpty() string {
	msg := lipgloss.NewStyle().Faint(true).Render("Select an issue to view details")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}
