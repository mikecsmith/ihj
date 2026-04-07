package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Layout constants for the detail pane.
const (
	minContentWidth  = 20 // below this, snap to fallbackContentWidth
	fallbackContentW = 60 // used before terminal size is known
	maxWrapWidth     = 90 // soft-wrap body text at this width
	maxDividerWidth  = 64 // horizontal rule capped here

	// Label column constraints for the metadata grid.
	maxLabelWidth = 20 // cap prevents one long label from squashing values
	labelGap      = 2  // spaces between the longest label and its value

	// Value cell padding — matches gridPerColumnOverhead in metadata_grid.go.
	// Each value cell gets this much right padding to separate it from the
	// next label column (or the grid edge).
	valueCellPad = 6

	// Children table column truncation limits.
	maxChildTypeWidth   = 10
	maxChildStatusWidth = 14
)

// Children table column indices.
const (
	childColTree = iota
	childColID
	childColPriority
	childColType
	childColStatus
	childColSummary
	childColHint
)

// Dim placeholder styles used in the scalar grid for empty cells.
var (
	dimDashCentered = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(240)).
			Align(lipgloss.Center)
	dimDashLeft = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(240))
)

// rebuildContent re-renders the entire detail pane into the viewport.
//
// Rendering pipeline:
//  1. Identity line — location › ID › type › status › priority
//  2. Metadata grid — scalar fields in columns + array fields below
//  3. Summary header
//  4. Description body
//  5. Rich-text custom field blocks
//  6. Children table with hint-key navigation
//  7. Comments
func (m *DetailModel) rebuildContent() {
	if m.issue == nil {
		return
	}

	contentWidth := m.width - 2
	if contentWidth < minContentWidth {
		contentWidth = fallbackContentW
	}
	wrapWidth := min(maxWrapWidth, contentWidth)
	divider := m.styles.DetailDivider.Render(
		strings.Repeat(core.GlyphHorizLine, min(contentWidth, maxDividerWidth)),
	)

	var buf strings.Builder
	m.renderIdentityLine(&buf)
	m.renderMetadataSection(&buf, contentWidth)
	buf.WriteString(m.styles.DetailHeader.Render(strings.ToUpper(m.issue.Summary)) + "\n\n")
	m.renderDescription(&buf, wrapWidth)
	m.renderRichTextBlocks(&buf, divider, wrapWidth)
	m.renderChildrenTable(&buf, divider)
	m.renderComments(&buf, divider, wrapWidth)
	m.viewport.SetContent(buf.String())
}

// ── Identity line ───────────────────────────────────────────────────

// renderIdentityLine writes the top breadcrumb: location › ID › type › status › priority.
func (m *DetailModel) renderIdentityLine(buf *strings.Builder) {
	issue := m.issue
	styles := m.styles
	var parts []string

	location := m.ws.Name
	if issue.Location != "" {
		location = issue.Location
	}
	if location != "" {
		parts = append(parts, styles.BoardName.Render(core.IconTeam+strings.ToUpper(location)))
	}

	displayID := issue.ID
	if issue.DisplayID != "" {
		displayID = issue.DisplayID
	}
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render(displayID))

	typeColor := styles.TypeColor(issue.Type)
	parts = append(parts, lipgloss.NewStyle().Foreground(typeColor).Render(core.IconType+strings.ToUpper(issue.Type)))

	statusIcon, statusColor := styles.StatusStyle(issue.Status)
	parts = append(parts, lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon+" "+strings.ToUpper(issue.Status)))

	if urgKey := m.urgencyFieldKey(); urgKey != "" {
		priority := issue.StringField(urgKey)
		parts = append(parts, styles.PriorityIcon(priority)+" "+strings.ToUpper(priority))
	}

	sep := lipgloss.NewStyle().Faint(true).Render(" " + core.GlyphChevron + " ")
	buf.WriteString(strings.Join(parts, sep) + "\n")
}

// ── Description ─────────────────────────────────────────────────────

