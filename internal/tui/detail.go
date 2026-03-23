package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// DetailMode determines what the detail pane is showing.
type DetailMode int

const (
	DetailBrowse  DetailMode = iota // Viewing issue details.
	DetailComment                   // Composing a comment.
)

// DetailModel is the preview pane (top of screen).
type DetailModel struct {
	issue    *jira.IssueView
	mode     DetailMode
	viewport viewport.Model
	input    textarea.Model
	styles   *Styles
	width    int
	height   int

	// Navigation — allows drilling into child issues and back.
	history  []*jira.IssueView          // Stack of previously viewed issues.
	registry map[string]*jira.IssueView // Full issue registry for child lookup.

	// Sorted children for the current issue (for number-key navigation).
	sortedChildren []*jira.IssueView
}

// NewDetailModel creates the detail pane.
func NewDetailModel(styles *Styles, registry map[string]*jira.IssueView) DetailModel {
	vp := viewport.New()

	ta := textarea.New()
	ta.Placeholder = "Type here..."
	ta.ShowLineNumbers = false
	ta.SetWidth(40)
	ta.SetHeight(5)
	ta.CharLimit = 4000

	return DetailModel{
		viewport: vp,
		input:    ta,
		styles:   styles,
		mode:     DetailBrowse,
		registry: registry,
	}
}

// SetIssue updates the displayed issue and re-renders content.
// Clears the navigation history (fresh selection from the list).
func (m *DetailModel) SetIssue(issue *jira.IssueView) {
	if issue == nil || (m.issue != nil && m.issue.Key == issue.Key && m.mode == DetailBrowse && len(m.history) == 0) {
		return
	}
	m.issue = issue
	m.mode = DetailBrowse
	m.history = nil // Clear history — this is a new list selection.
	m.rebuildContent()
	m.viewport.GotoTop()
}

// NavigateTo pushes the current issue onto the history stack and shows a new one.
func (m *DetailModel) NavigateTo(issue *jira.IssueView) {
	if issue == nil || m.issue == nil {
		return
	}
	m.history = append(m.history, m.issue)
	m.issue = issue
	m.mode = DetailBrowse
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
		parts = append(parts, h.Key)
	}
	if m.issue != nil {
		parts = append(parts, m.issue.Key)
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
	m.input.SetWidth(w - 4)
	m.input.SetHeight(max(3, h-4))
	if m.issue != nil {
		m.rebuildContent()
	}
}

// StartComment enters comment composition mode.
func (m *DetailModel) StartComment() {
	m.mode = DetailComment
	m.input.Placeholder = "Write a comment... (Ctrl+S to send, Esc to cancel)"
	m.input.Reset()
	m.input.Focus()
}

// InputValue returns the current text input value and resets mode.
func (m *DetailModel) InputValue() string {
	val := strings.TrimSpace(m.input.Value())
	m.mode = DetailBrowse
	m.rebuildContent()
	return val
}

// CancelInput returns to browse mode without capturing input.
func (m *DetailModel) CancelInput() {
	m.mode = DetailBrowse
	m.input.Blur()
	m.rebuildContent()
}

// Mode returns the current detail mode.
func (m *DetailModel) Mode() DetailMode { return m.mode }

// Issue returns the currently displayed issue.
func (m *DetailModel) Issue() *jira.IssueView { return m.issue }

// --- Bubble Tea interface ---

func (m DetailModel) Init() tea.Cmd { return nil }

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd
	if m.mode == DetailComment {
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DetailModel) View() string {
	if m.issue == nil {
		return m.renderEmpty()
	}

	switch m.mode {
	case DetailComment:
		return m.renderInputMode("Comment on " + m.issue.Key)
	default:
		return m.viewport.View()
	}
}

