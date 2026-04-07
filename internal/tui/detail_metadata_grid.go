package tui

import (
	"sort"

	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

// ── Types ───────────────────────────────────────────────────────────

// metadataCell is one cell in the metadata grid. A nil Def marks an
// empty placeholder that the renderer fills with an em dash.
type metadataCell struct {
	Def *core.FieldDef
	Val string
}

// metadataGrid is a row-major layout of metadata cells. Each row has
// exactly Cols cells; cell at index c is the c-th visible column.
type metadataGrid struct {
	Rows [][]metadataCell
	Cols int
}

// ── Width measurement ───────────────────────────────────────────────

// GridRequiredWidth returns the total width the grid needs to render
// without truncation, given a shared label column width. It also
// returns the per-column widest value width, which the renderer uses
// for alignment.
//
// Guarantees (see metadata_grid_test.go):
//   - len(maxValueWidths) == grid.Cols
//   - required == sum over col of (labelColWidth + maxValueWidths[col] + metadataColumnGap)
//   - maxValueWidths[col] is the widest lipgloss.Width of any non-empty cell in that column
func GridRequiredWidth(grid metadataGrid, labelColWidth int) (required int, maxValueWidths []int) {
	maxValueWidths = make([]int, grid.Cols)
	for _, row := range grid.Rows {
		for col, cell := range row {
			if cell.Def == nil || cell.Val == "" {
				continue
			}
			if width := lipgloss.Width(cell.Val); width > maxValueWidths[col] {
				maxValueWidths[col] = width
			}
		}
	}
	for col := range grid.Cols {
		required += labelColWidth + maxValueWidths[col] + metadataColumnGap
	}
	return required, maxValueWidths
}

// ChooseMetadataCols picks the largest column count in {3, 2, 1} whose
// rendered grid fits within contentWidth, returning the chosen grid and
// its per-column max value widths. Falls back to single-column when
// nothing else fits.
//
// Guarantees (see metadata_grid_test.go):
//   - Returned grid.Cols is 3, 2, or 1
//   - If grid.Cols > 1, GridRequiredWidth(grid, labelColWidth) <= contentWidth
//   - The largest fitting column count is always chosen
func ChooseMetadataCols(scalarGroups [][]metadataEntry, labelColWidth, contentWidth int) (metadataGrid, []int) {
	for cols := 3; cols >= 2; cols-- {
		grid := buildMetadataGrid(scalarGroups, cols)
		required, maxValueWidths := GridRequiredWidth(grid, labelColWidth)
		if required <= contentWidth {
			return grid, maxValueWidths
		}
	}
	// Single column always fits — it's the minimum layout.
	grid := buildMetadataGrid(scalarGroups, 1)
	_, maxValueWidths := GridRequiredWidth(grid, labelColWidth)
	return grid, maxValueWidths
}

// ── Grid construction ───────────────────────────────────────────────

// buildMetadataGrid lays out role-ordered scalar groups into a grid.
//
// The algorithm has three phases:
//  1. Separate custom fields from non-custom groups. Non-custom groups
//     are batched by column count, keeping semantically related fields
//     (Ownership, Temporal) vertically aligned.
//  2. Fill empty slots in the non-custom layout with custom fields,
//     packing them top-to-bottom, left-to-right.
//  3. Append any remaining custom fields as new rows.
func buildMetadataGrid(scalarGroups [][]metadataEntry, cols int) metadataGrid {
	if cols < 1 {
		cols = 1
	}

	customCells, nonCustomGroups := separateCustomFields(scalarGroups)
	rows := layoutNonCustomGroups(nonCustomGroups, cols)
	rows = removeEmptyRows(rows)
	rows = fillCustomFields(rows, customCells, cols)
	rows = removeEmptyRows(rows)

	return metadataGrid{Rows: rows, Cols: cols}
}

// separateCustomFields splits scalar groups into individual custom field
// cells (sorted by key for stable ordering) and non-custom groups with
// custom entries removed.
func separateCustomFields(scalarGroups [][]metadataEntry) ([]metadataCell, [][]metadataEntry) {
	var customCells []metadataCell
	var nonCustomGroups [][]metadataEntry

	for _, group := range scalarGroups {
		var kept []metadataEntry
		for _, entry := range group {
			if entry.def.Role == core.RoleCustom {
				customCells = append(customCells, metadataCell{Def: entry.def, Val: entry.val})
			} else {
				kept = append(kept, entry)
			}
		}
		if len(kept) > 0 {
			nonCustomGroups = append(nonCustomGroups, kept)
		}
	}

	sort.Slice(customCells, func(i, j int) bool {
		return customCells[i].Def.Key < customCells[j].Def.Key
	})

	return customCells, nonCustomGroups
}

// layoutNonCustomGroups arranges non-custom field groups into grid rows.
// Groups are consumed in batches of `cols`; each group in a batch
// occupies one column for its full length, so semantically related
// fields stay vertically aligned.
func layoutNonCustomGroups(groups [][]metadataEntry, cols int) [][]metadataCell {
	var rows [][]metadataCell

	for batchStart := 0; batchStart < len(groups); batchStart += cols {
		batchEnd := min(batchStart+cols, len(groups))
		batch := groups[batchStart:batchEnd]

		tallestGroup := 0
		for _, group := range batch {
			if len(group) > tallestGroup {
				tallestGroup = len(group)
			}
		}

		for rowIdx := range tallestGroup {
			row := make([]metadataCell, cols)
			for col, group := range batch {
				if rowIdx < len(group) {
					row[col] = metadataCell{Def: group[rowIdx].def, Val: group[rowIdx].val}
				}
			}
			rows = append(rows, row)
		}
	}

	return rows
}

// fillCustomFields greedily places custom cells into empty slots in
// existing rows (top-to-bottom, left-to-right), then appends new rows
// for any remaining custom fields.
func fillCustomFields(rows [][]metadataCell, customCells []metadataCell, cols int) [][]metadataCell {
	customIdx := 0

	// Fill empty slots in existing rows.
	for rowIdx := range rows {
		if customIdx >= len(customCells) {
			break
		}
		for col := range rows[rowIdx] {
			if customIdx >= len(customCells) {
				break
			}
			if rows[rowIdx][col].Def == nil {
				rows[rowIdx][col] = customCells[customIdx]
				customIdx++
			}
		}
	}

	// Append remaining custom fields as new rows.
	for customIdx < len(customCells) {
		row := make([]metadataCell, cols)
		for col := range row {
			if customIdx >= len(customCells) {
				break
			}
			row[col] = customCells[customIdx]
			customIdx++
		}
		rows = append(rows, row)
	}

	return rows
}

// ── Helpers ─────────────────────────────────────────────────────────

// removeEmptyRows drops any row where every cell is empty (nil Def).
func removeEmptyRows(rows [][]metadataCell) [][]metadataCell {
	writeIdx := 0
	for _, row := range rows {
		hasContent := false
		for _, cell := range row {
			if cell.Def != nil {
				hasContent = true
				break
			}
		}
		if hasContent {
			rows[writeIdx] = row
			writeIdx++
		}
	}
	return rows[:writeIdx]
}
