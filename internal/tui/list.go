package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// List table column indices. Kept in one place so StyleFunc and the
// buildRowCells slice ordering stay in sync.
const (
	colID int = iota
	colPrio
	colType
	colStatus
	colAssignee
	colSummary
)

// listItem wraps a WorkItem with display metadata for the list.
type listItem struct {
	Issue         *core.WorkItem
	Depth         int
	IsLast        bool     // Last child at this depth (for tree glyphs).
	Ancestors     []bool   // For each depth level, whether that ancestor is the last child.
	AncestorTypes []string // Issue type at each ancestor depth level (for tree glyph coloring).
	Injected      bool     // Parent injected for context (not a real match).
	ParentType    string   // Immediate parent's issue type.
}

// ListModel is the fuzzy-filterable issue list panel.
type ListModel struct {
	// Data.
	allItems  []listItem // Full flattened tree.
	filtered  []listItem // After fuzzy filter.
	matchIdxs map[int][]int
	maxIDW    int // Widest issue ID across allItems (drives summary budget).

	// State.
	cursor int
	offset int // First visible row (for scrolling).
	search textinput.Model

	// Config.
	styles        *terminal.Styles
	fieldDefs     core.FieldDefs
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
	fieldDefs core.FieldDefs,
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
		fieldDefs:   fieldDefs,
		statusOrder: statusOrder,
		typeOrder:   typeOrder,
	}
	lm.updateMaxIDW()
	lm.updatePrompt()
	return lm
}

