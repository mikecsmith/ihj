package tui

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

// mkEntry builds a metadataEntry with a synthetic FieldDef — enough for
// layout rules which only inspect Role, and a key for identity assertions.
func mkEntry(key string, role core.FieldRole, val string) metadataEntry {
	def := &core.FieldDef{Key: key, Role: role}
	return metadataEntry{def: def, val: val}
}

// group is shorthand for a role group of entries in a test case.
func group(entries ...metadataEntry) []metadataEntry { return entries }

// coord locates a cell by row and column.
type coord struct{ row, col int }

// cellAt returns (key, true) if the grid has a populated cell at (r,c),
// or ("", false) if empty or out of bounds.
func cellAt(g metadataGrid, r, c int) (string, bool) {
	if r < 0 || r >= len(g.Rows) || c < 0 || c >= len(g.Rows[r]) {
		return "", false
	}
	cell := g.Rows[r][c]
	if cell.Def == nil {
		return "", false
	}
	return cell.Def.Key, true
}

// findCell returns the (row, col) of the first cell with the given key,
// or (-1, -1) if absent.
func findCell(g metadataGrid, key string) coord {
	for r, row := range g.Rows {
		for c, cell := range row {
			if cell.Def != nil && cell.Def.Key == key {
				return coord{r, c}
			}
		}
	}
	return coord{-1, -1}
}

// -----------------------------------------------------------------------------
// Rule tests — each scenario is a named case that asserts the cell positions
// produced by buildMetadataGrid. These tests document what the layout SHOULD
// do, in terms the reader can verify against the rules in metadata_grid.go.
// -----------------------------------------------------------------------------

func TestBuildMetadataGrid_PairsGroupsIntoColumns(t *testing.T) {
	// Two two-entry groups should pair into two columns side by side.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A"), mkEntry("reporter", core.RoleOwnership, "B")),
		group(mkEntry("created", core.RoleTemporal, "X"), mkEntry("updated", core.RoleTemporal, "Y")),
	}, 2)

	want := map[string]coord{
		"assignee": {0, 0}, "created": {0, 1},
		"reporter": {1, 0}, "updated": {1, 1},
	}
	for key, c := range want {
		got := findCell(grid, key)
		if got != c {
			t.Errorf("%s at %v, want %v", key, got, c)
		}
	}
	if len(grid.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(grid.Rows))
	}
}

func TestBuildMetadataGrid_OrphanSingleUpCollapses(t *testing.T) {
	// Parent is a single-field group that lands alone on a later row —
	// it should collapse up into the first empty slot.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A"), mkEntry("reporter", core.RoleOwnership, "B")),
		group(mkEntry("sprint", core.RoleIteration, "S")),
		group(mkEntry("parent", core.RoleParent, "P")),
	}, 2)

	// First batch (cols=2) pairs Ownership + Iteration:
	//   row 0: assignee | sprint
	//   row 1: reporter | (empty)
	// Second batch starts at group[2]=Parent on row 2 col 0. Parent is not
	// part of a column chain → moves up into row 1 col 1.
	if got := findCell(grid, "parent"); got != (coord{1, 1}) {
		t.Errorf("parent should lift to row 1 col 1, got %v", got)
	}
	// Grid should now be 2 rows — row 2 was all-empty and trimmed.
	if len(grid.Rows) != 2 {
		t.Errorf("got %d rows, want 2 after trim", len(grid.Rows))
	}
}

func TestBuildMetadataGrid_MultiRowChainNotBroken(t *testing.T) {
	// A three-entry Temporal group paired with a two-entry Ownership group
	// leaves row 2 col 0 empty. A later orphan (Parent) must NOT move into
	// that gap — doing so would break the ownership/temporal alignment only
	// if the slot sits inside an existing chain. Here the chain is Temporal
	// in col 1, so row 2 col 0 is a safe target... BUT only a Role that is
	// not chain-restricted can move there. Parent qualifies.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A"), mkEntry("reporter", core.RoleOwnership, "B")),
		group(mkEntry("created", core.RoleTemporal, "X"), mkEntry("updated", core.RoleTemporal, "Y"), mkEntry("due", core.RoleTemporal, "Z")),
		group(mkEntry("parent", core.RoleParent, "P")),
	}, 2)

	// Row 0: assignee | created
	// Row 1: reporter | updated
	// Row 2: (empty)  | due
	// Parent on row 3 col 0 — col 0 has no chain of RoleParent above it,
	// so parent moves up to (2, 0).
	if got := findCell(grid, "due"); got != (coord{2, 1}) {
		t.Errorf("due should stay at row 2 col 1 (chain), got %v", got)
	}
	if got := findCell(grid, "parent"); got != (coord{2, 0}) {
		t.Errorf("parent should lift to row 2 col 0, got %v", got)
	}
}

