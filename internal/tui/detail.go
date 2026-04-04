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

// DetailModel is the detail pane (top of screen).
type DetailModel struct {
	issue     *core.WorkItem
	viewport  viewport.Model
	styles    *terminal.Styles
	keys      terminal.KeyMap
	fieldDefs core.FieldDefs
	teamName  string
	width     int
	height    int

	// Navigation — allows drilling into child issues and back.
	history  []*core.WorkItem          // Stack of previously viewed issues.
	registry map[string]*core.WorkItem // Full issue registry for child lookup.

	// Sorted children for the current issue (for hint-key navigation).
	sortedChildren []*core.WorkItem
	// Available single-key hints for child navigation (computed from keymap).
	hintKeys []rune
}

// NewDetailModel creates the detail pane.
func NewDetailModel(styles *terminal.Styles, registry map[string]*core.WorkItem, teamName string, keys terminal.KeyMap, fieldDefs core.FieldDefs) DetailModel {
	return DetailModel{
		viewport:  viewport.New(),
		styles:    styles,
		keys:      keys,
		registry:  registry,
		teamName:  teamName,
		fieldDefs: fieldDefs,
		hintKeys:  keys.HintKeys(),
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
// without resetting the view state. The next syncDetail call will
// re-render the current issue with fresh data.
func (m *DetailModel) UpdateRegistry(reg map[string]*core.WorkItem) {
	m.registry = reg
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

// visibleFieldsByRole returns non-WriteOnly FieldDefs for the given role,
// preserving provider-declared order.
func (m *DetailModel) visibleFieldsByRole(role core.FieldRole) core.FieldDefs {
	var out core.FieldDefs
	for _, def := range m.fieldDefs.ByRole(role) {
		if !def.WriteOnly {
			out = append(out, def)
		}
	}
	return out
}

// urgencyFieldKey returns the key of the primary urgency field, or "" if none.
func (m *DetailModel) urgencyFieldKey() string {
	if def := m.fieldDefs.ByRole(core.RoleUrgency).Primary(); def != nil {
		return def.Key
	}
	return ""
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
		teamStr := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(core.IconTeam + strings.ToUpper(m.teamName))
		parts = append(parts, teamStr)
	}

	keyStr := lipgloss.NewStyle().Bold(true).Render(iss.ID)
	parts = append(parts, keyStr)

	typeColor := s.TypeColor(iss.Type)
	typeStr := lipgloss.NewStyle().Foreground(typeColor).Render(core.IconType + strings.ToUpper(iss.Type))
	parts = append(parts, typeStr)

	statusIcon, statusColor := s.StatusStyle(iss.Status)
	statusStr := lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon + " " + strings.ToUpper(iss.Status))
	parts = append(parts, statusStr)

	if urgKey := m.urgencyFieldKey(); urgKey != "" {
		priority := iss.StringField(urgKey)
		prioStr := s.PriorityIcon(priority) + " " + strings.ToUpper(priority)
		parts = append(parts, prioStr)
	}

	// Cleanly join all present parts with the faint chevron
	bc := lipgloss.NewStyle().Faint(true).Render(" " + core.GlyphChevron + " ")
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

	dimStyle := lipgloss.NewStyle().Faint(true)
	renderField := func(key string, width int) string {
		val := iss.DisplayStringField(key)
		if val == "" {
			return dimStyle.Render(pad(core.GlyphEmDash, width))
		}
		return s.DetailValue.Render(pad(val, width))
	}

	// Metadata blocks — each role group renders as a block of rows.
	// Currently each block is a single line, but the abstraction
	// allows blocks to grow into multi-line sections later.
	m.renderMetadataBlocks(&b, iss, s, renderField)

	divider := s.DetailDivider.Render(strings.Repeat(core.GlyphHorizLine, min(contentWidth, 64)))
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

	// Custom / dynamic fields section.
	m.renderFieldsSection(&b, iss, s, divider)

	// Child issues (sorted by key for stable ordering).
	m.sortedChildren = nil
	if len(iss.Children) > 0 {
		b.WriteString("\n" + divider + "\n")
		b.WriteString(s.ChildSection.Render(core.IconChildren+"CHILD ISSUES") + "\n")

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

		urgKey := m.urgencyFieldKey()
		for idx, child := range sortedChildren {
			icon, clr := s.StatusStyle(child.Status)
			statusStyle := lipgloss.NewStyle().Foreground(clr)
			typeClr := s.TypeColor(child.Type)

			childStatus := child.Status
			if len(childStatus) > 14 {
				childStatus = childStatus[:14]
			}

			var hintStr string
			if idx < len(m.hintKeys) {
				hintStr = lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("[%c]", m.hintKeys[idx]))
			}

			childType := child.Type
			if len(childType) > 10 {
				childType = childType[:10]
			}

			prio := s.PriorityIcon(child.StringField(urgKey))

			line := "  " + s.TreeGlyph.Render(core.GlyphReturn) + " " +
				lipgloss.NewStyle().Foreground(typeClr).Bold(true).Render(fmt.Sprintf(idFmt, child.ID)) + " " +
				prio + " " +
				lipgloss.NewStyle().Foreground(typeClr).Render(fmt.Sprintf(typeFmt, childType)) + " " +
				statusStyle.Render(fmt.Sprintf(statusFmt, icon, childStatus)) + " " +
				child.Summary
			if hintStr != "" {
				line += " " + hintStr
			}
			b.WriteString(line + "\n")
		}
	}

	// Comments.
	if len(iss.Comments) > 0 {
		b.WriteString("\n" + divider + "\n")
		b.WriteString(s.CommentSection.Render(core.IconComments+"LATEST COMMENTS") + "\n\n")

		commentSep := lipgloss.NewStyle().Faint(true).Render(core.GlyphHorizLine + core.GlyphHorizLine + core.GlyphHorizLine)
		for i, c := range iss.Comments {
			if i > 0 {
				b.WriteString("\n" + commentSep + "\n")
			}
			header := s.CommentAuthor.Render(c.Author) + "  " +
				s.CommentDate.Render(core.GlyphDot+" "+c.Created)
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

// renderMetadataBlocks writes the FieldDef-driven metadata section.
func (m *DetailModel) renderMetadataBlocks(b *strings.Builder, iss *core.WorkItem, s *terminal.Styles, renderField func(string, int) string) {
	// Ownership + temporal block: paired rows.
	ownership := m.visibleFieldsByRole(core.RoleOwnership)
	temporal := m.visibleFieldsByRole(core.RoleTemporal)
	for i := range max(len(ownership), len(temporal)) {
		if i < len(ownership) {
			def := ownership[i]
			lbl := s.MetadataLabelStyle(core.RoleOwnership, def.Primary, i).
				Render(m.fieldLabel(def, 12))
			b.WriteString(lbl + renderField(def.Key, 22))
		}
		if i < len(temporal) {
			def := temporal[i]
			lbl := s.MetadataLabelStyle(core.RoleTemporal, def.Primary, i).
				Render(m.fieldLabel(def, 9))
			b.WriteString(" " + lbl + s.DetailValue.Render(iss.DisplayStringField(def.Key)))
		}
		b.WriteString("\n")
	}

	// Categorisation block: one row per non-empty field.
	categ := m.visibleFieldsByRole(core.RoleCategorisation)
	for i, def := range categ {
		val := iss.DisplayStringField(def.Key)
		if val == "" {
			continue
		}
		lbl := s.MetadataLabelStyle(core.RoleCategorisation, def.Primary, i).
			Render(m.fieldLabel(def, 12))
		b.WriteString(lbl + s.DetailValue.Render(val) + "\n")
	}

	// Parent block (structural — not driven by FieldDefs).
	if iss.ParentID != "" {
		lblParent := s.ParentLabelStyle().Render(core.IconParent + fmt.Sprintf("%-12s", "Parent:"))
		b.WriteString(lblParent + lipgloss.NewStyle().Bold(true).Render(iss.ParentID) + "\n")
	}
}

// renderFieldsSection writes the FIELDS section for custom/dynamic fields.
// Auto-discovered fields (not Pinned) are only shown if they have a value.
// Pinned fields (user opted-in via per-type config) are always shown, with
// an em dash if empty.
func (m *DetailModel) renderFieldsSection(b *strings.Builder, iss *core.WorkItem, s *terminal.Styles, divider string) {
	custom := m.visibleFieldsByRole(core.RoleCustom)
	if len(custom) == 0 {
		return
	}

	// First pass: collect visible fields and compute max label width.
	type fieldEntry struct {
		def core.FieldDef
		val string
	}
	var visible []fieldEntry
	maxLabel := 0
	for _, def := range custom {
		val := iss.DisplayStringField(def.Key)
		if val == "" && !def.Pinned {
			continue
		}
		visible = append(visible, fieldEntry{def, val})
		if w := len(def.Label) + 2; w > maxLabel { // +1 for ":", +1 for trailing space
			maxLabel = w
		}
	}

	// Second pass: render with aligned labels.
	var rows []string
	for _, fe := range visible {
		label := s.MetadataLabelStyle(core.RoleCustom, false, 0).
			Render(m.fieldLabel(fe.def, maxLabel))
		if fe.val == "" {
			rows = append(rows, label+lipgloss.NewStyle().Faint(true).Render(core.GlyphEmDash))
		} else {
			rows = append(rows, label+s.DetailValue.Render(fe.val))
		}
	}

	if len(rows) == 0 {
		return
	}

	b.WriteString("\n" + divider + "\n")
	b.WriteString(s.ChildSection.Render(core.IconFields+"FIELDS") + "\n")
	for _, row := range rows {
		b.WriteString(row + "\n")
	}
}

// fieldLabel formats a FieldDef's icon and label into a padded string
// suitable for the metadata section. The width parameter is the total
// width after the icon prefix (icon + " " is prepended if present).
func (m *DetailModel) fieldLabel(def core.FieldDef, width int) string {
	labelText := fmt.Sprintf("%-*s", width, def.Label+":")
	icon := def.Icon
	if icon == "" && def.Role == core.RoleCustom {
		icon = core.IconField
	}
	if icon != "" {
		return icon + labelText
	}
	return " " + labelText
}

func (m *DetailModel) renderEmpty() string {
	msg := lipgloss.NewStyle().Faint(true).Render("Select an issue to view details")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}
