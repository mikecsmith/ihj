package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// listItem wraps a WorkItem with display metadata for the list.
type listItem struct {
	Issue         *core.WorkItem
	Depth         int
	IsLast        bool     // Last child at this depth (for tree glyphs).
	Ancestors     []bool   // For each depth level, whether that ancestor is the last child.
	AncestorTypes []string // Issue type at each ancestor depth level (for tree glyph coloring).
	Injected      bool     // Parent injected for context (not a real match).
	TreePrefix    string   // Pre-computed tree glyph prefix for the summary column.
	ParentType    string   // Immediate parent's issue type.
}

// ListModel is the fuzzy-filterable issue list panel.
type ListModel struct {
	// Data.
	allItems  []listItem // Full flattened tree.
	filtered  []listItem // After fuzzy filter.
	matchIdxs map[int][]int

	// State.
	cursor      int
	offset      int // First visible row (for scrolling).
	search      textinput.Model
	maxRowWidth int // Cached max row width across all filtered items for highlight bar.

	// Config.
	styles        *terminal.Styles
	statusOrder   map[string]core.StatusOrderEntry
	typeOrder     map[string]core.TypeOrderEntry
	width, height int
}

// NewListModel creates a list model from a built and linked registry.
func NewListModel(
	registry map[string]*core.WorkItem,
	styles *terminal.Styles,
	statusOrder map[string]core.StatusOrderEntry,
	typeOrder map[string]core.TypeOrderEntry,
) ListModel {
	roots := core.Roots(registry)
	core.SortItems(roots, statusOrder, typeOrder)

	var items []listItem
	flattenTree(roots, 0, nil, nil, &items, statusOrder, typeOrder)

	ti := textinput.New()
	ti.Placeholder = ""
	ti.Prompt = "> "
	ti.CharLimit = 120
	ti.Focus()

	lm := ListModel{
		allItems:    items,
		filtered:    items,
		matchIdxs:   make(map[int][]int),
		search:      ti,
		styles:      styles,
		statusOrder: statusOrder,
		typeOrder:   typeOrder,
	}
	lm.updateMaxRowWidth()
	lm.updatePrompt()
	return lm
}

// Rebuild re-flattens the issue tree from the registry, preserving the current
// search query and cursor position by tracking the selected issue key.
func (m *ListModel) Rebuild(registry map[string]*core.WorkItem) {
	// Remember the currently selected issue key so we can restore position.
	var selectedKey string
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		selectedKey = m.filtered[m.cursor].Issue.ID
	}

	roots := core.Roots(registry)
	core.SortItems(roots, m.statusOrder, m.typeOrder)

	var items []listItem
	flattenTree(roots, 0, nil, nil, &items, m.statusOrder, m.typeOrder)
	m.allItems = items
	m.applyFilter()

	// Restore cursor to the same issue if still present.
	if selectedKey != "" {
		for i, item := range m.filtered {
			if item.Issue.ID == selectedKey {
				m.cursor = i
				return
			}
		}
	}
}

// flattenTree converts the issue tree into a flat list with tree-command-style
// glyph prefixes. ancestorTypes tracks the issue type at each depth for coloring.
func flattenTree(
	items []*core.WorkItem, depth int, ancestors []bool, ancestorTypes []string,
	out *[]listItem, sw map[string]core.StatusOrderEntry, to map[string]core.TypeOrderEntry,
) {
	for i, v := range items {
		isLast := i == len(items)-1
		currentAncestors := append(append([]bool(nil), ancestors...), isLast)
		currentAncestorTypes := append(append([]string(nil), ancestorTypes...), v.Type)

		// Build tree prefix for summary column.
		prefix := buildTreePrefix(depth, isLast)

		parentType := ""
		if len(ancestorTypes) > 0 {
			parentType = ancestorTypes[len(ancestorTypes)-1]
		}

		*out = append(*out, listItem{
			Issue:         v,
			Depth:         depth,
			IsLast:        isLast,
			Ancestors:     currentAncestors,
			AncestorTypes: append([]string(nil), ancestorTypes...), // Types of ancestors ABOVE this node.
			TreePrefix:    prefix,
			ParentType:    parentType,
		})

		if len(v.Children) > 0 {
			children := make([]*core.WorkItem, len(v.Children))
			copy(children, v.Children)
			core.SortItems(children, sw, to)
			flattenTree(children, depth+1, currentAncestors, currentAncestorTypes, out, sw, to)
		}
	}
}