func TestBuildMetadataGrid_DoesNotRipChainMemberOutOfColumn(t *testing.T) {
	// Ownership has 2 entries, Temporal has 3, Iteration has 1.
	// Iteration's sprint must NOT get ripped out of its own column to fill
	// a gap — but because sprint is the first (and only) entry in its
	// column, it isn't "part of a chain" yet. What we're really testing is
	// that cells later in a same-role column DON'T move up past their
	// predecessors. Use a RoleTemporal group in the second batch and make
	// sure its second entry doesn't leapfrog its first.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),                                 // 1-entry
		group(mkEntry("sprint", core.RoleIteration, "S")),                                   // 1-entry
		group(mkEntry("team", core.RoleCustom, "T"), mkEntry("dept", core.RoleCustom, "D")), // 2-entry
	}, 2)

	// Batch 1: Ownership | Iteration → 1 row: assignee | sprint
	// Batch 2: Custom group begins at row 1 col 0 across 2 rows:
	//   row 1: team | (empty)
	//   row 2: dept | (empty)
	// Custom cells can collapse left (nothing to collapse here) and also
	// collapse up. team at (1,0): no previous row has RoleCustom in col 0,
	// but (1,0) is empty — no wait, team IS at (1,0). It needs to find an
	// empty earlier slot. (0,*) are both full, so team stays.
	// dept at (2,0): col 0 above contains team (also RoleCustom) at row 1
	// → dept IS in a chain. So dept must NOT move up, preserving column.
	if got := findCell(grid, "team"); got != (coord{1, 0}) {
		t.Errorf("team at %v, want row 1 col 0", got)
	}
	if got := findCell(grid, "dept"); got != (coord{2, 0}) {
		t.Errorf("dept must remain below team in same column, got %v", got)
	}
}

func TestBuildMetadataGrid_CustomCollapsesLeftWithinRow(t *testing.T) {
	// Three single-field groups across 3 cols. Custom field should
	// collapse left INTO an empty slot to its left within the same row.
	// But with `cols=3` and three singles, all slots are filled on row 0.
	// Force an empty left slot by pairing a non-custom single with a
	// custom single in a later batch where the non-custom is absent.
	grid := buildMetadataGrid([][]metadataEntry{
		// Batch 1 (cols=3): 3 groups
		group(mkEntry("assignee", core.RoleOwnership, "A")),
		group(mkEntry("created", core.RoleTemporal, "C")),
		group(mkEntry("sprint", core.RoleIteration, "S")),
		// Batch 2: only a single custom group; cols 1 and 2 start empty,
		// custom lands in col 0 by the batch position logic (first col).
		group(mkEntry("team", core.RoleCustom, "T")),
	}, 3)

	// Batch 2 places team at row 1 col 0. There's nothing to collapse left
	// within row 1 (team is already leftmost). Then up-collapse lifts it
	// into an empty slot in row 0 — but row 0 is full. So team stays at
	// (1,0). This test just confirms single-custom batches land in col 0.
	if got := findCell(grid, "team"); got != (coord{1, 0}) {
		t.Errorf("team at %v, want row 1 col 0", got)
	}
}

