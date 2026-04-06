package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/terminal"
)

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

	location := m.ws.Name
	if iss.Location != "" {
		location = iss.Location
	}
	if location != "" {
		teamStr := s.BoardName.Render(core.IconTeam + strings.ToUpper(location))
		parts = append(parts, teamStr)
	}

	idLabel := iss.ID
	if iss.DisplayID != "" {
		idLabel = iss.DisplayID
	}
	keyStr := lipgloss.NewStyle().Bold(true).Render(idLabel)
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

	// Metadata — lipgloss table grid for scalars + arrays.
	m.renderMetadataBlocks(&b, iss, s, contentWidth)

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

	// Divider for subsections (child issues, comments, rich text).
	divider := s.DetailDivider.Render(strings.Repeat(core.GlyphHorizLine, min(contentWidth, 64)))

	// Rich text custom fields (e.g. Acceptance Criteria) — rendered as
	// full-width AST blocks between description and child issues.
	m.renderRichTextBlocks(&b, iss, s, divider, wrapWidth)

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

		urgKey := m.urgencyFieldKey()
		var childRows [][]string
		for idx, child := range sortedChildren {
			icon, _ := s.StatusStyle(child.Status)
			childType := child.Type
			if len(childType) > 10 {
				childType = childType[:10]
			}
			childStatus := child.Status
			if len(childStatus) > 14 {
				childStatus = childStatus[:14]
			}
			hint := ""
			if idx < len(m.hintKeys) {
				hint = fmt.Sprintf("[%c]", m.hintKeys[idx])
			}
			childRows = append(childRows, []string{
				core.GlyphReturn,
				child.ID,
				s.PriorityIcon(child.StringField(urgKey)),
				childType,
				icon + " " + childStatus,
				child.Summary,
				hint,
			})
		}

		ct := table.New().
			Border(lipgloss.HiddenBorder()).
			BorderColumn(false).
			BorderHeader(false).
			StyleFunc(func(row, col int) lipgloss.Style {
				// PaddingRight(1) gives a single space between columns; the
				// last (hint) column's trailing pad is harmless.
				pad := lipgloss.NewStyle().PaddingRight(1)
				if row < 0 || row >= len(sortedChildren) {
					return pad
				}
				child := sortedChildren[row]
				typeClr := s.TypeColor(child.Type)
				_, statusClr := s.StatusStyle(child.Status)

				switch col {
				case 0: // tree glyph
					return s.TreeGlyph.PaddingRight(1)
				case 1: // ID
					return lipgloss.NewStyle().Foreground(typeClr).Bold(true).PaddingRight(1)
				case 3: // type
					return lipgloss.NewStyle().Foreground(typeClr).PaddingRight(1)
				case 4: // status
					return lipgloss.NewStyle().Foreground(statusClr).PaddingRight(1)
				case 6: // hint key
					return lipgloss.NewStyle().Faint(true).PaddingRight(1)
				default:
					return pad
				}
			}).
			Rows(childRows...)

		b.WriteString(ct.Render() + "\n")
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

// renderRichTextBlocks emits each rich-text custom field as a divider-
// separated, full-width ANSI block with a section header labelled with
// the field's label. Fields that are unset on this issue are skipped.
func (m *DetailModel) renderRichTextBlocks(b *strings.Builder, iss *core.WorkItem, s *terminal.Styles, divider string, wrapWidth int) {
	for _, def := range m.issueFieldDefs() {
		if def.Type != core.FieldRichText {
			continue
		}
		// Only show rich text blocks for fields the user explicitly opted
		// into (Pinned). Unpinned custom fields from createmeta are noise —
		// Jira scopes custom fields broadly across all types and often
		// populates them with default/template content.
		if def.Role == core.RoleCustom && !def.Pinned {
			continue
		}
		node := iss.RichTextField(def.Key)
		if node == nil {
			continue
		}
		body := strings.TrimSpace(document.RenderANSI(node, document.ANSIConfig{
			WrapWidth: wrapWidth,
			Style:     s.ContentStyle,
		}))
		if body == "" {
			continue
		}
		b.WriteString("\n" + divider + "\n")
		b.WriteString(s.ChildSection.Render(strings.ToUpper(def.Label)) + "\n\n")
		b.WriteString(body + "\n")
	}
}

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

