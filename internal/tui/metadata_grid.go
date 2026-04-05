package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

// gridPerColumnOverhead is the fixed chrome charged to each column beyond
// the label+value cell contents: single-char gap after label (already
// baked into scalarLabelColW) plus a trailing 6-cell pad (value right
// margin + column separator). Centralising it here keeps the grid
// breakpoint math in one place.
const gridPerColumnOverhead = 6

// GridRequiredWidth returns the total width the grid requires to render
// without truncation, given the shared scalar label-column width. It
// also returns the per-column max value width used to compute that
// total, which the renderer needs for alignment.
//
// Guarantees (see metadata_grid_test.go):
//   - len(maxValW) == grid.Cols.
//   - required == sum over c of (scalarLabelColW + maxValW[c] + gridPerColumnOverhead).
//   - maxValW[c] is the widest lipgloss.Width of any non-empty cell in column c.
func GridRequiredWidth(grid metadataGrid, scalarLabelColW int) (required int, maxValW []int) {
	maxValW = make([]int, grid.Cols)
	for _, row := range grid.Rows {
		for c, cell := range row {
			if cell.Def == nil || cell.Val == "" {
				continue
			}
			if w := lipgloss.Width(cell.Val); w > maxValW[c] {
				maxValW[c] = w
			}
		}
	}
	for c := 0; c < grid.Cols; c++ {
		required += scalarLabelColW + maxValW[c] + gridPerColumnOverhead
	}
	return required, maxValW
}

// ChooseMetadataCols picks the largest column count in {3, 2, 1} whose
// rendered grid fits in contentWidth, and returns the chosen grid plus
// its per-column max value widths. When no column count fits, the
// single-column grid is returned (the minimum).
//
// Guarantees (see metadata_grid_test.go):
//   - Returned grid.Cols is 3, 2, or 1.
//   - If grid.Cols > 1, GridRequiredWidth(grid, scalarLabelColW) <= contentWidth.
//   - If any of the {3,2} column grids fits, the returned grid.Cols is
//     the largest such count.
func ChooseMetadataCols(scalarGroups [][]metadataEntry, scalarLabelColW, contentWidth int) (metadataGrid, []int) {
	var grid metadataGrid
	var maxValW []int
	for cols := 3; cols >= 1; cols-- {
		g := buildMetadataGrid(scalarGroups, cols)
		required, mv := GridRequiredWidth(g, scalarLabelColW)
		if required <= contentWidth || cols == 1 {
			grid = g
			maxValW = mv
			break
		}
	}
	return grid, maxValW
}

// metadataCell is one cell in the metadata grid. A nil Def marks an empty
// placeholder that the renderer will fill with an em dash.
type metadataCell struct {
	Def *core.FieldDef
	Val string
}

// metadataGrid is a row-major layout of metadata cells. Each row has exactly
// Cols cells. Cell at index c corresponds to the c-th visible column.
type metadataGrid struct {
	Rows [][]metadataCell
	Cols int
}

// buildMetadataGrid lays out role-ordered scalar groups into a grid.
//
// Groups are consumed in batches of `cols`; each group in a batch occupies
// one column for its length, so semantically related fields stay vertically
// aligned. Two compaction passes then tidy the result:
//
//   - leftward collapse: only RoleCustom cells shift into empty slots on
//     their left. Other roles keep their column so paired groups (e.g.
//     Ownership | Temporal) stay aligned.
//   - upward collapse: a cell whose column does NOT already contain an
//     earlier cell of the same role (i.e. it is not part of a column chain)
//     may move into the first empty slot in an earlier row. This lifts
//     orphan singles (Parent, lone customs) up without ripping multi-row
//     role chains apart.
//
// Finally, trailing rows that are entirely empty are trimmed.
func buildMetadataGrid(scalarGroups [][]metadataEntry, cols int) metadataGrid {
	if cols < 1 {
		cols = 1
	}
	var rows [][]metadataCell

	for gi := 0; gi < len(scalarGroups); gi += cols {
		end := gi + cols
		if end > len(scalarGroups) {
			end = len(scalarGroups)
		}
		batch := scalarGroups[gi:end]

		maxRows := 0
		for _, grp := range batch {
			if len(grp) > maxRows {
				maxRows = len(grp)
			}
		}

		for r := 0; r < maxRows; r++ {
			row := make([]metadataCell, cols)
			for c, grp := range batch {
				if r < len(grp) {
					row[c] = metadataCell{Def: grp[r].def, Val: grp[r].val}
				}
			}
			row = collapseLeftCustom(row)
			rows = append(rows, row)
		}
	}

	rows = collapseUp(rows, cols)
	rows = trimTrailingEmpty(rows)

	return metadataGrid{Rows: rows, Cols: cols}
}

// collapseLeftCustom shifts RoleCustom cells into empty slots on their left
// within a single row. Non-custom cells stay put so semantic column pairings
// remain intact.
func collapseLeftCustom(row []metadataCell) []metadataCell {
	for c := 0; c < len(row)-1; c++ {
		if row[c].Def != nil {
			continue
		}
		for nc := c + 1; nc < len(row); nc++ {
			if row[nc].Def == nil {
				continue
			}
			if row[nc].Def.Role != core.RoleCustom {
				continue
			}
			row[c] = row[nc]
			row[nc] = metadataCell{}
			break
		}
	}
	return row
}

// collapseUp lifts orphan cells into empty slots in earlier rows. An orphan
// is a cell whose column does NOT contain an earlier cell of the same role
// — i.e. it is not the continuation of a role chain. This preserves multi-
// row groups (e.g. three temporal fields in one column) while compacting
// single-field groups (Parent, lone customs) upward.
func collapseUp(rows [][]metadataCell, cols int) [][]metadataCell {
	for i := 1; i < len(rows); i++ {
		for c := 0; c < cols; c++ {
			cell := rows[i][c]
			if cell.Def == nil {
				continue
			}
			if inColumnChain(rows, i, c, cell.Def.Role) {
				continue
			}
			moved := false
			for j := 0; j < i && !moved; j++ {
				for nc := 0; nc < cols; nc++ {
					if rows[j][nc].Def != nil {
						continue
					}
					rows[j][nc] = rows[i][c]
					rows[i][c] = metadataCell{}
					moved = true
					break
				}
			}
		}
	}
	return rows
}

// inColumnChain reports whether column c has a cell with the same role in
// any row before row i. If so, the cell at (i,c) is part of a role chain
// and must not be moved out of its column.
func inColumnChain(rows [][]metadataCell, i, c int, role core.FieldRole) bool {
	for k := 0; k < i; k++ {
		if prev := rows[k][c].Def; prev != nil && prev.Role == role {
			return true
		}
	}
	return false
}

// trimTrailingEmpty drops trailing rows in which every cell is empty.
func trimTrailingEmpty(rows [][]metadataCell) [][]metadataCell {
	for len(rows) > 0 {
		last := rows[len(rows)-1]
		allEmpty := true
		for _, cell := range last {
			if cell.Def != nil {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			break
		}
		rows = rows[:len(rows)-1]
	}
	return rows
}