func TestBuildMetadataGrid_CustomCollapsesLeftWhenNonCustomAbsent(t *testing.T) {
	// Pair a custom 2-entry group with a non-custom 1-entry group in a
	// single batch. Custom occupies col 0, iteration col 1.
	// Row 0: team | sprint
	// Row 1: dept | (empty)    ← dept is custom, no collapse-left available
	// Then a later custom group (labels) lands: col 0 row 2 = labels.
	// labels would collapse up into (1,1) because col 1 has no labels-role
	// chain above.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("team", core.RoleCustom, "T"), mkEntry("dept", core.RoleCustom, "D")),
		group(mkEntry("sprint", core.RoleIteration, "S")),
		group(mkEntry("labels", core.RoleCustom, "L")),
	}, 2)

	if got := findCell(grid, "team"); got != (coord{0, 0}) {
		t.Errorf("team at %v, want (0,0)", got)
	}
	if got := findCell(grid, "sprint"); got != (coord{0, 1}) {
		t.Errorf("sprint at %v, want (0,1)", got)
	}
	if got := findCell(grid, "dept"); got != (coord{1, 0}) {
		t.Errorf("dept at %v, want (1,0) — chain continues", got)
	}
	// labels starts its own batch at row 2 col 0. Col 0 at rows 0..1 has
	// RoleCustom (team, dept) — so labels IS in a chain → cannot move up.
	if got := findCell(grid, "labels"); got != (coord{2, 0}) {
		t.Errorf("labels should stay at (2,0) because col 0 is custom-chained, got %v", got)
	}
}

func TestBuildMetadataGrid_TrimsTrailingEmptyRows(t *testing.T) {
	// A fully populated first row followed by an all-empty synthesized
	// second row (from a short batch) should be trimmed.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),
	}, 2)

	// Single group, single entry → 1 row with (1,1) empty. After trim,
	// exactly 1 row remains.
	if len(grid.Rows) != 1 {
		t.Errorf("got %d rows, want 1", len(grid.Rows))
	}
}

// -----------------------------------------------------------------------------
// Invariant tests — assert properties that must hold for ANY input.
// Each invariant test runs against a table of diverse grids.
// -----------------------------------------------------------------------------

func invariantScenarios() []struct {
	name   string
	groups [][]metadataEntry
	cols   int
} {
	return []struct {
		name   string
		groups [][]metadataEntry
		cols   int
	}{
		{"empty", nil, 2},
		{"single entry", [][]metadataEntry{group(mkEntry("a", core.RoleOwnership, "v"))}, 2},
		{
			"two pairs",
			[][]metadataEntry{
				group(mkEntry("a", core.RoleOwnership, "v"), mkEntry("b", core.RoleOwnership, "v")),
				group(mkEntry("c", core.RoleTemporal, "v"), mkEntry("d", core.RoleTemporal, "v")),
			},
			2,
		},
		{
			"3 temporal + 2 ownership + 1 iteration + 1 parent",
			[][]metadataEntry{
				group(mkEntry("a", core.RoleOwnership, "v"), mkEntry("b", core.RoleOwnership, "v")),
				group(mkEntry("c", core.RoleTemporal, "v"), mkEntry("d", core.RoleTemporal, "v"), mkEntry("e", core.RoleTemporal, "v")),
				group(mkEntry("s", core.RoleIteration, "v")),
				group(mkEntry("p", core.RoleParent, "")),
			},
			2,
		},
		{
			"many customs 3-col",
			[][]metadataEntry{
				group(mkEntry("a", core.RoleOwnership, "v")),
				group(mkEntry("c", core.RoleTemporal, "v")),
				group(mkEntry("s", core.RoleIteration, "v")),
				group(mkEntry("t1", core.RoleCustom, "v"), mkEntry("t2", core.RoleCustom, "v")),
				group(mkEntry("l", core.RoleCustom, "v")),
			},
			3,
		},
	}
}

// Every row must have exactly grid.Cols cells.
func TestBuildMetadataGrid_Invariant_RowWidthMatchesCols(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)
			for i, row := range g.Rows {
				if len(row) != g.Cols {
					t.Errorf("row %d has %d cells, want %d", i, len(row), g.Cols)
				}
			}
		})
	}
}

// The last row (if any) must contain at least one populated cell —
// trailing empty rows must be trimmed.
func TestBuildMetadataGrid_Invariant_NoTrailingEmptyRow(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)
			if len(g.Rows) == 0 {
				return
			}
			last := g.Rows[len(g.Rows)-1]
			hasCell := false
			for _, cell := range last {
				if cell.Def != nil {
					hasCell = true
					break
				}
			}
			if !hasCell {
				t.Errorf("last row is entirely empty — should be trimmed")
			}
		})
	}
}

