package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikecsmith/ihj/internal/core"
)

// ── Shared list styling ───────────────────────────────────────────

// withBackground returns a style-modifier that bakes in the cursor background
// when the row is selected. Used by both buildRowCells and buildSummaryLine
// so every styled span extends the highlight cleanly across padding.
func (m *ListModel) withBackground(selected bool) func(lipgloss.Style) lipgloss.Style {
	cursorBg := m.styles.Cursor.GetBackground()
	return func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(cursorBg)
		}
		return st
	}
}

// summaryStyle returns the style for the summary text. Non-task types
// are coloured by their type colour; selected rows are bold.
func (m *ListModel) summaryStyle(typeName string, selected bool, rowWithBackground func(lipgloss.Style) lipgloss.Style) lipgloss.Style {
	st := rowWithBackground(m.styles.Summary)
	if strings.ToLower(typeName) != "task" {
		st = rowWithBackground(lipgloss.NewStyle().Foreground(m.styles.TypeColor(typeName)))
	}
	if selected {
		st = st.Bold(true)
	}
	return st
}

// ── List layout (table vs card mode) ───────────────────────────────

// List column widths (in cells) and inter-column padding. These feed
// the summary-budget calculation and drive the threshold between the
// single-line table layout and the 2-line card layout.
const (
	listPrioW        = 1
	listTypeW        = 10
	listStatusW      = 16 // icon + space + 14-char name
	listAssigneeW    = 16
	listInterColGaps = 14 // sum of StyleFunc pads: 3+1+3+3+3+1

	// CardModeMinBudget is the smallest acceptable summary column
	// width in table mode. When the budget would fall below this, the
	// list switches to 2-line cards so summaries get room to breathe.
	CardModeMinBudget = 40

	// cardSummaryIndent is the left-indent for summary lines in card
	// mode, visually nesting the summary under the metadata row.
	cardSummaryIndent = 2

	// cardSummaryTrailPad is reserved at the end of the summary line
	// in card mode to prevent text from touching the right edge.
	cardSummaryTrailPad = 1
)

// ListLayout describes the list's rendering decisions for a given
// viewport and dataset: whether to render single-line table rows or
// 2-line cards, how many items the viewport can show, and how many
// cells are available for the summary column in table mode.
type ListLayout struct {
	CardMode      bool // true = 2-line cards (narrow), false = 1-line table rows
	ItemsVisible  int  // maximum items the window can display (≥ 1)
	SummaryBudget int  // cells for summary column (may be < CardModeMinBudget)
	RowsPerItem   int  // 1 in table mode, 2 in card mode
}

// CalculateListLayout is the pure decision function for list rendering.
// Given the inner content width/height and the widest issue ID in the
// dataset, it decides which mode to render in and how many items fit.
//
// Guarantees (see helpers_test.go):
//   - SummaryBudget == contentW - (maxIDW + fixed column widths + gaps).
//   - CardMode == (SummaryBudget < CardModeMinBudget).
//   - RowsPerItem is 1 iff !CardMode, else 2.
//   - ItemsVisible >= 1 for any contentH >= 1, and equals
//     floor((contentH-1) / RowsPerItem) otherwise (1 line reserved for
//     the header row).
func CalculateListLayout(contentW, contentH, maxIDW int) ListLayout {
	budget := contentW - maxIDW - listPrioW - listTypeW - listStatusW - listAssigneeW - listInterColGaps
	cardMode := budget < CardModeMinBudget

	rowsPerItem := 1
	if cardMode {
		rowsPerItem = 2
	}

	visibleLines := max(contentH-1, 1) // reserve the header row
	itemsVisible := max(visibleLines/rowsPerItem, 1)

	return ListLayout{
		CardMode:      cardMode,
		ItemsVisible:  itemsVisible,
		SummaryBudget: budget,
		RowsPerItem:   rowsPerItem,
	}
}

// ── Tree prefix glyphs ─────────────────────────────────────────────

// TreeToken is one column of a tree-prefix row. A sequence of tokens
// fully describes the shape of a row's branch/connector glyphs; the
// renderer maps each token to a concrete glyph string via Glyph() and
// then applies colour.
type TreeToken int

