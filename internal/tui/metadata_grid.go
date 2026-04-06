package tui

import (
	"sort"

	"charm.land/lipgloss/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

// gridPerColumnOverhead is the fixed padding to each column beyond
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
// one column for its length, so semantically related fields (Ownership,
// Temporal) stay vertically aligned. After the initial layout, all
// RoleCustom cells are collected and greedily placed into empty slots
// top-to-bottom, left-to-right — filling any gaps left by uneven group
// sizes. Empty rows are then removed.
func buildMetadataGrid(scalarGroups [][]metadataEntry, cols int) metadataGrid {
	if cols < 1 {
		cols = 1
	}
	var rows [][]metadataCell

	// Phase 1: collect customs in input order, lay out non-customs.
	var customs []metadataCell
	var nonCustomGroups [][]metadataEntry
	for _, grp := range scalarGroups {
		var kept []metadataEntry
		for _, e := range grp {
			if e.def.Role == core.RoleCustom {
				customs = append(customs, metadataCell{Def: e.def, Val: e.val})
			} else {
				kept = append(kept, e)
			}
		}
		if len(kept) > 0 {
			nonCustomGroups = append(nonCustomGroups, kept)
		}
	}

	sort.Slice(customs, func(i, j int) bool {
		return customs[i].Def.Key < customs[j].Def.Key
	})

	// Batch layout for non-custom groups only.
	for gi := 0; gi < len(nonCustomGroups); gi += cols {
		end := min(gi+cols, len(nonCustomGroups))
		batch := nonCustomGroups[gi:end]

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
			rows = append(rows, row)
		}
	}

	// Phase 3: remove fully empty rows before filling customs.
	rows = removeEmptyRows(rows)

	// Phase 4: greedily fill custom cells into empty slots.
	ci := 0
	for r := range rows {
		if ci >= len(customs) {
			break
		}
		for c := range rows[r] {
			if ci >= len(customs) {
				break
			}
			if rows[r][c].Def == nil {
				rows[r][c] = customs[ci]
				ci++
			}
		}
	}

	// Append remaining customs as new rows.
	for ci < len(customs) {
		row := make([]metadataCell, cols)
		for c := range row {
			if ci >= len(customs) {
				break
			}
			row[c] = customs[ci]
			ci++
		}
		rows = append(rows, row)
	}

	rows = removeEmptyRows(rows)

	return metadataGrid{Rows: rows, Cols: cols}
}

// removeEmptyRows drops any row in which every cell is empty.
func removeEmptyRows(rows [][]metadataCell) [][]metadataCell {
	n := 0
	for _, row := range rows {
		empty := true
		for _, cell := range row {
			if cell.Def != nil {
				empty = false
				break
			}
		}
		if !empty {
			rows[n] = row
			n++
		}
	}
	return rows[:n]
}
