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
// Rule tests — each scenario asserts cell positions produced by
// buildMetadataGrid. Non-custom groups keep their batch-assigned columns;
// custom cells fill empty slots greedily top-to-bottom, left-to-right.
// -----------------------------------------------------------------------------

func TestBuildMetadataGrid_PairsGroupsIntoColumns(t *testing.T) {
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

func TestBuildMetadataGrid_CustomFillsGaps(t *testing.T) {
	// Ownership has 2 entries, Temporal has 1, Custom has 2.
	// After batch layout: row 0 full, row 1 has empty col 1.
	// Custom cells get collected and fill all empty slots greedily.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A"), mkEntry("reporter", core.RoleOwnership, "B")),
		group(mkEntry("created", core.RoleTemporal, "X")),
		group(mkEntry("team", core.RoleCustom, "T"), mkEntry("dept", core.RoleCustom, "D")),
	}, 2)

	// Row 0: assignee | created
	// Row 1: reporter | (empty) — customs fill the gap alphabetically
	// dept < team, so dept fills (1,1), team goes to next available slot
	if got := findCell(grid, "assignee"); got != (coord{0, 0}) {
		t.Errorf("assignee at %v, want (0,0)", got)
	}
	if got := findCell(grid, "created"); got != (coord{0, 1}) {
		t.Errorf("created at %v, want (0,1)", got)
	}
	if got := findCell(grid, "dept"); got != (coord{1, 1}) {
		t.Errorf("dept should fill gap at (1,1), got %v", got)
	}
}

func TestBuildMetadataGrid_CustomFillsMultipleGaps(t *testing.T) {
	// Three single-field non-custom groups in 3 cols, then customs.
	// Row 0 is full. Customs should start filling from row 1.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),
		group(mkEntry("created", core.RoleTemporal, "C")),
		group(mkEntry("sprint", core.RoleIteration, "S")),
		group(mkEntry("t1", core.RoleCustom, "v1"), mkEntry("t2", core.RoleCustom, "v2")),
		group(mkEntry("t3", core.RoleCustom, "v3")),
	}, 3)

	// Row 0: assignee | created | sprint (all full)
	// Customs (t1, t2, t3) fill new row(s)
	if got := findCell(grid, "t1"); got.row < 1 {
		t.Errorf("t1 should be on row >= 1, got %v", got)
	}
}

func TestBuildMetadataGrid_EmptyRowsRemoved(t *testing.T) {
	// Two groups: Ownership(2) in col 0, nothing in col 1.
	// Without empty row removal we'd have 2 rows with col 1 empty.
	// With customs filling gaps, the layout should be compact.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A"), mkEntry("reporter", core.RoleOwnership, "B")),
	}, 2)

	if len(grid.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(grid.Rows))
	}
}

func TestBuildMetadataGrid_EmptyRowFromUnevenBatch(t *testing.T) {
	// Batch of 2 groups: Ownership(1) and Temporal(3). After extracting
	// any customs and removing empty rows, the non-custom rows should
	// all have at least one populated cell.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),
		group(mkEntry("c", core.RoleTemporal, "X"), mkEntry("u", core.RoleTemporal, "Y"), mkEntry("d", core.RoleTemporal, "Z")),
	}, 2)

	for i, row := range grid.Rows {
		empty := true
		for _, cell := range row {
			if cell.Def != nil {
				empty = false
				break
			}
		}
		if empty {
			t.Errorf("row %d is fully empty — should have been removed", i)
		}
	}
}

func TestBuildMetadataGrid_NonCustomStaysInColumn(t *testing.T) {
	// Temporal group should stay in its assigned column even when
	// there are gaps elsewhere.
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),
		group(mkEntry("created", core.RoleTemporal, "X"), mkEntry("updated", core.RoleTemporal, "Y")),
	}, 2)

	if got := findCell(grid, "created"); got.col != 1 {
		t.Errorf("created should stay in col 1, got %v", got)
	}
	if got := findCell(grid, "updated"); got.col != 1 {
		t.Errorf("updated should stay in col 1, got %v", got)
	}
}

func TestBuildMetadataGrid_TrimsTrailingEmptyRows(t *testing.T) {
	grid := buildMetadataGrid([][]metadataEntry{
		group(mkEntry("assignee", core.RoleOwnership, "A")),
	}, 2)

	if len(grid.Rows) != 1 {
		t.Errorf("got %d rows, want 1", len(grid.Rows))
	}
}

// -----------------------------------------------------------------------------
// Invariant tests — assert properties that must hold for ANY input.
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
		{
			"customs fill uneven gaps",
			[][]metadataEntry{
				group(mkEntry("a", core.RoleOwnership, "v"), mkEntry("b", core.RoleOwnership, "v")),
				group(mkEntry("c", core.RoleTemporal, "v")),
				group(mkEntry("t1", core.RoleCustom, "v"), mkEntry("t2", core.RoleCustom, "v"), mkEntry("t3", core.RoleCustom, "v")),
			},
			2,
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

// No row should be fully empty — empty rows must be removed.
func TestBuildMetadataGrid_Invariant_NoEmptyRows(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)
			for i, row := range g.Rows {
				empty := true
				for _, cell := range row {
					if cell.Def != nil {
						empty = false
						break
					}
				}
				if empty {
					t.Errorf("row %d is entirely empty — should be removed", i)
				}
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

// Non-custom cells must remain in their batch-assigned column.
func TestBuildMetadataGrid_Invariant_NonCustomKeepColumn(t *testing.T) {
	for _, sc := range invariantScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			g := buildMetadataGrid(sc.groups, sc.cols)

			// Build expected column assignments from batch layout.
			wantCol := map[string]int{}
			for gi := 0; gi < len(sc.groups); gi += sc.cols {
				end := min(gi+sc.cols, len(sc.groups))
				batch := sc.groups[gi:end]
				for c, grp := range batch {
					for _, e := range grp {
						if e.def.Role != core.RoleCustom {
							wantCol[e.def.Key] = c
						}
					}
				}
			}

			for key, wc := range wantCol {
				got := findCell(g, key)
				if got.col != wc {
					t.Errorf("%s: col %d, want %d", key, got.col, wc)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// GridRequiredWidth + ChooseMetadataCols tests.
// -----------------------------------------------------------------------------

func TestGridRequiredWidth_Formula(t *testing.T) {
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
	grid, _ := ChooseMetadataCols(groups, 8, 51)
	if grid.Cols != 3 {
		t.Errorf("contentWidth=51 should fit 3 cols; got %d", grid.Cols)
	}
	grid, _ = ChooseMetadataCols(groups, 8, 50)
	if grid.Cols != 2 {
		t.Errorf("contentWidth=50 should drop to 2 cols; got %d", grid.Cols)
	}
	grid, _ = ChooseMetadataCols(groups, 8, 33)
	if grid.Cols != 1 {
		t.Errorf("contentWidth=33 should drop to 1 col; got %d", grid.Cols)
	}
}

func TestChooseMetadataCols_AlwaysReturnsAtLeastOne(t *testing.T) {
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