// renderDescription writes the issue description or a "No description." placeholder.
func (m *DetailModel) renderDescription(buf *strings.Builder, wrapWidth int) {
	placeholder := lipgloss.NewStyle().Faint(true).Italic(true).Render("No description.") + "\n"
	if m.issue.Description == nil {
		buf.WriteString(placeholder)
		return
	}
	desc := strings.TrimSpace(document.RenderANSI(m.issue.Description, document.ANSIConfig{
		WrapWidth: wrapWidth,
		Style:     m.styles.ContentStyle,
	}))
	if desc == "" {
		buf.WriteString(placeholder)
		return
	}
	buf.WriteString(desc + "\n")
}

// ── Children table ──────────────────────────────────────────────────

// renderChildrenTable writes the child issues section with hint keys for navigation.
func (m *DetailModel) renderChildrenTable(buf *strings.Builder, divider string) {
	issue := m.issue
	styles := m.styles
	m.sortedChildren = nil
	if len(issue.Children) == 0 {
		return
	}

	buf.WriteString("\n" + divider + "\n")
	buf.WriteString(styles.ChildSection.Render(core.IconChildren+"CHILD ISSUES") + "\n")

	children := make([]*core.WorkItem, len(issue.Children))
	copy(children, issue.Children)
	sort.Slice(children, func(i, j int) bool {
		return children[i].ID < children[j].ID
	})
	m.sortedChildren = children

	urgKey := m.urgencyFieldKey()
	var rows [][]string
	for idx, child := range children {
		statusIcon, _ := styles.StatusStyle(child.Status)
		childType := child.Type
		if len(childType) > maxChildTypeWidth {
			childType = childType[:maxChildTypeWidth]
		}
		childStatus := child.Status
		if len(childStatus) > maxChildStatusWidth {
			childStatus = childStatus[:maxChildStatusWidth]
		}
		hint := ""
		if idx < len(m.hintKeys) {
			hint = fmt.Sprintf("[%c]", m.hintKeys[idx])
		}
		rows = append(rows, []string{
			core.GlyphReturn,
			child.ID,
			styles.PriorityIcon(child.StringField(urgKey)),
			childType,
			statusIcon + " " + childStatus,
			child.Summary,
			hint,
		})
	}

	// Column layout: tree-glyph | ID | priority | type | status | summary | hint-key
	childTable := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderColumn(false).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			padded := lipgloss.NewStyle().PaddingRight(1)
			if row < 0 || row >= len(children) {
				return padded
			}
			child := children[row]
			typeColor := styles.TypeColor(child.Type)
			_, statusColor := styles.StatusStyle(child.Status)

			switch col {
			case childColTree:
				return styles.TreeGlyph.PaddingRight(1)
			case childColID:
				return lipgloss.NewStyle().Foreground(typeColor).Bold(true).PaddingRight(1)
			case childColType:
				return lipgloss.NewStyle().Foreground(typeColor).PaddingRight(1)
			case childColStatus:
				return lipgloss.NewStyle().Foreground(statusColor).PaddingRight(1)
			case childColHint:
				return lipgloss.NewStyle().Faint(true).PaddingRight(1)
			default:
				return padded
			}
		}).
		Rows(rows...)

	buf.WriteString(childTable.Render() + "\n")
}

// ── Comments ────────────────────────────────────────────────────────

// renderComments writes the last N comments with author, date, and body.
func (m *DetailModel) renderComments(buf *strings.Builder, divider string, wrapWidth int) {
	issue := m.issue
	styles := m.styles
	if len(issue.Comments) == 0 {
		return
	}

	buf.WriteString("\n" + divider + "\n")
	buf.WriteString(styles.CommentSection.Render(core.IconComments+"LATEST COMMENTS") + "\n\n")

	commentSep := lipgloss.NewStyle().Faint(true).Render(
		core.GlyphHorizLine + core.GlyphHorizLine + core.GlyphHorizLine,
	)
	for i, comment := range issue.Comments {
		if i > 0 {
			buf.WriteString("\n" + commentSep + "\n")
		}
		header := styles.CommentAuthor.Render(comment.Author) + "  " +
			styles.CommentDate.Render(core.GlyphDot+" "+comment.Created)
		buf.WriteString(header + "\n")
		if comment.Body != nil {
			body := document.RenderANSI(comment.Body, document.ANSIConfig{
				WrapWidth: wrapWidth,
				Style:     styles.ContentStyle,
			})
			buf.WriteString(strings.Trim(body, "\n") + "\n")
		}
	}
}