// --- Content rendering ---

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

	// Identity bar: TEAM ❯ KEY ❯ TYPE ❯ STATUS ❯ PRIORITY
	// Nerd font icons and ❯ separator matching the original Python TUI.
	bc := lipgloss.NewStyle().Faint(true).Render(" ❯ ")

	typeColor := s.TypeColor(iss.Type)
	statusIcon, statusColor := s.StatusStyle(iss.Status)
	typeIcon := s.TypeIcon(iss.Type)

	identity := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("  "+m.teamName()) + bc +
		lipgloss.NewStyle().Bold(true).Render(iss.Key) + bc +
		lipgloss.NewStyle().Foreground(typeColor).Render(typeIcon+" "+strings.ToUpper(iss.Type)) + bc +
		lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon+" "+strings.ToUpper(iss.Status)) + bc +
		s.PriorityIcon(iss.Priority) + " " + strings.ToUpper(iss.Priority)

	b.WriteString(identity + "\n")

	// Metadata grid — two-column layout with nerd font icons.
	pad := func(text string, width int) string {
		if len(text) > width {
			return text[:width-3] + "..."
		}
		return text + strings.Repeat(" ", max(0, width-len(text)))
	}

	assignee := pad(iss.Assignee, 22)
	reporter := pad(iss.Reporter, 22)

	// Nerd font icons: 󰀄 account, 󰁥 calendar, 󰔚 update
	b.WriteString(s.LabelAssignee.Render(" Assignee:   ") + s.DetailValue.Render(assignee) +
		s.LabelCreated.Render(" Created: ") + s.DetailValue.Render(iss.Created) + "\n")
	b.WriteString(s.LabelReporter.Render(" Reporter:   ") + s.DetailValue.Render(reporter) +
		s.LabelUpdated.Render(" Updated: ") + s.DetailValue.Render(iss.Updated) + "\n")

	if iss.Components != "" {
		b.WriteString(s.LabelComponents.Render(" Components: ") + s.DetailValue.Render(iss.Components) + "\n")
	}
	if iss.Labels != "" {
		b.WriteString(s.LabelLabels.Render(" Labels:     ") + s.DetailValue.Render(iss.Labels) + "\n")
	}
	if iss.ParentKey != "" {
		b.WriteString(s.LabelParent.Render(" Parent:     ") + " " +
			lipgloss.NewStyle().Bold(true).Render(iss.ParentKey) + "\n")
	}

	// Back navigation hint.
	if len(m.history) > 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  ← Esc to go back") + "\n")
	}

	// Divider + Summary.
	divider := s.DetailDivider.Render(strings.Repeat("─", min(contentWidth, 64)))
	b.WriteString("\n" + divider + "\n")
	b.WriteString(s.DetailHeader.Render(strings.ToUpper(iss.Summary)) + "\n\n")

	// Description (rendered from AST).
	if iss.Desc != nil {
		desc := document.RenderANSI(iss.Desc, document.ANSIConfig{
			Styles:    s.Doc,
			WrapWidth: wrapWidth,
		})
		b.WriteString(desc)
	} else {
		b.WriteString(lipgloss.NewStyle().Faint(true).Italic(true).Render("No description provided.") + "\n")
	}

	// Child issues (sorted by key for stable ordering).
	m.sortedChildren = nil
	if len(iss.Children) > 0 {
		b.WriteString("\n" + divider + "\n\n")
		b.WriteString(s.ChildSection.Render("󰙔 CHILD ISSUES") + "\n\n")

		sortedChildren := make([]*jira.IssueView, 0, len(iss.Children))
		for _, child := range iss.Children {
			sortedChildren = append(sortedChildren, child)
		}
		sort.Slice(sortedChildren, func(i, j int) bool {
			return sortedChildren[i].Key < sortedChildren[j].Key
		})
		m.sortedChildren = sortedChildren

		for idx, child := range sortedChildren {
			icon, clr := s.StatusStyle(child.Status)
			statusStyle := lipgloss.NewStyle().Foreground(clr)
			typeClr := s.TypeColor(child.Type)

			childStatus := child.Status
			if len(childStatus) > 14 {
				childStatus = childStatus[:14]
			}

			// Number hint for keyboard navigation.
			numHint := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("[%d]", idx+1))

			line := "  " + s.TreeGlyph.Render("↳") + " " +
				lipgloss.NewStyle().Foreground(typeClr).Bold(true).Render(fmt.Sprintf("%-11s", child.Key)) + " " +
				statusStyle.Render(fmt.Sprintf("%s %-14s", icon, childStatus)) + " " +
				child.Summary + " " + numHint
			b.WriteString(line + "\n")
		}
	}

	// Comments.
	if len(iss.Comments) > 0 {
		b.WriteString("\n" + divider + "\n\n")
		b.WriteString(s.CommentSection.Render("󱠁 LATEST COMMENTS") + "\n\n")

		for _, c := range iss.Comments {
			header := s.CommentAuthor.Render(c.Author) + "  " +
				s.CommentDate.Render("• "+c.Created)
			b.WriteString(header + "\n")
			if c.Body != nil {
				body := document.RenderANSI(c.Body, document.ANSIConfig{
					Styles:    s.Doc,
					WrapWidth: wrapWidth,
				})
				b.WriteString(body + "\n")
			}
		}
	}

	m.viewport.SetContent(b.String())
}

func (m *DetailModel) teamName() string {
	// Extract project key from the issue key (e.g., "DEMO-1" → "DEMO").
	if m.issue != nil {
		if idx := strings.Index(m.issue.Key, "-"); idx > 0 {
			return m.issue.Key[:idx]
		}
	}
	return "TEAM"
}

func (m *DetailModel) renderEmpty() string {
	msg := lipgloss.NewStyle().Faint(true).Render("Select an issue to view details")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}

func (m *DetailModel) renderInputMode(title string) string {
	var b strings.Builder
	b.WriteString(m.styles.InputLabel.Render(title) + "\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.ActionKey.Render("Ctrl+S") + " " + m.styles.ActionDesc.Render("Send") +
		m.styles.ActionDesc.Render("  │  ") +
		m.styles.ActionKey.Render("Esc") + " " + m.styles.ActionDesc.Render("Cancel"))
	return b.String()
}