const (
	TokenSpace  TreeToken = iota // ancestor at this column was the last child (blank)
	TokenVert                    // ancestor had more siblings, connector continues
	TokenTee                     // this item has siblings after it
	TokenCorner                  // this item is the last child
)

// Glyph returns the two-cell-wide string that renders this token. All
// glyph values come from core.TreeColXxx, so the visual alphabet lives
// in one place and stays in sync across every tree renderer.
func (t TreeToken) Glyph() string {
	switch t {
	case TokenSpace:
		return core.TreeColSpace
	case TokenVert:
		return core.TreeColVert
	case TokenTee:
		return core.TreeColTee
	case TokenCorner:
		return core.TreeColCorner
	default:
		return core.TreeColSpace
	}
}

// BuildTreeTokens returns the column tokens for a tree row of the
// given depth. The returned slice has length == depth:
//
//   - tokens[0 .. depth-2] are connector columns for ancestors, driven
//     by the ancestors slice: ancestors[i] == true ⇒ TokenSpace at
//     tokens[i-1]; false ⇒ TokenVert. (ancestors[0] is unused — it
//     represents the root level.)
//   - tokens[depth-1] is the branch glyph for this item: TokenCorner
//     if isLast, otherwise TokenTee.
//
// Root items (depth == 0) have no tree prefix and produce an empty
// slice. The ancestors slice is read only and never mutated; callers
// may pass a shorter slice than depth, in which case missing entries
// are treated as false (draw a vertical connector).
func BuildTreeTokens(depth int, isLast bool, ancestors []bool) []TreeToken {
	if depth <= 0 {
		return nil
	}
	tokens := make([]TreeToken, depth)
	for i := 1; i < depth; i++ {
		if i < len(ancestors) && ancestors[i] {
			tokens[i-1] = TokenSpace
		} else {
			tokens[i-1] = TokenVert
		}
	}
	if isLast {
		tokens[depth-1] = TokenCorner
	} else {
		tokens[depth-1] = TokenTee
	}
	return tokens
}

// ── Table and card rendering ──────────────────────────────────────

// newListTable creates a borderless lipgloss table pre-configured with
// the shared border and wrap settings used by both table and card mode.
func newListTable() *table.Table {
	return table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		Wrap(false)
}

// listStyleFunc builds the StyleFunc used by both table and card mode.
// It applies per-column widths and padding, header styling, and cursor
// background for the selected row. If zeroLastPad is true, the last
// column gets zero trailing padding (used in card mode where the
// assignee column sits at the right edge).
func (m *ListModel) listStyleFunc(selectedRow int, zeroLastPad bool) func(int, int) lipgloss.Style {
	cursorBg := m.styles.Cursor.GetBackground()
	headerStyle := m.styles.ColumnHeader

	return func(row, col int) lipgloss.Style {
		padding := m.colPadding(col)
		if zeroLastPad && col == colAssignee {
			padding = 0
		}
		width := m.colWidth(col)

		if row == table.HeaderRow {
			st := headerStyle.PaddingRight(padding)
			if width > 0 {
				st = st.Width(width + padding)
			}
			return st
		}

		st := lipgloss.NewStyle().PaddingRight(padding)
		if width > 0 {
			st = st.Width(width + padding)
		}
		if row == selectedRow {
			st = st.Background(cursorBg)
		}
		return st
	}
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

	listTable := newListTable().
		Headers("ID", urgLabel, "TYPE", "STATUS", ownerLabel, "SUMMARY").
		StyleFunc(m.listStyleFunc(selectedRow, false)).
		Rows(rows...)

	return padToHeight(listTable.Render(), visible+1)
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

	cardTable := newListTable().
		Headers("ID", urgLabel, "TYPE", "STATUS", ownerLabel).
		StyleFunc(m.listStyleFunc(selectedRow, true)).
		Rows(rows...)

	tableLines := strings.Split(cardTable.Render(), "\n")
	if len(tableLines) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(tableLines[0]) // header row

	// Each card: metadata line + summary line (2 lines per item).
	for i := start; i < end; i++ {
		lineIdx := i - start + 1
		metaLine := ""
		if lineIdx < len(tableLines) {
			metaLine = tableLines[lineIdx]
		}
		buf.WriteString("\n" + metaLine)
		buf.WriteString("\n" + m.buildSummaryLine(m.filtered[i], i == m.cursor))
	}

	// Pad to the full height: 1 header + itemsVisible * 2 lines.
	return padToHeight(buf.String(), 1+itemsVisible*2)
}