// ── Rich-text blocks ────────────────────────────────────────────────

// renderRichTextBlocks emits each rich-text custom field as a divider-separated,
// full-width ANSI block with a section header. Fields that are unset or produce
// empty output are skipped. Only pinned custom fields are shown — unpinned
// createmeta fields are noise.
func (m *DetailModel) renderRichTextBlocks(buf *strings.Builder, divider string, wrapWidth int) {
	issue := m.issue
	styles := m.styles
	for _, def := range m.issueFieldDefs() {
		if def.Type != core.FieldRichText {
			continue
		}
		if def.Role == core.RoleCustom && !def.Pinned {
			continue
		}
		node := issue.RichTextField(def.Key)
		if node == nil {
			continue
		}
		body := strings.TrimSpace(document.RenderANSI(node, document.ANSIConfig{
			WrapWidth: wrapWidth,
			Style:     styles.ContentStyle,
		}))
		if body == "" {
			continue
		}
		buf.WriteString("\n" + divider + "\n")
		buf.WriteString(styles.ChildSection.Render(strings.ToUpper(def.Label)) + "\n\n")
		buf.WriteString(body + "\n")
	}
}

// ── Metadata section ────────────────────────────────────────────────

// metadataRoleOrder defines the role rendering sequence for the detail pane.
var metadataRoleOrder = []core.FieldRole{
	core.RoleOwnership,
	core.RoleTemporal,
	core.RoleIteration,
	core.RoleParent,
	core.RoleCategorisation,
	core.RoleCustom,
}

// metadataEntry holds a field definition and its display value for the
// metadata grid. A nil def pointer marks an empty cell.
type metadataEntry struct {
	def *core.FieldDef
	val string
}

// renderMetadataSection writes the unified metadata block: a column-adaptive
// scalar grid followed by full-width array rows, wrapped in top/bottom borders.
func (m *DetailModel) renderMetadataSection(buf *strings.Builder, contentWidth int) {
	scalarGroups, arrayEntries := m.collectMetadataEntries()
	grid, maxValueWidths, labelWidths := m.computeGridLayout(scalarGroups, arrayEntries, contentWidth)

	var section strings.Builder

	if len(grid.Rows) > 0 {
		section.WriteString(m.renderScalarGrid(grid, labelWidths, maxValueWidths) + "\n")
	}
	section.WriteString(m.renderArrayFields(arrayEntries, labelWidths[0], contentWidth))

	if section.Len() == 0 {
		return
	}

	borderedMetadata := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(lipgloss.ANSIColor(8)).
		Width(contentWidth).
		Render(strings.TrimRight(section.String(), "\n"))
	buf.WriteString(borderedMetadata + "\n")
}