// buildTreePrefix creates tree glyphs for the summary column.
// Uses "  " (2 spaces) per depth level matching the original Python TUI's
// `"  " * depth`, then ├─/└─ for the branch.
func buildTreePrefix(depth int, isLast bool) string {
	if depth == 0 {
		return ""
	}

	var b strings.Builder
	// 2 spaces per depth level (matching Python's "  " * depth).
	for range depth {
		b.WriteString("  ")
	}
	// Branch glyph.
	if isLast {
		b.WriteString("└─ ")
	} else {
		b.WriteString("├─ ")
	}
	return b.String()
}

// SelectedIssue returns the currently highlighted issue, or nil.
func (m *ListModel) SelectedIssue() *core.WorkItem {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.filtered[m.cursor].Issue
	}
	return nil
}

// SetSize updates the available dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	promptW := lipgloss.Width(m.search.Prompt)
	m.search.SetWidth(w - promptW)
	m.updateMaxRowWidth()
}

// ScrollList scrolls the list by delta rows (positive = down).
func (m *ListModel) ScrollList(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m ListModel) Init() tea.Cmd { return textinput.Blink }

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	// Navigation is handled by app.go — this only processes search input.
	var cmd tea.Cmd
	prevQuery := m.search.Value()
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != prevQuery {
		m.applyFilter()
	}
	return m, cmd
}

func (m *ListModel) applyFilter() {
	query := strings.TrimSpace(m.search.Value())

	if query == "" {
		m.filtered = m.allItems
		m.matchIdxs = make(map[int][]int)
		m.cursor = min(m.cursor, max(0, len(m.filtered)-1))
		m.updatePrompt()
		return
	}

	sources := make([]string, len(m.allItems))
	for i, item := range m.allItems {
		iss := item.Issue
		sources[i] = iss.ID + " " + iss.Summary + " " +
			iss.DisplayStringField("assignee") + " " + iss.Status + " " + iss.Type
	}

	matches := fuzzy.Find(query, sources)

	matchedSet := make(map[int]bool, len(matches))
	m.matchIdxs = make(map[int][]int, len(matches))
	for _, match := range matches {
		matchedSet[match.Index] = true
		m.matchIdxs[match.Index] = match.MatchedIndexes
	}

	seen := make(map[string]bool)
	m.filtered = nil

	for _, match := range matches {
		item := m.allItems[match.Index]
		iss := item.Issue

		// Inject parent for context if child matched but parent didn't.
		if iss.ParentID != "" && !seen[iss.ParentID] {
			if parent := findItemByKey(m.allItems, iss.ParentID); parent != nil &&
				!matchedSet[indexOfKey(m.allItems, iss.ParentID)] {
				m.filtered = append(m.filtered, listItem{
					Issue: parent.Issue, Depth: 0, Injected: true,
				})
				seen[iss.ParentID] = true
			}
		}

		if !seen[iss.ID] {
			m.filtered = append(m.filtered, item)
			seen[iss.ID] = true
		}
	}

	m.cursor = 0
	m.updatePrompt()
}

// SearchBarView returns the search input line (rendered separately in the layout).
func (m ListModel) SearchBarView() string {
	return m.search.View()
}

// View returns the column header + list rows (without the search bar).
// The list always renders exactly m.height lines (fixed size, like FZF).
func (m ListModel) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Column header.
	header := m.styles.ColumnHeader.Render(
		fmt.Sprintf("%-12s P %-10s %-16s %-16s SUMMARY", "ID", "TYPE", "STATUS", "ASSIGNEE"),
	)
	b.WriteString(header)

	// List rows with proper scrolling.
	visible := m.visibleRows()
	start := min(m.cursor, m.offset)
	if m.cursor >= start+visible {
		start = m.cursor - visible + 1
	}
	if start < 0 {
		start = 0
	}
	m.offset = start

	end := min(start+visible, len(m.filtered))

	rendered := 0
	for i := start; i < end; i++ {
		b.WriteString("\n")
		b.WriteString(m.renderRow(m.filtered[i], i == m.cursor, m.maxRowWidth))
		rendered++
	}

	// Pad remaining rows with empty lines to maintain fixed height.
	for rendered < visible {
		b.WriteString("\n")
		rendered++
	}

	return b.String()
}

