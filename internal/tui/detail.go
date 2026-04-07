package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// DetailModel is the detail pane (top of screen).
type DetailModel struct {
	issue    *core.WorkItem
	viewport viewport.Model
	styles   *terminal.Styles
	keys     terminal.KeyMap
	ws       *core.Workspace // workspace for type-specific field lookups
	width    int
	height   int

	// Navigation — allows drilling into child issues and back.
	history  []*core.WorkItem          // Stack of previously viewed issues.
	registry map[string]*core.WorkItem // Full issue registry for child lookup.

	// Sorted children for the current issue (for hint-key navigation).
	sortedChildren []*core.WorkItem
	// Available single-key hints for child navigation (computed from keymap).
	hintKeys []rune
}

// NewDetailModel creates the detail pane.
func NewDetailModel(styles *terminal.Styles, registry map[string]*core.WorkItem, ws *core.Workspace, keys terminal.KeyMap) DetailModel {
	return DetailModel{
		viewport: viewport.New(),
		styles:   styles,
		keys:     keys,
		registry: registry,
		ws:       ws,
		hintKeys: keys.HintKeys(),
	}
}

// SetIssue updates the displayed issue and re-renders content.
// Clears the navigation history (fresh selection from the list).
func (m *DetailModel) SetIssue(issue *core.WorkItem) {
	if issue == nil {
		return
	}
	sameIssue := m.issue != nil && m.issue.ID == issue.ID && len(m.history) == 0
	m.issue = issue
	if !sameIssue {
		m.history = nil // Clear history — this is a new list selection.
	}
	m.rebuildContent()
	if !sameIssue {
		m.viewport.GotoTop()
	}
}

// UpdateRegistry replaces the issue registry (e.g. after a data reload)
// and refreshes the current issue and navigation history from the new data.
// This ensures the detail pane shows updated fields even when navigated
// into a child issue (where syncDetail would otherwise skip the update).
func (m *DetailModel) UpdateRegistry(reg map[string]*core.WorkItem) {
	m.registry = reg

	// Refresh history entries from the new registry.
	for idx, historyIssue := range m.history {
		if fresh, ok := reg[historyIssue.ID]; ok {
			m.history[idx] = fresh
		}
	}

	// Refresh the currently displayed issue.
	if m.issue != nil {
		if fresh, ok := reg[m.issue.ID]; ok {
			m.issue = fresh
			m.rebuildContent()
		}
	}
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

// ClearHistory discards the navigation history without changing the current issue.
func (m *DetailModel) ClearHistory() {
	m.history = nil
}

// ChildIndexForKey returns the child index for a hint key press, or -1 if not valid.
func (m *DetailModel) ChildIndexForKey(r rune) int {
	for i, hint := range m.hintKeys {
		if i >= len(m.sortedChildren) {
			break
		}
		if hint == r {
			return i
		}
	}
	return -1
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
	return strings.Join(parts, " "+core.GlyphArrow+" ")
}

// ScrollUp scrolls the detail viewport up.
func (m *DetailModel) ScrollUp(lines int) {
	m.viewport.ScrollUp(lines)
}

// ScrollDown scrolls the detail viewport down.
func (m *DetailModel) ScrollDown(lines int) {
	m.viewport.ScrollDown(lines)
}

// ScrollToTop scrolls the detail viewport to the top.
func (m *DetailModel) ScrollToTop() {
	m.viewport.GotoTop()
}

// ScrollToBottom scrolls the detail viewport to the bottom.
func (m *DetailModel) ScrollToBottom() {
	m.viewport.GotoBottom()
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

// issueFieldDefs returns the FieldDefs for the current issue's type.
// Falls back to the workspace-wide union if the type is unknown.
func (m *DetailModel) issueFieldDefs() core.FieldDefs {
	if m.issue != nil {
		if tc := m.ws.TypeByName(m.issue.Type); tc != nil && len(tc.Fields) > 0 {
			return tc.Fields
		}
	}
	return m.ws.AllFieldDefs()
}

// visibleFieldsByRole returns FieldDefs for the given role,
// scoped to the current issue's type.
func (m *DetailModel) visibleFieldsByRole(role core.FieldRole) core.FieldDefs {
	return m.issueFieldDefs().ByRole(role)
}

// urgencyFieldKey returns the key of the primary urgency field, or "" if none.
func (m *DetailModel) urgencyFieldKey() string {
	if def := m.issueFieldDefs().ByRole(core.RoleUrgency).Primary(); def != nil {
		return def.Key
	}
	return ""
}