// updateMaxIDW scans allItems for the widest issue ID. Used to compute
// the summary-column budget dynamically instead of hardcoding a width.
func (m *ListModel) updateMaxIDW() {
	w := 0
	for _, item := range m.allItems {
		if iw := lipgloss.Width(item.Issue.ID); iw > w {
			w = iw
		}
	}
	m.maxIDW = w
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
	m.updateMaxIDW()
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
		// Pre-size the clones: parent slice + 1 for the new tail element.
		currentAncestors := make([]bool, 0, len(ancestors)+1)
		currentAncestors = append(append(currentAncestors, ancestors...), isLast)
		currentAncestorTypes := make([]string, 0, len(ancestorTypes)+1)
		currentAncestorTypes = append(append(currentAncestorTypes, ancestorTypes...), v.Type)

		parentType := ""
		if len(ancestorTypes) > 0 {
			parentType = ancestorTypes[len(ancestorTypes)-1]
		}

		*out = append(*out, listItem{
			Issue:         v,
			Depth:         depth,
			IsLast:        isLast,
			Ancestors:     currentAncestors,
			AncestorTypes: slices.Clone(ancestorTypes), // Types of ancestors ABOVE this node.
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

// SelectedIssue returns the currently highlighted issue, or nil.
func (m *ListModel) SelectedIssue() *core.WorkItem {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.filtered[m.cursor].Issue
	}
	return nil
}

// summaryBudget returns the cells available for the summary column in
// table mode. Thin wrapper over the pure CalculateListLayout so that
// buildRowCells has a direct accessor without re-threading layout
// through every call site.
func (m *ListModel) summaryBudget() int {
	return CalculateListLayout(m.width, m.height, m.maxIDW).SummaryBudget
}

// SetSize updates the available dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	promptW := lipgloss.Width(m.search.Prompt)
	m.search.SetWidth(w - promptW)
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

	ownerKey := ""
	if def := m.fieldDefs.ByRole(core.RoleOwnership).Primary(); def != nil {
		ownerKey = def.Key
	}
	sources := make([]string, len(m.allItems))
	for i, item := range m.allItems {
		iss := item.Issue
		sources[i] = iss.ID + " " + iss.Summary + " " +
			iss.DisplayStringField(ownerKey) + " " + iss.Status + " " + iss.Type
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

	// Search results are a flat list — fuzzy.Find orders by relevance, so
	// items no longer sit adjacent to their relatives and tree glyphs
	// would render disconnected. Strip tree metadata (Depth/IsLast/
	// Ancestors) on every filtered row so no tree prefix is drawn.
	flatten := func(item listItem) listItem {
		item.Depth = 0
		item.IsLast = false
		item.Ancestors = nil
		item.AncestorTypes = nil
		return item
	}

	for _, match := range matches {
		item := m.allItems[match.Index]
		iss := item.Issue

		// Inject parent for context if child matched but parent didn't.
		if iss.ParentID != "" && !seen[iss.ParentID] {
			if parent := findItemByKey(m.allItems, iss.ParentID); parent != nil &&
				!matchedSet[indexOfKey(m.allItems, iss.ParentID)] {
				m.filtered = append(m.filtered, listItem{
					Issue: parent.Issue, Injected: true,
				})
				seen[iss.ParentID] = true
			}
		}

		if !seen[iss.ID] {
			m.filtered = append(m.filtered, flatten(item))
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
// Layout is driven by lipgloss/v2/table. When the summary column can
// support at least cardModeMinBudget cells, rows render as single-line
// table rows (table mode). Below that threshold, View switches to card
// mode: two lines per item (compact metadata row + summary line) with
// the SUMMARY header and tree glyph dropped.
func (m ListModel) View() string {
	if m.width == 0 {
		return ""
	}

	layout := CalculateListLayout(m.width, m.height, m.maxIDW)
	start, end := CalculateScrollWindow(m.cursor, m.offset, len(m.filtered), layout.ItemsVisible)
	m.offset = start

	// Column labels — derived from the primary FieldDef for each role.
	urgLabel := "P"
	if def := m.fieldDefs.ByRole(core.RoleUrgency).Primary(); def != nil {
		urgLabel = def.ShortLabel()
	}
	ownerLabel := "OWNER"
	if def := m.fieldDefs.ByRole(core.RoleOwnership).Primary(); def != nil {
		ownerLabel = strings.ToUpper(def.ShortLabel())
	}

	if layout.CardMode {
		return m.renderCards(start, end, layout.ItemsVisible, urgLabel, ownerLabel)
	}
	return m.renderTable(start, end, layout.ItemsVisible, urgLabel, ownerLabel)
}

// renderTable renders the classic single-line-per-row list.
func (m *ListModel) renderTable(start, end, visible int, urgLabel, ownerLabel string) string {
	rows := make([][]string, 0, end-start)
	selectedRow := -1
	for i := start; i < end; i++ {
		if i == m.cursor {
			selectedRow = i - start
		}
		rows = append(rows, m.buildRowCells(m.filtered[i], i == m.cursor))
	}

	cursorBg := m.styles.Cursor.GetBackground()
	headerStyle := m.styles.ColumnHeader

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		Wrap(false).
		Headers("ID", urgLabel, "TYPE", "STATUS", ownerLabel, "SUMMARY").
		StyleFunc(func(row, col int) lipgloss.Style {
			pad := m.colPadding(col)
			if row == table.HeaderRow {
				return headerStyle.PaddingRight(pad)
			}
			st := lipgloss.NewStyle().PaddingRight(pad)
			if row == selectedRow {
				st = st.Background(cursorBg)
			}
			return st
		}).
		Rows(rows...)

	return padToHeight(t.Render(), visible+1)
}

// renderCards renders one card per item: a 5-column metadata table row
// (no SUMMARY column / header) followed by a summary line beneath. The
// metadata table is rendered once, then its data lines are interleaved
// with summary lines so column alignment is preserved across cards.
func (m *ListModel) renderCards(start, end, itemsVisible int, urgLabel, ownerLabel string) string {
	rows := make([][]string, 0, end-start)
	selectedRow := -1
	for i := start; i < end; i++ {
		if i == m.cursor {
			selectedRow = i - start
		}
		cells := m.buildRowCells(m.filtered[i], i == m.cursor)
		// Drop the summary column for card mode.
		rows = append(rows, cells[:colSummary])
	}

	cursorBg := m.styles.Cursor.GetBackground()
	headerStyle := m.styles.ColumnHeader

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		Wrap(false).
		Headers("ID", urgLabel, "TYPE", "STATUS", ownerLabel).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Last column (assignee) has no trailing pad in card mode.
			pad := m.colPadding(col)
			if col == colAssignee {
				pad = 0
			}
			if row == table.HeaderRow {
				return headerStyle.PaddingRight(pad)
			}
			st := lipgloss.NewStyle().PaddingRight(pad)
			if row == selectedRow {
				st = st.Background(cursorBg)
			}
			return st
		}).
		Rows(rows...)

	tableLines := strings.Split(t.Render(), "\n")
	if len(tableLines) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(tableLines[0]) // header row

	// Each card: metadata line + summary line (2 lines per item).
	for i := start; i < end; i++ {
		idx := i - start
		metaLine := ""
		if idx+1 < len(tableLines) {
			metaLine = tableLines[idx+1]
		}
		b.WriteString("\n" + metaLine)
		b.WriteString("\n" + m.buildSummaryLine(m.filtered[i], i == m.cursor))
	}

	// Pad to the full height: 1 header + itemsVisible * 2 lines.
	return padToHeight(b.String(), 1+itemsVisible*2)
}

// colPadding returns the trailing padding for a given column. Shared
// between table mode and card mode so gaps stay consistent.
func (m *ListModel) colPadding(col int) int {
	switch col {
	case colPrio:
		return 1
	case colSummary:
		return 1
	default:
		return 3
	}
}

// padToHeight ensures a rendered block is exactly wantLines tall by
// appending blank lines. Used to keep list height constant across
// filter/scroll so the FZF-style fixed viewport doesn't jitter.
func padToHeight(rendered string, wantLines int) string {
	have := strings.Count(rendered, "\n") + 1
	if have < wantLines {
		rendered += strings.Repeat("\n", wantLines-have)
	}
	return rendered
}

// buildRowCells returns the six pre-styled cells for one list row: ID,
// priority, type, status, assignee, summary (tree prefix + summary body).
// When selected, each styled span has the cursor background baked in so
// lipgloss's row-level background painting in StyleFunc extends cleanly
// across padding spaces between cells.
func (m *ListModel) buildRowCells(item listItem, selected bool) []string {
	s := m.styles
	iss := item.Issue

	cursorBg := s.Cursor.GetBackground()
	withBg := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(cursorBg)
		}
		return st
	}

	// ID cell — coloured by the issue's type.
	typeColor := s.TypeColor(iss.Type)
	keyStyle := withBg(lipgloss.NewStyle().Foreground(typeColor)).Bold(true)
	if item.Injected {
		keyStyle = withBg(s.IssueKeyDim)
	}
	idLabel := iss.ID
	if iss.DisplayID != "" {
		idLabel = iss.DisplayID
	}
	keyCell := keyStyle.Render(idLabel)

	// Priority icon.
	urgKey := ""
	if def := m.fieldDefs.ByRole(core.RoleUrgency).Primary(); def != nil {
		urgKey = def.Key
	}
	prioCell := s.PriorityIconWithBg(iss.StringField(urgKey), selected)

	// Type.
	typeName := iss.Type
	if len(typeName) > 10 {
		typeName = typeName[:10]
	}
	typeCell := withBg(lipgloss.NewStyle().Foreground(typeColor)).Render(typeName)

	// Status icon + name.
	icon, statusColor := s.StatusStyle(iss.Status)
	statusName := iss.Status
	if len(statusName) > 14 {
		statusName = statusName[:14]
	}
	statusCell := withBg(lipgloss.NewStyle().Foreground(statusColor)).Render(icon + " " + statusName)

	// Assignee.
	ownerKey := ""
	if def := m.fieldDefs.ByRole(core.RoleOwnership).Primary(); def != nil {
		ownerKey = def.Key
	}
	assignee := iss.DisplayStringField(ownerKey)
	if assignee == "" {
		assignee = core.GlyphEmDash
	}
	if len(assignee) > 16 {
		assignee = assignee[:13] + "..."
	}
	assigneeCell := withBg(lipgloss.NewStyle().Faint(true)).Render(assignee)

	// Summary cell: tree prefix + styled body + optional child count.
	treePart := m.renderColoredTreePrefix(item, selected)

	summaryBody := iss.Summary
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

	budget := m.summaryBudget() - lipgloss.Width(treePart)
	childSuffix := ""
	if len(iss.Children) > 0 {
		childSuffix = fmt.Sprintf(" (%d sub)", len(iss.Children))
	}
	// Reserve space for the child-count suffix before truncating the body.
	bodyBudget := budget - len(childSuffix)
	if bodyBudget > 0 && lipgloss.Width(summaryBody) > bodyBudget {
		summaryBody = ansi.Truncate(summaryBody, bodyBudget, "…")
	}
	summaryRendered := summaryStyle.Render(summaryBody)
	if childSuffix != "" {
		summaryRendered += withBg(s.ChildCount).Render(childSuffix)
	}
	summaryCell := treePart + summaryRendered

	return []string{keyCell, prioCell, typeCell, statusCell, assigneeCell, summaryCell}
}

// buildSummaryLine renders the card-mode summary line that sits below
// each metadata row. Tree glyph is omitted by design — narrow-width
// layouts drop hierarchy for readability. When selected, the full line
// (including right-padding) carries the cursor background so the card
// reads as a contiguous highlighted block.
func (m *ListModel) buildSummaryLine(item listItem, selected bool) string {
	s := m.styles
	iss := item.Issue

	cursorBg := s.Cursor.GetBackground()
	withBg := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(cursorBg)
		}
		return st
	}

	// Style summary body — same rules as the table-mode summary cell.
	summaryStyle := withBg(s.Summary)
	if strings.ToLower(iss.Type) != "task" {
		summaryStyle = withBg(lipgloss.NewStyle().Foreground(s.TypeColor(iss.Type)))
	}
	if item.Injected {
		summaryStyle = summaryStyle.Faint(true)
	}
	if selected {
		summaryStyle = summaryStyle.Bold(true)
	}

	// Indent the summary so it visually belongs to its card. Matches
	// the ID column width so the summary sits under the ID header.
	const indent = 2
	summaryBody := iss.Summary
	childSuffix := ""
	if len(iss.Children) > 0 {
		childSuffix = fmt.Sprintf(" (%d sub)", len(iss.Children))
	}
	budget := m.width - indent - len(childSuffix) - 1 // 1 for trailing pad
	if budget > 0 && lipgloss.Width(summaryBody) > budget {
		summaryBody = ansi.Truncate(summaryBody, budget, "…")
	}

	indentStr := strings.Repeat(" ", indent)
	if selected {
		indentStr = lipgloss.NewStyle().Background(cursorBg).Render(indentStr)
	}
	line := indentStr + summaryStyle.Render(summaryBody)
	if childSuffix != "" {
		line += withBg(s.ChildCount).Render(childSuffix)
	}

	// Extend cursor background across the full width for selected rows.
	if selected {
		visibleW := lipgloss.Width(line)
		if visibleW < m.width {
			line += lipgloss.NewStyle().Background(cursorBg).Render(
				strings.Repeat(" ", m.width-visibleW),
			)
		}
	}
	return line
}