// ── Column dimensions ─────────────────────────────────────────────

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

// colWidth returns the fixed display width for a column, or 0 for
// columns that should auto-size (summary). Setting Width on the table
// StyleFunc ensures columns stay stable regardless of which items are
// visible on screen.
func (m *ListModel) colWidth(col int) int {
	switch col {
	case colID:
		return m.maxIDW
	case colPrio:
		return listPrioW
	case colType:
		return listTypeW
	case colStatus:
		return listStatusW
	case colAssignee:
		return listAssigneeW
	default:
		return 0
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

// ── Row cell building ─────────────────────────────────────────────

// buildRowCells returns the six pre-styled cells for one list row: ID,
// priority, type, status, assignee, summary (tree prefix + summary body).
// When selected, each styled span has the cursor background baked in so
// lipgloss's row-level background painting in StyleFunc extends cleanly
// across padding spaces between cells.
func (m *ListModel) buildRowCells(item listItem, selected bool) []string {
	styles := m.styles
	issue := item.Issue
	rowWithBackground := m.withBackground(selected)

	// ID — coloured by the issue's type.
	typeColor := styles.TypeColor(issue.Type)
	idLabel := issue.ID
	if issue.DisplayID != "" {
		idLabel = issue.DisplayID
	}
	idCell := rowWithBackground(lipgloss.NewStyle().Foreground(typeColor)).Bold(true).Render(idLabel)

	// Priority icon.
	urgKey := ""
	if def := m.fieldDefs.ByRole(core.RoleUrgency).Primary(); def != nil {
		urgKey = def.Key
	}
	priorityCell := styles.PriorityIconWithBg(issue.StringField(urgKey), selected)

	// Type — prefer the workspace-defined short form when set.
	displayType := issue.Type
	if entry, ok := m.typeOrder[strings.ToLower(issue.Type)]; ok && entry.Short != "" {
		displayType = entry.Short
	}
	typeCell := rowWithBackground(lipgloss.NewStyle().Foreground(typeColor)).Render(displayType)

	// Status — icon + name.
	statusIcon, statusColor := styles.StatusStyle(issue.Status)
	statusCell := rowWithBackground(lipgloss.NewStyle().Foreground(statusColor)).Render(statusIcon + " " + issue.Status)

	// Assignee — falls back to em dash.
	ownerKey := ""
	if def := m.fieldDefs.ByRole(core.RoleOwnership).Primary(); def != nil {
		ownerKey = def.Key
	}
	assignee := issue.DisplayStringField(ownerKey)
	if assignee == "" {
		assignee = core.GlyphEmDash
	}
	assigneeCell := rowWithBackground(lipgloss.NewStyle().Faint(true)).Render(assignee)

	// Summary — tree prefix + styled body + optional child count.
	summaryCell := m.buildSummaryCell(item, issue, selected, rowWithBackground)

	return []string{idCell, priorityCell, typeCell, statusCell, assigneeCell, summaryCell}
}

// buildSummaryCell renders the summary for table mode: tree prefix +
// truncated body + optional "(N sub)" suffix.
func (m *ListModel) buildSummaryCell(item listItem, issue *core.WorkItem, selected bool, rowWithBackground func(lipgloss.Style) lipgloss.Style) string {
	treePrefix := m.renderTreePrefix(item, selected)
	style := m.summaryStyle(issue.Type, selected, rowWithBackground)

	budget := m.summaryBudget() - lipgloss.Width(treePrefix)
	body := issue.Summary
	issueWithNavSuffix := childIssueNavigationSuffix(issue)

	// Reserve space for the child-count suffix before truncating the body.
	bodyBudget := budget - len(issueWithNavSuffix)
	if bodyBudget > 0 && lipgloss.Width(body) > bodyBudget {
		body = ansi.Truncate(body, bodyBudget, "…")
	}

	rendered := style.Render(body)
	if issueWithNavSuffix != "" {
		rendered += rowWithBackground(m.styles.ChildCount).Render(issueWithNavSuffix)
	}
	return treePrefix + rendered
}

// buildSummaryLine renders the card-mode summary line that sits below
// each metadata row. Tree glyph is omitted by design — narrow-width
// layouts drop hierarchy for readability. When selected, the full line
// (including right-padding) carries the cursor background so the card
// reads as a contiguous highlighted block.
func (m *ListModel) buildSummaryLine(item listItem, selected bool) string {
	styles := m.styles
	issue := item.Issue
	rowWithBackground := m.withBackground(selected)
	style := m.summaryStyle(issue.Type, selected, rowWithBackground)

	body := issue.Summary
	issueWithNavSuffix := childIssueNavigationSuffix(issue)
	budget := m.width - cardSummaryIndent - len(issueWithNavSuffix) - cardSummaryTrailPad
	if budget > 0 && lipgloss.Width(body) > budget {
		body = ansi.Truncate(body, budget, "…")
	}

	indent := strings.Repeat(" ", cardSummaryIndent)
	if selected {
		indent = lipgloss.NewStyle().Background(styles.Cursor.GetBackground()).Render(indent)
	}
	line := indent + style.Render(body)
	if issueWithNavSuffix != "" {
		line += rowWithBackground(styles.ChildCount).Render(issueWithNavSuffix)
	}

	// Extend cursor background across the full width for selected rows.
	if selected {
		visibleWidth := lipgloss.Width(line)
		if visibleWidth < m.width {
			line += lipgloss.NewStyle().Background(styles.Cursor.GetBackground()).Render(
				strings.Repeat(" ", m.width-visibleWidth),
			)
		}
	}
	return line
}

// childIssueNavigationSuffix returns " (N sub)" for issues with children, or ""
// if there are none. Extracted so table and card mode stay consistent.
func childIssueNavigationSuffix(issue *core.WorkItem) string {
	if len(issue.Children) > 0 {
		return fmt.Sprintf(" (%d sub)", len(issue.Children))
	}
	return ""
}

// ── Tree prefix rendering ─────────────────────────────────────────

// renderTreePrefix renders the tree prefix with branch glyphs colored by
// the parent's type color, including vertical connection lines. Token
// sequence comes from the pure BuildTreeTokens function; this method
// only maps tokens to styled glyph strings.
func (m *ListModel) renderTreePrefix(item listItem, selected bool) string {
	tokens := BuildTreeTokens(item.Depth, item.IsLast, item.Ancestors)
	if len(tokens) == 0 {
		return ""
	}

	styles := m.styles
	rowWithBackground := m.withBackground(selected)

	var buf strings.Builder
	for i, token := range tokens {
		glyph := token.Glyph()
		switch token {
		case TokenSpace:
			buf.WriteString(rowWithBackground(lipgloss.NewStyle()).Render(glyph))

		case TokenVert:
			// Vertical connector at column i is coloured by the
			// ancestor it belongs to (AncestorTypes[i]).
			var ancestorType string
			if i < len(item.AncestorTypes) {
				ancestorType = item.AncestorTypes[i]
			}
			ancestorColor := styles.TypeColor(ancestorType)
			buf.WriteString(rowWithBackground(lipgloss.NewStyle().Foreground(ancestorColor)).Render(glyph))

		case TokenTee, TokenCorner:
			if item.ParentType != "" {
				parentColor := styles.TypeColor(item.ParentType)
				buf.WriteString(rowWithBackground(lipgloss.NewStyle().Foreground(parentColor)).Render(glyph))
			} else {
				buf.WriteString(rowWithBackground(lipgloss.NewStyle().Faint(true)).Render(glyph))
			}
		}
	}
	return buf.String()
}
