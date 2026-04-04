package tui

import (
	"github.com/mikecsmith/ihj/internal/core"
)

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
