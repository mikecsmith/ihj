package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikecsmith/ihj/internal/core"
)

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

	visibleLines := max(
		// reserve the header row
		contentH-1, 1)
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

// ── Row rendering ─────────────────────────────────────────────────

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

	// Type — prefer the workspace-defined short form when set.
	typeName := iss.Type
	if entry, ok := m.typeOrder[strings.ToLower(iss.Type)]; ok && entry.Short != "" {
		typeName = entry.Short
	}
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