// renderColoredTreePrefix renders the tree prefix with the branch glyph
// colored by the parent's type color, including vertical connection lines.
// Token sequence comes from the pure BuildTreeTokens function; this
// method only maps tokens to styled glyph strings.
func (m *ListModel) renderColoredTreePrefix(item listItem, selected bool) string {
	tokens := BuildTreeTokens(item.Depth, item.IsLast, item.Ancestors)
	if len(tokens) == 0 {
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
	for i, tok := range tokens {
		glyph := tok.Glyph()
		switch tok {
		case TokenSpace:
			if selected {
				b.WriteString(lipgloss.NewStyle().Background(cursorBg).Render(glyph))
			} else {
				b.WriteString(glyph)
			}
		case TokenVert:
			// Vertical connector at column i is coloured by the
			// ancestor it belongs to (AncestorTypes[i]).
			var ancType string
			if i < len(item.AncestorTypes) {
				ancType = item.AncestorTypes[i]
			}
			ancColor := s.TypeColor(ancType)
			b.WriteString(withBg(lipgloss.NewStyle().Foreground(ancColor)).Render(glyph))
		case TokenTee, TokenCorner:
			if item.ParentType != "" {
				parentClr := s.TypeColor(item.ParentType)
				b.WriteString(withBg(lipgloss.NewStyle().Foreground(parentClr)).Render(glyph))
			} else {
				b.WriteString(withBg(lipgloss.NewStyle().Faint(true)).Render(glyph))
			}
		}
	}
	return b.String()
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
	chevron := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Muted).Render(core.GlyphChevron + " ")

	m.search.Prompt = countStyled + chevron
}