// collectMetadataEntries walks metadataRoleOrder and splits every visible
// field into scalar groups (for the column grid) and array entries (rendered
// as full-width rows below the grid).
func (m *DetailModel) collectMetadataEntries() (scalarGroups [][]metadataEntry, arrayEntries []metadataEntry) {
	issue := m.issue

	type roleGroup struct {
		scalars []metadataEntry
		arrays  []metadataEntry
	}
	groups := make([]roleGroup, 0, len(metadataRoleOrder))

	for _, role := range metadataRoleOrder {
		var group roleGroup

		// Parent is synthetic — not stored in FieldDefs.
		if role == core.RoleParent {
			def := core.FieldDef{
				Key: "parent", Label: "Parent", Icon: core.IconParent,
				Role: core.RoleParent, Primary: true,
			}
			group.scalars = append(group.scalars, metadataEntry{def: &def, val: issue.ParentID})
			groups = append(groups, group)
			continue
		}

		defs := m.visibleFieldsByRole(role)
		for i := range defs {
			// Rich text fields render in their own full-width block below
			// the description, not in the scalar/array metadata grid.
			if defs[i].Type == core.FieldRichText {
				continue
			}
			// Only show custom fields the user explicitly pinned. Unpinned
			// createmeta fields are noise — Jira scopes them broadly.
			if role == core.RoleCustom && !defs[i].Pinned {
				continue
			}
			entry := metadataEntry{
				def: &defs[i],
				val: issue.DisplayStringField(defs[i].Key),
			}
			if defs[i].Type == core.FieldStringArray {
				group.arrays = append(group.arrays, entry)
			} else {
				group.scalars = append(group.scalars, entry)
			}
		}
		if len(group.scalars) > 0 || len(group.arrays) > 0 {
			groups = append(groups, group)
		}
	}

	for _, group := range groups {
		if len(group.scalars) > 0 {
			scalarGroups = append(scalarGroups, group.scalars)
		}
		arrayEntries = append(arrayEntries, group.arrays...)
	}
	return scalarGroups, arrayEntries
}

// computeGridLayout selects the column count that fits contentWidth, builds
// the grid, and returns per-column label and value widths.
func (m *DetailModel) computeGridLayout(
	scalarGroups [][]metadataEntry,
	arrayEntries []metadataEntry,
	contentWidth int,
) (metadataGrid, []int, []int) {
	// Approximate label width for column-count selection. The exact
	// per-column widths are computed from the final grid below.
	widestScalarLabel := 0
	for _, group := range scalarGroups {
		for _, entry := range group {
			if w := lipgloss.Width(m.fieldLabel(*entry.def)); w > widestScalarLabel {
				widestScalarLabel = w
			}
		}
	}
	approxLabelWidth := min(widestScalarLabel, maxLabelWidth) + labelGap

	grid, maxValueWidths := ChooseMetadataCols(scalarGroups, approxLabelWidth, contentWidth)
	labelWidths := m.perColumnLabelWidths(grid, arrayEntries)

	return grid, maxValueWidths, labelWidths
}

// perColumnLabelWidths computes the label column width for each grid column.
// Column 0 also considers non-empty array labels so that array rows below
// the grid align with grid column 0. Each column's width is the longest
// label in that column plus labelGap, capped at maxLabelWidth + labelGap.
func (m *DetailModel) perColumnLabelWidths(grid metadataGrid, arrayEntries []metadataEntry) []int {
	widths := make([]int, grid.Cols)
	for _, row := range grid.Rows {
		for col, cell := range row {
			if cell.Def == nil {
				continue
			}
			if w := lipgloss.Width(m.fieldLabel(*cell.Def)); w > widths[col] {
				widths[col] = w
			}
		}
	}
	// Array rows render below the grid using column 0's label width.
	for _, entry := range arrayEntries {
		if entry.val == "" {
			continue
		}
		if w := lipgloss.Width(m.fieldLabel(*entry.def)); w > widths[0] {
			widths[0] = w
		}
	}
	for col := range widths {
		widths[col] = min(widths[col], maxLabelWidth) + labelGap
	}
	return widths
}

// renderArrayFields renders array-type fields (Labels, Components, etc.) as
// full-width rows that align with grid column 0.
func (m *DetailModel) renderArrayFields(entries []metadataEntry, labelWidth, contentWidth int) string {
	styles := m.styles
	var out strings.Builder
	for _, entry := range entries {
		if entry.val == "" {
			continue
		}
		label := styles.MetadataLabelStyle(entry.def.Role, entry.def.Primary, 0).
			Width(labelWidth).
			Render(m.fieldLabel(*entry.def))
		valueWidth := max(contentWidth-labelWidth, 1)
		value := styles.DetailValue.Width(valueWidth).Render(entry.val)
		out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, label, value) + "\n")
	}
	return out.String()
}