// renderMetadataBlocks writes the unified metadata section using a
// lipgloss/v2/table grid for scalar fields and a second table for array
// fields. The number of visible columns adapts to terminal width.
func (m *DetailModel) renderMetadataBlocks(b *strings.Builder, iss *core.WorkItem, s *terminal.Styles, contentWidth int) {
	// ── Collect entries grouped by role ──────────────────────────────
	type roleGroup struct {
		scalars []metadataEntry
		arrays  []metadataEntry
	}
	groups := make([]roleGroup, 0, len(metadataRoleOrder))

	for _, role := range metadataRoleOrder {
		var g roleGroup

		if role == core.RoleParent {
			def := core.FieldDef{
				Key: "parent", Label: "Parent", Icon: core.IconParent,
				Role: core.RoleParent, Primary: true,
			}
			g.scalars = append(g.scalars, metadataEntry{def: &def, val: iss.ParentID})
			groups = append(groups, g)
			continue
		}

		defs := m.visibleFieldsByRole(role)
		for i := range defs {
			// Rich text fields render in their own full-width block below
			// the description, not in the scalar/array metadata grid.
			if defs[i].Type == core.FieldRichText {
				continue
			}
			// Custom fields from createmeta are noisy — Jira scopes them
			// broadly across all types. Only show custom fields the user
			// explicitly opted into via config (Pinned).
			if role == core.RoleCustom && !defs[i].Pinned {
				continue
			}
			val := iss.DisplayStringField(defs[i].Key)
			e := metadataEntry{def: &defs[i], val: val}
			if defs[i].Type == core.FieldStringArray {
				g.arrays = append(g.arrays, e)
			} else {
				g.scalars = append(g.scalars, e)
			}
		}
		if len(g.scalars) > 0 || len(g.arrays) > 0 {
			groups = append(groups, g)
		}
	}

	// ── Build scalar/array groups ───────────────────────────────────
	var scalarGroups [][]metadataEntry
	var allArrays []metadataEntry
	for _, g := range groups {
		if len(g.scalars) > 0 {
			scalarGroups = append(scalarGroups, g.scalars)
		}
		allArrays = append(allArrays, g.arrays...)
	}

	// Precompute the scalar label column width across all scalar fields.
	// This is independent of column count — every label cell in the grid
	// is padded to this width.
	scalarMaxLabelW := 0
	for _, grp := range scalarGroups {
		for _, e := range grp {
			if w := lipgloss.Width(m.fieldLabel(*e.def)); w > scalarMaxLabelW {
				scalarMaxLabelW = w
			}
		}
	}
	scalarLabelColW := scalarMaxLabelW + 1 // single char gap after label

	// Dynamic column selection: try 3 cols, drop to 2 or 1 if the grid
	// doesn't fit in contentWidth. See ChooseMetadataCols for the
	// breakpoint math and its rule-based tests.
	grid, maxValW := ChooseMetadataCols(scalarGroups, scalarLabelColW, contentWidth)

	// Array fields use their own label column width so long array labels
	// (like "Components:") don't inflate the scalar grid.
	labelColW := scalarLabelColW
	for _, e := range allArrays {
		if w := lipgloss.Width(m.fieldLabel(*e.def)) + 1; w > labelColW {
			labelColW = w
		}
	}

	// ── Render metadata into a local buffer, then wrap with border-bottom ──
	var metaBuf strings.Builder

	if len(grid.Rows) > 0 {
		metaBuf.WriteString(m.renderScalarGrid(grid, s, scalarLabelColW, maxValW) + "\n")
	}

	// ── Array fields — appended as full-width rows below scalars ────
	for _, e := range allArrays {
		if e.val == "" {
			continue // skip empty arrays entirely
		}
		lbl := s.MetadataLabelStyle(e.def.Role, e.def.Primary, 0).
			Width(labelColW).
			Render(m.fieldLabel(*e.def))
		valW := contentWidth - labelColW
		if valW < 1 {
			valW = 1
		}
		val := s.DetailValue.Width(valW).Render(e.val)
		metaBuf.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lbl, val) + "\n")
	}

	// Wrap entire metadata block with top + bottom borders.
	if metaBuf.Len() > 0 {
		metaBlock := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(false).
			BorderRight(false).
			BorderForeground(lipgloss.ANSIColor(8)).
			Width(contentWidth).
			Render(strings.TrimRight(metaBuf.String(), "\n"))
		b.WriteString(metaBlock + "\n")
	}
}