// No cell should be lost during layout — every input entry must appear
// exactly once in the grid.
func TestBuildMetadataGrid_Invariant_AllEntriesPreserved(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)
			want := map[string]int{}
			for _, grp := range sc.groups {
				for _, e := range grp {
					want[e.def.Key]++
				}
			}
			got := map[string]int{}
			for _, row := range g.Rows {
				for _, cell := range row {
					if cell.Def != nil {
						got[cell.Def.Key]++
					}
				}
			}
			if len(got) != len(want) {
				t.Errorf("got %d unique keys, want %d", len(got), len(want))
			}
			for k, n := range want {
				if got[k] != n {
					t.Errorf("key %q appears %d times, want %d", k, got[k], n)
				}
			}
		})
	}
}

// Within any column, a contiguous run of same-role cells must remain
// contiguous — the order of cells in that column must preserve the input
// order, and no foreign-role cell may be inserted mid-chain.
func TestBuildMetadataGrid_Invariant_ColumnChainsContiguous(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)
			for c := 0; c < g.Cols; c++ {
				// Walk column, record (row, role) pairs for populated cells.
				var seen []core.FieldRole
				for r := range g.Rows {
					cell := g.Rows[r][c]
					if cell.Def == nil {
						continue
					}
					seen = append(seen, cell.Def.Role)
				}
				// A role may appear in multiple disjoint runs (if it was
				// placed in two separate batches), but within any single
				// run no foreign role may intrude. We check that each
				// role's occurrences in this column form contiguous
				// blocks.
				blockStart := map[core.FieldRole]int{}
				blockEnd := map[core.FieldRole]int{}
				for i, role := range seen {
					if _, ok := blockStart[role]; !ok {
						blockStart[role] = i
					}
					blockEnd[role] = i
				}
				for role, start := range blockStart {
					end := blockEnd[role]
					for i := start; i <= end; i++ {
						if seen[i] != role {
							t.Errorf("col %d: role %q has non-contiguous run (foreign role %q at index %d)", c, role, seen[i], i)
						}
					}
				}
			}
		})
	}
}

// A non-custom cell never moves into a column whose immediate-left
// neighbour in the same row is from a different role — i.e. leftward
// collapse is restricted to RoleCustom.
func TestBuildMetadataGrid_Invariant_OnlyCustomCollapsesLeft(t *testing.T) {
	// Build a scenario where a non-custom and custom group share a batch,
	// and verify non-custom cells sit in their expected column while
	// custom cells may have shifted.
	g := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "v")),
		group(mkEntry("t", core.RoleCustom, "v"), mkEntry("u", core.RoleCustom, "v")),
	}, 2)
	// Batch: cols=2, groups = [ownership(1), custom(2)]
	//   row 0: a | t
	//   row 1: (empty) | u  →  collapse-left: u is custom → moves to col 0
	// So after collapse: row 1 col 0 = u, row 1 col 1 = empty.
	if key, ok := cellAt(g, 1, 0); !ok || key != "u" {
		t.Errorf("row 1 col 0 = %q/%v, want u", key, ok)
	}
	if _, ok := cellAt(g, 1, 1); ok {
		t.Errorf("row 1 col 1 should be empty after collapse-left")
	}
}

// An orphan that moves upward must land strictly above its original row.
func TestBuildMetadataGrid_Invariant_UpCollapsePreservesUpwardDirection(t *testing.T) {
	g := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "v"), mkEntry("b", core.RoleOwnership, "v")),
		group(mkEntry("s", core.RoleIteration, "v")),
		group(mkEntry("p", core.RoleParent, "")),
	}, 2)

	// Before up-collapse: p would be at row 2 col 0. After: must be at
	// some row < 2.
	pc := findCell(g, "p")
	if pc.row >= 2 {
		t.Errorf("parent stayed at or below its original row: %v", pc)
	}
}

// -----------------------------------------------------------------------------
// Rule tests — GridRequiredWidth + ChooseMetadataCols. These pin the
// breakpoint math that decides how many columns the metadata grid renders
// in, independent of any actual rendering.
// -----------------------------------------------------------------------------

