package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	ownerKey := ""
	if def := m.fieldDefs.ByRole(core.RoleOwnership).Primary(); def != nil {
		ownerKey = def.Key
	}
	result := filterItems(m.allItems, m.search.Value(), ownerKey)
	m.filtered = result.items
	m.matchIdxs = result.matchIdxs
	if result.reset {
		m.cursor = 0
	} else {
		m.cursor = min(m.cursor, max(0, len(m.filtered)-1))
	}
	m.updatePrompt()
}

// filterResult holds the output of the pure filterItems function.
type filterResult struct {
	items     []listItem
	matchIdxs map[int][]int
	reset     bool // true when a query was active (cursor should reset to 0)
}

// filterItems performs fuzzy filtering over allItems and returns the
// filtered list with parent injection and match indices. It is a pure
// function — no model state is read or written — making it independently
// testable without constructing a ListModel.
func filterItems(allItems []listItem, rawQuery string, ownerKey string) filterResult {
	query := strings.TrimSpace(rawQuery)

	if query == "" {
		return filterResult{
			items:     allItems,
			matchIdxs: make(map[int][]int),
		}
	}

	sources := make([]string, len(allItems))
	for i, item := range allItems {
		iss := item.Issue
		sources[i] = iss.ID + " " + iss.Summary + " " +
			iss.DisplayStringField(ownerKey) + " " + iss.Status + " " + iss.Type
	}

	matches := fuzzy.Find(query, sources)

	matchedSet := make(map[int]bool, len(matches))
	matchIdxs := make(map[int][]int, len(matches))
	for _, match := range matches {
		matchedSet[match.Index] = true
		matchIdxs[match.Index] = match.MatchedIndexes
	}

	seen := make(map[string]bool)
	var filtered []listItem

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
		item := allItems[match.Index]
		iss := item.Issue

		// Inject parent for context if child matched but parent didn't.
		if iss.ParentID != "" && !seen[iss.ParentID] {
			if parent := findItemByKey(allItems, iss.ParentID); parent != nil &&
				!matchedSet[indexOfKey(allItems, iss.ParentID)] {
				filtered = append(filtered, listItem{
					Issue: parent.Issue, Injected: true,
				})
				seen[iss.ParentID] = true
			}
		}

		if !seen[iss.ID] {
			filtered = append(filtered, flatten(item))
			seen[iss.ID] = true
		}
	}

	return filterResult{
		items:     filtered,
		matchIdxs: matchIdxs,
		reset:     true,
	}
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