// renderRow renders a single list row. When selected, an optional padToWidth
// sets the highlight bar length (aligned to the longest visible row + 2).
func (m *ListModel) renderRow(item listItem, selected bool, padToWidth ...int) string {
	s := m.styles
	iss := item.Issue

	// When selected, every column style gets the cursor background so the
	// highlight bar is visually continuous across the entire row.
	cursorBg := s.Cursor.GetBackground()
	withBg := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(cursorBg)
		}
		return st
	}

	// Type color — applied to key and type columns.
	typeColor := s.TypeColor(iss.Type)
	typeStyle := withBg(lipgloss.NewStyle().Foreground(typeColor))

	// Key column (flat, never indented).
	keyStyle := typeStyle.Bold(true)
	if item.Injected {
		keyStyle = withBg(s.IssueKeyDim)
	}
	key := keyStyle.Render(fmt.Sprintf("%-12s", iss.ID))

	// Priority icon.
	priority := iss.StringField("priority")
	prio := s.PriorityIconWithBg(priority, selected)

	// Type column.
	typeName := iss.Type
	if len(typeName) > 10 {
		typeName = typeName[:10]
	}
	typeCol := typeStyle.Render(fmt.Sprintf("%-10s", typeName))

	// Status column with icon.
	icon, statusColor := s.StatusStyle(iss.Status)
	statusStyle := withBg(lipgloss.NewStyle().Foreground(statusColor))
	statusName := iss.Status
	if len(statusName) > 14 {
		statusName = statusName[:14]
	}
	statusCol := statusStyle.Render(fmt.Sprintf("%s %-14s", icon, statusName))

	// Assignee column (dimmed). Show em dash for unassigned items.
	assignee := iss.DisplayStringField("assignee")
	if assignee == "" {
		assignee = "—"
	}
	if len(assignee) > 16 {
		assignee = assignee[:13] + "..."
	}
	assigneeCol := withBg(lipgloss.NewStyle().Faint(true)).Render(fmt.Sprintf("%-16s", assignee))

	// Summary with tree prefix — each segment colored per ancestor type.
	treePart := m.renderColoredTreePrefix(item, selected)

	summaryBody := iss.Summary
	if len(iss.Children) > 0 {
		summaryBody += withBg(s.ChildCount).Render(fmt.Sprintf(" (%d sub)", len(iss.Children)))
	}

	// Summary color: tasks use default, non-tasks use type color (matching original).
	summaryStyle := withBg(s.Summary)
	if strings.ToLower(iss.Type) != "task" {
		summaryStyle = withBg(lipgloss.NewStyle().Foreground(typeColor))
	}
	if item.Injected {
		summaryStyle = summaryStyle.Faint(true)
	}
	if selected {
		summaryStyle = summaryStyle.Bold(true)
	}

	summaryText := treePart + summaryStyle.Render(summaryBody)

	// Truncate summary to available width.
	// key(12) + sp(1) + prio(1) + sp(1) + type(10) + sp(1) + status_icon(1)+sp(1)+status(14) + sp(1) + assignee(16) + sp(1) = 60
	colsUsed := 60
	summaryW := m.width - colsUsed
	if summaryW > 0 && lipgloss.Width(summaryText) > summaryW {
		runes := []rune(summaryBody)
		if len(runes) > summaryW-3 {
			summaryText = treePart + summaryStyle.Render(string(runes[:summaryW-3])+"...")
		}
	}

	// Spaces between columns also need the cursor background.
	sp := " "
	if selected {
		sp = lipgloss.NewStyle().Background(cursorBg).Render(" ")
	}

	line := key + sp + prio + sp + typeCol + sp + statusCol + sp + assigneeCol + sp + summaryText

	if selected {
		// Pad to the target width so the highlight bar aligns with the
		// longest visible summary + 2 spaces of breathing room.
		targetW := 0
		if len(padToWidth) > 0 {
			targetW = padToWidth[0]
		}
		visible := lipgloss.Width(line)
		if targetW > visible {
			line += lipgloss.NewStyle().Background(cursorBg).Render(strings.Repeat(" ", targetW-visible))
		} else {
			line += lipgloss.NewStyle().Background(cursorBg).Render("  ")
		}
		return line
	}
	return line
}