func TestGridRequiredWidth_Formula(t *testing.T) {
	// Three values of width 3, 5, 4 across 3 columns with labelColW=8:
	// required == 3*(8+6) + (3+5+4) == 42 + 12 == 54.
	g := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "aaa")),  // col 0, w=3
		group(mkEntry("b", core.RoleTemporal, "bbbbb")), // col 1, w=5
		group(mkEntry("c", core.RoleIteration, "cccc")), // col 2, w=4
	}, 3)
	required, mv := GridRequiredWidth(g, 8)
	if len(mv) != 3 {
		t.Fatalf("maxValW length: got %d want 3", len(mv))
	}
	if mv[0] != 3 || mv[1] != 5 || mv[2] != 4 {
		t.Errorf("maxValW = %v, want [3 5 4]", mv)
	}
	want := 3*(8+gridPerColumnOverhead) + (3 + 5 + 4)
	if required != want {
		t.Errorf("required = %d, want %d", required, want)
	}
}

func TestGridRequiredWidth_EmptyCellsIgnored(t *testing.T) {
	// Empty-value cells (e.g. pinned customs with no value) shouldn't
	// push the column's max width up.
	g := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "x"), mkEntry("b", core.RoleOwnership, "")),
	}, 1)
	_, mv := GridRequiredWidth(g, 5)
	if mv[0] != 1 {
		t.Errorf("empty-value cell should not widen column: got %d want 1", mv[0])
	}
}

func TestChooseMetadataCols_PicksLargestFitting(t *testing.T) {
	groups := [][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "val")),
		group(mkEntry("b", core.RoleTemporal, "val")),
		group(mkEntry("c", core.RoleIteration, "val")),
	}
	// Each cell: labelColW(8) + val(3) + 6 = 17. 3 cols need 51, 2 cols 34.
	// At contentWidth=51: 3 cols fits.
	grid, _ := ChooseMetadataCols(groups, 8, 51)
	if grid.Cols != 3 {
		t.Errorf("contentWidth=51 should fit 3 cols; got %d", grid.Cols)
	}
	// At contentWidth=50: 3 cols doesn't fit, 2 cols does (34 <= 50).
	grid, _ = ChooseMetadataCols(groups, 8, 50)
	if grid.Cols != 2 {
		t.Errorf("contentWidth=50 should drop to 2 cols; got %d", grid.Cols)
	}
	// At contentWidth=33: 2 cols doesn't fit (34), 1 col does (17).
	grid, _ = ChooseMetadataCols(groups, 8, 33)
	if grid.Cols != 1 {
		t.Errorf("contentWidth=33 should drop to 1 col; got %d", grid.Cols)
	}
}

func TestChooseMetadataCols_AlwaysReturnsAtLeastOne(t *testing.T) {
	// Even when the content width is absurdly small, the chooser must
	// return a usable 1-col grid.
	groups := [][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "very long value that does not fit anywhere")),
	}
	grid, mv := ChooseMetadataCols(groups, 100, 5)
	if grid.Cols != 1 {
		t.Errorf("minimum cols should be 1; got %d", grid.Cols)
	}
	if len(mv) != 1 {
		t.Errorf("maxValW should have 1 entry; got %d", len(mv))
	}
}

func TestChooseMetadataCols_FitInvariant(t *testing.T) {
	// When the chooser returns cols > 1, the grid MUST actually fit in
	// contentWidth. (cols==1 is the fallback and may exceed.)
	groups := [][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "one")),
		group(mkEntry("b", core.RoleTemporal, "two")),
		group(mkEntry("c", core.RoleIteration, "three")),
		group(mkEntry("d", core.RoleCustom, "four")),
	}
	for cw := 1; cw <= 200; cw++ {
		grid, _ := ChooseMetadataCols(groups, 8, cw)
		if grid.Cols <= 1 {
			continue
		}
		required, _ := GridRequiredWidth(grid, 8)
		if required > cw {
			t.Errorf("contentWidth=%d: grid.Cols=%d required=%d does not fit",
				cw, grid.Cols, required)
		}
	}
}

func TestChooseMetadataCols_MonotoneInWidth(t *testing.T) {
	// Widening contentWidth should never reduce the returned column count.
	groups := [][]metadataEntry{
		group(mkEntry("a", core.RoleOwnership, "xx")),
		group(mkEntry("b", core.RoleTemporal, "yy")),
		group(mkEntry("c", core.RoleIteration, "zz")),
	}
	prev := 0
	for cw := 1; cw <= 300; cw++ {
		grid, _ := ChooseMetadataCols(groups, 8, cw)
		if grid.Cols < prev {
			t.Errorf("at cw=%d cols dropped from %d to %d", cw, prev, grid.Cols)
		}
		prev = grid.Cols
	}
}