// renderScalarGrid renders the metadata grid as a borderless lipgloss table.
// Each grid row becomes a table row with cols*2 cells (label+value pairs).
// Empty cells render as dimmed em dashes — left-aligned in label columns,
// centered in value columns. Trailing empty columns in the last row are
// blanked so placeholder em dashes don't sit next to arrays or the divider.
func (m *DetailModel) renderScalarGrid(grid metadataGrid, s *terminal.Styles, labelColW int, maxValW []int) string {
	cols := grid.Cols
	cellsPerRow := cols * 2

	// Find the rightmost populated column in the last row so we can blank
	// trailing placeholder cells.
	trailingEmptyFrom := cols
	if len(grid.Rows) > 0 {
		last := grid.Rows[len(grid.Rows)-1]
		for c := cols - 1; c >= 0; c-- {
			if last[c].Def != nil {
				break
			}
			trailingEmptyFrom = c
		}
	}

	tableRows := make([][]string, len(grid.Rows))
	for ri, row := range grid.Rows {
		tr := make([]string, cellsPerRow)
		isLast := ri == len(grid.Rows)-1
		for c, cell := range row {
			labelIdx := c * 2
			valIdx := c*2 + 1
			if isLast && c >= trailingEmptyFrom {
				tr[labelIdx] = ""
				tr[valIdx] = ""
				continue
			}
			if cell.Def == nil {
				tr[labelIdx] = core.GlyphEmDash
				tr[valIdx] = core.GlyphEmDash
				continue
			}
			tr[labelIdx] = m.fieldLabel(*cell.Def)
			if cell.Val == "" {
				tr[valIdx] = core.GlyphEmDash
			} else {
				tr[valIdx] = cell.Val
			}
		}
		tableRows[ri] = tr
	}

	dimDashCentered := lipgloss.NewStyle().
		Foreground(lipgloss.ANSIColor(240)).
		Align(lipgloss.Center)
	dimDashLeft := lipgloss.NewStyle().
		Foreground(lipgloss.ANSIColor(240))

	t := table.New().
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
			cells := grid.Rows[row]
			colPos := col / 2
			if colPos >= len(cells) {
				return lipgloss.NewStyle()
			}
			cell := cells[colPos]
			valW := maxValW[colPos] + 6

			// Last-row trailing blank columns: no styling.
			isLast := row == len(grid.Rows)-1
			if isLast && colPos >= trailingEmptyFrom {
				return lipgloss.NewStyle()
			}

			// Empty cell — label em dash left-aligned, value em dash centered.
			if cell.Def == nil {
				if col%2 == 0 {
					return dimDashLeft.Width(labelColW)
				}
				return dimDashCentered.Width(valW).PaddingRight(6)
			}

			// Label cell.
			if col%2 == 0 {
				return s.MetadataLabelStyle(cell.Def.Role, cell.Def.Primary, 0).
					Width(labelColW)
			}

			// Value cell — em dash centered when empty.
			if cell.Val == "" {
				return dimDashCentered.Width(valW).PaddingRight(6)
			}
			return s.DetailValue.Width(valW)
		}).
		Rows(tableRows...)

	return t.Render()
}

// fieldLabel formats a FieldDef's icon and label into a string suitable
// for the metadata grid label cell.
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