// renderColoredTreePrefix renders the tree prefix with the branch glyph
// colored by the parent's type color, including vertical connection lines.
func (m *ListModel) renderColoredTreePrefix(item listItem, selected bool) string {
	if item.Depth == 0 {
		return ""
	}

	s := m.styles
	cursorBg := s.Cursor.GetBackground()
	withBg := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(cursorBg)
		}
		return st
	}

	var b strings.Builder

	b.WriteString("")

	for i := 1; i < item.Depth; i++ {
		// item.Ancestors[i] tells us if the ancestor at this depth level was the LAST child.
		if item.Ancestors[i] {
			// If it was the last child, the branch is closed. Just print spaces.
			if selected {
				b.WriteString(lipgloss.NewStyle().Background(cursorBg).Render("  "))
			} else {
				b.WriteString("  ")
			}
		} else {
			// If it wasn't the last child, the branch is still open. Draw the vertical line.
			// We color this line based on the ancestor that owns it (depth i-1).
			ancColor := s.TypeColor(item.AncestorTypes[i-1])
			b.WriteString(withBg(lipgloss.NewStyle().Foreground(ancColor)).Render("│ "))
		}
	}

	var branch string
	if item.IsLast {
		branch = "└─ "
	} else {
		branch = "├─ "
	}

	// Color the branch glyph based on the immediate parent
	if item.ParentType != "" {
		parentClr := s.TypeColor(item.ParentType)
		b.WriteString(withBg(lipgloss.NewStyle().Foreground(parentClr)).Render(branch))
	} else {
		b.WriteString(withBg(lipgloss.NewStyle().Faint(true)).Render(branch))
	}

	return b.String()
}

// updateMaxRowWidth recalculates the cached max row width from all items.
// Called when the dataset changes (construction, rebuild, resize). Uses
// allItems so the highlight bar width stays stable regardless of search.
func (m *ListModel) updateMaxRowWidth() {
	// Fixed columns: key(12)+sp+prio(1)+sp+type(10)+sp+status(17)+sp+assignee(16)+sp = 60
	const fixedCols = 60
	// Summary column is capped at m.width - fixedCols by renderRow truncation.
	summaryMax := m.width - fixedCols
	maxW := fixedCols
	for _, item := range m.allItems {
		iss := item.Issue
		// Tree prefix width: each depth level = 2 chars, branch glyph = 3 chars.
		treePrefixW := 0
		if item.Depth > 0 {
			treePrefixW = (item.Depth-1)*2 + 3
		}
		summaryW := len([]rune(iss.Summary))
		if len(iss.Children) > 0 {
			summaryW += len(fmt.Sprintf(" (%d sub)", len(iss.Children)))
		}
		displayW := treePrefixW + summaryW
		if summaryMax > 0 && displayW > summaryMax {
			displayW = summaryMax
		}
		total := fixedCols + displayW
		if total > maxW {
			maxW = total
		}
	}
	m.maxRowWidth = maxW + 2 // 2 spaces trailing pad
}

func (m *ListModel) visibleRows() int {
	// One line per item, minus header row.
	rows := m.height - 1
	if rows < 1 {
		return 1
	}
	return rows
}

func findItemByKey(items []listItem, key string) *listItem {
	for i := range items {
		if items[i].Issue.ID == key {
			return &items[i]
		}
	}
	return nil
}

func indexOfKey(items []listItem, key string) int {
	for i := range items {
		if items[i].Issue.ID == key {
			return i
		}
	}
	return -1
}

func (m *ListModel) updatePrompt() {
	countStr := fmt.Sprintf(" %d/%d ", len(m.filtered), len(m.allItems))

	countStyled := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Info).Render(countStr)
	chevron := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Muted).Render("❯ ")

	m.search.Prompt = countStyled + chevron
}