// ── Scalar grid ─────────────────────────────────────────────────────

// renderScalarGrid renders the metadata grid as a borderless lipgloss table.
// Each grid row becomes a table row with cols×2 cells (label+value pairs).
// Empty cells render as dimmed em dashes — left-aligned in label columns,
// centered in value columns. Trailing empty columns in the last row are
// blanked so placeholder dashes don't sit next to arrays or the divider.
func (m *DetailModel) renderScalarGrid(grid metadataGrid, labelWidths []int, maxValueWidths []int) string {
	styles := m.styles
	cols := grid.Cols
	cellsPerRow := cols * 2

	// Find the rightmost populated column in the last row so we can blank
	// trailing placeholder cells.
	trailingEmptyFrom := cols
	if len(grid.Rows) > 0 {
		lastRow := grid.Rows[len(grid.Rows)-1]
		for col := cols - 1; col >= 0; col-- {
			if lastRow[col].Def != nil {
				break
			}
			trailingEmptyFrom = col
		}
	}

	tableRows := make([][]string, len(grid.Rows))
	for rowIdx, row := range grid.Rows {
		cells := make([]string, cellsPerRow)
		isLastRow := rowIdx == len(grid.Rows)-1
		for col, entry := range row {
			labelIdx := col * 2
			valueIdx := col*2 + 1
			if isLastRow && col >= trailingEmptyFrom {
				cells[labelIdx] = ""
				cells[valueIdx] = ""
				continue
			}
			if entry.Def == nil {
				cells[labelIdx] = core.GlyphEmDash
				cells[valueIdx] = core.GlyphEmDash
				continue
			}
			cells[labelIdx] = m.fieldLabel(*entry.Def)
			if entry.Val == "" {
				cells[valueIdx] = core.GlyphEmDash
			} else {
				cells[valueIdx] = entry.Val
			}
		}
		tableRows[rowIdx] = cells
	}

	gridTable := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row < 0 || row >= len(grid.Rows) {
				return lipgloss.NewStyle()
			}
			gridCol := col / 2
			if gridCol >= len(grid.Rows[row]) {
				return lipgloss.NewStyle()
			}
			cell := grid.Rows[row][gridCol]
			valueWidth := maxValueWidths[gridCol] + valueCellPad
			isLabel := col%2 == 0

			// Last-row trailing blank columns: no styling.
			if row == len(grid.Rows)-1 && gridCol >= trailingEmptyFrom {
				return lipgloss.NewStyle()
			}

			// Empty cell — label em dash left-aligned, value em dash centered.
			if cell.Def == nil {
				if isLabel {
					return dimDashLeft.Width(labelWidths[gridCol])
				}
				return dimDashCentered.Width(valueWidth).PaddingRight(valueCellPad)
			}

			// Label cell.
			if isLabel {
				return styles.MetadataLabelStyle(cell.Def.Role, cell.Def.Primary, 0).
					Width(labelWidths[gridCol])
			}

			// Value cell — em dash centered when empty.
			if cell.Val == "" {
				return dimDashCentered.Width(valueWidth).PaddingRight(valueCellPad)
			}
			return styles.DetailValue.Width(valueWidth)
		}).
		Rows(tableRows...)

	return gridTable.Render()
}

// ── Helpers ─────────────────────────────────────────────────────────

// fieldLabel formats a FieldDef's icon and label into a display string
// like "◉ Assignee:" for use in the metadata grid label cells.
func (m *DetailModel) fieldLabel(def core.FieldDef) string {
	icon := def.Icon
	if icon == "" && def.Role == core.RoleCustom {
		icon = core.IconField
	}
	label := def.Label + ":"
	if icon != "" {
		return icon + label
	}
	return " " + label
}

func (m *DetailModel) renderEmpty() string {
	msg := lipgloss.NewStyle().Faint(true).Render("Select an issue to view details")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}
