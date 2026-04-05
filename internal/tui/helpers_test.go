package tui

import (
	"reflect"
	"testing"
)

// Tests for the pure helpers in helpers.go: CalculateLayout,
// CalculateWindow, BuildTreeTokens. Each group pins down the named
// rules and invariants the corresponding function must satisfy.

// ── CalculateLayout ────────────────────────────────────────────────

// Baseline inputs used by most rule tests — a realistic 160x40 terminal
// with a 50/50 split, help bar on, not in vim mode, no breadcrumb.
func baseLayout() LayoutInputs {
	return LayoutInputs{
		TermW:       160,
		TermH:       40,
		DetailPct:   50,
		View:        ViewList,
		ShowHelpBar: true,
		VimMode:     false,
		CanGoBack:   false,
	}
}

// TestCalculateLayout_HeightSumEqualsTerminal — the core invariant:
// everything the user sees vertically adds up to the available inner
// body height (InnerH). If this fails, rows are either overflowing the
// terminal or wasting empty space.
func TestCalculateLayout_HeightSumEqualsTerminal(t *testing.T) {
	cases := []struct {
		name string
		in   LayoutInputs
	}{
		{"list split, help bar", baseLayout()},
		{"list split, vim mode", func() LayoutInputs {
			in := baseLayout()
			in.ShowHelpBar = false
			in.VimMode = true
			return in
		}()},
		{"detail split", func() LayoutInputs {
			in := baseLayout()
			in.View = ViewDetail
			return in
		}()},
		{"fullscreen", func() LayoutInputs {
			in := baseLayout()
			in.View = ViewFullscreen
			return in
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := CalculateLayout(tc.in)
			got := b.ListH + b.DetailTotalH + b.ChromeH
			if got != b.InnerH {
				t.Errorf("ListH(%d) + DetailTotalH(%d) + ChromeH(%d) = %d, want InnerH=%d",
					b.ListH, b.DetailTotalH, b.ChromeH, got, b.InnerH)
			}
		})
	}
}

// TestCalculateLayout_FullscreenCollapsesList — in fullscreen, no list
// pane is drawn; detail owns all body space.
func TestCalculateLayout_FullscreenCollapsesList(t *testing.T) {
	in := baseLayout()
	in.View = ViewFullscreen
	b := CalculateLayout(in)
	if b.ListH != 0 {
		t.Errorf("ListH = %d, want 0 in fullscreen", b.ListH)
	}
	if b.DetailContentH <= 0 {
		t.Errorf("DetailContentH = %d, want > 0 in fullscreen", b.DetailContentH)
	}
}

// TestCalculateLayout_MinimumClamps — aggressive shrink must not
// produce negative or zero sizes. The floors defend against crashes
// when the user resizes below sensible minimums.
func TestCalculateLayout_MinimumClamps(t *testing.T) {
	in := LayoutInputs{TermW: 10, TermH: 5, DetailPct: 50, View: ViewList, ShowHelpBar: true}
	b := CalculateLayout(in)
	if b.InnerW < minInnerW {
		t.Errorf("InnerW = %d, want >= %d", b.InnerW, minInnerW)
	}
	if b.InnerH < minInnerH {
		t.Errorf("InnerH = %d, want >= %d", b.InnerH, minInnerH)
	}
	if b.ListH < minListH {
		t.Errorf("ListH = %d, want >= %d (split view)", b.ListH, minListH)
	}
	if b.DetailContentH < 2 {
		t.Errorf("DetailContentH = %d, want >= 2", b.DetailContentH)
	}
	if b.DetailContentW < 0 {
		t.Errorf("DetailContentW = %d, want >= 0", b.DetailContentW)
	}
}

// TestCalculateLayout_MinimumClamps_FullscreenAllowsZeroList —
// fullscreen is the one case where ListH is allowed to be 0 (the list
// simply isn't drawn).
func TestCalculateLayout_MinimumClamps_FullscreenAllowsZeroList(t *testing.T) {
	in := LayoutInputs{TermW: 10, TermH: 5, DetailPct: 50, View: ViewFullscreen, ShowHelpBar: true}
	b := CalculateLayout(in)
	if b.ListH != 0 {
		t.Errorf("ListH = %d, want 0 (fullscreen)", b.ListH)
	}
}

// TestCalculateLayout_DetailPctRespected — at 50% split, DetailTotalH
// should be within one line of half the body (rounding up).
func TestCalculateLayout_DetailPctRespected(t *testing.T) {
	in := baseLayout()
	in.DetailPct = 50
	b := CalculateLayout(in)
	body := b.InnerH - b.ChromeH
	half := body / 2
	if b.DetailTotalH < half || b.DetailTotalH > half+1 {
		t.Errorf("DetailTotalH = %d, want within [%d, %d] for 50%% of body=%d",
			b.DetailTotalH, half, half+1, body)
	}
}

// TestCalculateLayout_HelpBarChrome — visible chrome rows differ by
// exactly 2 when the help bar is toggled on/off (vs plain default mode)
// and by 1 when only the vim indicator is visible.
func TestCalculateLayout_HelpBarChrome(t *testing.T) {
	noHelp := baseLayout()
	noHelp.ShowHelpBar = false
	noHelp.VimMode = false
	withHelp := baseLayout()
	withHelp.ShowHelpBar = true
	withHelp.VimMode = false
	vimOnly := baseLayout()
	vimOnly.ShowHelpBar = false
	vimOnly.VimMode = true

	bn := CalculateLayout(noHelp)
	bh := CalculateLayout(withHelp)
	bv := CalculateLayout(vimOnly)

	if diff := bh.ChromeH - bn.ChromeH; diff != 2 {
		t.Errorf("help bar chrome delta = %d, want 2", diff)
	}
	if diff := bv.ChromeH - bn.ChromeH; diff != 1 {
		t.Errorf("vim-only chrome delta = %d, want 1", diff)
	}
}

// TestCalculateLayout_BreadcrumbReserve — when we're deep in the detail
// stack (CanGoBack), DetailH (what the detail sub-model draws into) is
// one line smaller to leave room for the breadcrumb bar.
func TestCalculateLayout_BreadcrumbReserve(t *testing.T) {
	noBack := baseLayout()
	noBack.View = ViewDetail
	noBack.CanGoBack = false
	withBack := baseLayout()
	withBack.View = ViewDetail
	withBack.CanGoBack = true

	bn := CalculateLayout(noBack)
	bb := CalculateLayout(withBack)

	if diff := bn.DetailH - bb.DetailH; diff != 1 {
		t.Errorf("breadcrumb reserve delta = %d, want 1 (DetailH)", diff)
	}
	if bn.DetailContentH != bb.DetailContentH {
		t.Errorf("DetailContentH should match regardless of breadcrumb: %d vs %d",
			bn.DetailContentH, bb.DetailContentH)
	}
}

// TestCalculateLayout_MouseZoneOrdering — invariant: DetailTop <
// DetailBottom <= ListTop <= ListBottom. If this ever fails, click
// routing will send clicks to the wrong pane.
func TestCalculateLayout_MouseZoneOrdering(t *testing.T) {
	cases := []LayoutInputs{
		baseLayout(),
		func() LayoutInputs { in := baseLayout(); in.View = ViewDetail; return in }(),
		func() LayoutInputs { in := baseLayout(); in.View = ViewFullscreen; return in }(),
	}
	for _, in := range cases {
		b := CalculateLayout(in)
		if b.DetailTop >= b.DetailBottom {
			t.Errorf("DetailTop(%d) >= DetailBottom(%d)", b.DetailTop, b.DetailBottom)
		}
		if b.DetailBottom > b.ListTop {
			t.Errorf("DetailBottom(%d) > ListTop(%d)", b.DetailBottom, b.ListTop)
		}
		if b.ListTop > b.ListBottom {
			t.Errorf("ListTop(%d) > ListBottom(%d)", b.ListTop, b.ListBottom)
		}
	}
}

// ── CalculateWindow ────────────────────────────────────────────────

// TestCalculateWindow_CursorAlwaysVisible — the core invariant: no
// matter where the cursor is or how big the window is, the cursor
// must fall inside the returned [start, end) range. If this fails,
// the user's selection highlight disappears off-screen.
func TestCalculateWindow_CursorAlwaysVisible(t *testing.T) {
	for _, total := range []int{1, 5, 10, 20, 50} {
		for _, maxVisible := range []int{3, 5, 10} {
			for cursor := 0; cursor < total; cursor++ {
				start, end := CalculateWindow(cursor, total, maxVisible)
				if cursor < start || cursor >= end {
					t.Errorf("cursor=%d total=%d maxVisible=%d: window=[%d,%d) does not contain cursor",
						cursor, total, maxVisible, start, end)
				}
			}
		}
	}
}

// TestCalculateWindow_NoTopOverflow — cursor at position 0 means
// the window must begin at 0 (can't slide above the list start).
func TestCalculateWindow_NoTopOverflow(t *testing.T) {
	start, _ := CalculateWindow(0, 20, 5)
	if start != 0 {
		t.Errorf("cursor=0: start = %d, want 0", start)
	}
}

// TestCalculateWindow_NoBottomOverflow — cursor on the last item
// means the window must end exactly at total (can't slide past).
func TestCalculateWindow_NoBottomOverflow(t *testing.T) {
	total := 20
	_, end := CalculateWindow(total-1, total, 5)
	if end != total {
		t.Errorf("cursor=last: end = %d, want %d", end, total)
	}
}

// TestCalculateWindow_WindowSize — when the list is bigger than the
// visible area the window is exactly maxVisible items wide; when
// smaller, the window covers the whole list.
func TestCalculateWindow_WindowSize(t *testing.T) {
	// total >= maxVisible: exactly maxVisible items visible.
	start, end := CalculateWindow(5, 20, 7)
	if end-start != 7 {
		t.Errorf("total>maxVisible: window size = %d, want 7", end-start)
	}
	// total < maxVisible: whole list visible.
	start, end = CalculateWindow(2, 4, 10)
	if start != 0 || end != 4 {
		t.Errorf("total<maxVisible: window = [%d,%d), want [0,4)", start, end)
	}
	// total == maxVisible: whole list visible.
	start, end = CalculateWindow(3, 5, 5)
	if start != 0 || end != 5 {
		t.Errorf("total==maxVisible: window = [%d,%d), want [0,5)", start, end)
	}
}

// TestCalculateWindow_MonotonicScroll — as the cursor moves down one
// row at a time, the window start never decreases. A non-monotonic
// start would cause the viewport to flick backwards mid-scroll.
func TestCalculateWindow_MonotonicScroll(t *testing.T) {
	total := 30
	maxVisible := 7
	prevStart := 0
	for cursor := 0; cursor < total; cursor++ {
		start, _ := CalculateWindow(cursor, total, maxVisible)
		if start < prevStart {
			t.Errorf("cursor=%d: start=%d decreased from prev=%d", cursor, start, prevStart)
		}
		prevStart = start
	}
}

// TestCalculateWindow_DegenerateInputs — zero or negative totals /
// maxVisible yield an empty window rather than panicking.
func TestCalculateWindow_DegenerateInputs(t *testing.T) {
	cases := []struct {
		cursor, total, maxVisible int
	}{
		{0, 0, 5},
		{0, 10, 0},
		{0, -1, 5},
		{0, 10, -1},
	}
	for _, c := range cases {
		start, end := CalculateWindow(c.cursor, c.total, c.maxVisible)
		if start != 0 || end != 0 {
			t.Errorf("CalculateWindow(%d,%d,%d) = [%d,%d), want [0,0)",
				c.cursor, c.total, c.maxVisible, start, end)
		}
	}
}

// ── CalculateScrollWindow ──────────────────────────────────────────

// TestCalculateScrollWindow_CursorAlwaysVisible — core invariant: the
// cursor is always inside [start, end) regardless of prevStart.
func TestCalculateScrollWindow_CursorAlwaysVisible(t *testing.T) {
	for _, total := range []int{1, 5, 10, 50} {
		for _, maxVisible := range []int{3, 5, 10} {
			for cursor := 0; cursor < total; cursor++ {
				for _, prev := range []int{0, cursor, total - 1} {
					if prev < 0 {
						prev = 0
					}
					start, end := CalculateScrollWindow(cursor, prev, total, maxVisible)
					if cursor < start || cursor >= end {
						t.Errorf("cursor=%d prev=%d total=%d maxVisible=%d: window=[%d,%d) excludes cursor",
							cursor, prev, total, maxVisible, start, end)
					}
				}
			}
		}
	}
}

// TestCalculateScrollWindow_StableWhenCursorInWindow — if the cursor
// is already inside the previous window, the viewport doesn't move.
// This is the core property that distinguishes scroll-style from
// centring-style windowing: minimum viewport movement.
func TestCalculateScrollWindow_StableWhenCursorInWindow(t *testing.T) {
	total, maxVisible := 100, 10
	prevStart := 40
	// Every cursor position inside [40, 50) must leave start unchanged.
	for cursor := prevStart; cursor < prevStart+maxVisible; cursor++ {
		start, end := CalculateScrollWindow(cursor, prevStart, total, maxVisible)
		if start != prevStart {
			t.Errorf("cursor=%d inside prev window: start=%d, want %d", cursor, start, prevStart)
		}
		if end != prevStart+maxVisible {
			t.Errorf("cursor=%d: end=%d, want %d", cursor, end, prevStart+maxVisible)
		}
	}
}

// TestCalculateScrollWindow_FollowsCursorUp — cursor below prevStart
// drags the viewport up to include it (no wasted scroll distance).
func TestCalculateScrollWindow_FollowsCursorUp(t *testing.T) {
	start, _ := CalculateScrollWindow(10, 30, 100, 10)
	if start != 10 {
		t.Errorf("cursor=10 prev=30: start=%d, want 10 (cursor at top)", start)
	}
}

// TestCalculateScrollWindow_FollowsCursorDown — cursor past the bottom
// edge of the previous window shifts start down so cursor lands on
// the last visible row.
func TestCalculateScrollWindow_FollowsCursorDown(t *testing.T) {
	// prevStart=0, maxVisible=10 ⇒ window was [0,10). cursor=15 ⇒
	// new window should be [6,16) so 15 is at the bottom.
	start, end := CalculateScrollWindow(15, 0, 100, 10)
	if start != 6 || end != 16 {
		t.Errorf("cursor=15 prev=0: window=[%d,%d), want [6,16)", start, end)
	}
}

// TestCalculateScrollWindow_NoBottomOverflow — cursor on the last item
// must produce end == total, not spill past.
func TestCalculateScrollWindow_NoBottomOverflow(t *testing.T) {
	total := 20
	_, end := CalculateScrollWindow(total-1, 0, total, 5)
	if end != total {
		t.Errorf("cursor=last: end=%d, want %d", end, total)
	}
}

// TestCalculateScrollWindow_WindowSize — same sizing rules as
// CalculateWindow: total <= maxVisible ⇒ whole list, otherwise
// exactly maxVisible rows.
func TestCalculateScrollWindow_WindowSize(t *testing.T) {
	start, end := CalculateScrollWindow(5, 0, 20, 7)
	if end-start != 7 {
		t.Errorf("total>maxVisible: window size=%d, want 7", end-start)
	}
	start, end = CalculateScrollWindow(2, 0, 4, 10)
	if start != 0 || end != 4 {
		t.Errorf("total<maxVisible: window=[%d,%d), want [0,4)", start, end)
	}
}

// TestCalculateScrollWindow_MonotonicScroll — walking the cursor in
// either direction produces a monotonic start sequence (never flicks
// back the way the cursor came from).
func TestCalculateScrollWindow_MonotonicScroll(t *testing.T) {
	total, maxVisible := 50, 10
	// Down-walk.
	prev := 0
	prevStart := 0
	for cursor := 0; cursor < total; cursor++ {
		start, _ := CalculateScrollWindow(cursor, prevStart, total, maxVisible)
		if start < prevStart {
			t.Errorf("down-walk cursor=%d (was %d): start=%d decreased from %d",
				cursor, prev, start, prevStart)
		}
		prevStart = start
		prev = cursor
	}
	// Up-walk from the bottom.
	prevStart = total - maxVisible
	for cursor := total - 1; cursor >= 0; cursor-- {
		start, _ := CalculateScrollWindow(cursor, prevStart, total, maxVisible)
		if start > prevStart {
			t.Errorf("up-walk cursor=%d: start=%d increased from %d", cursor, start, prevStart)
		}
		prevStart = start
	}
}

// TestCalculateScrollWindow_DegenerateInputs — zero/negative total or
// maxVisible yields an empty window.
func TestCalculateScrollWindow_DegenerateInputs(t *testing.T) {
	cases := []struct{ cursor, prev, total, maxVisible int }{
		{0, 0, 0, 5},
		{0, 0, 10, 0},
		{0, 0, -1, 5},
		{0, 0, 10, -1},
	}
	for _, c := range cases {
		start, end := CalculateScrollWindow(c.cursor, c.prev, c.total, c.maxVisible)
		if start != 0 || end != 0 {
			t.Errorf("CalculateScrollWindow(%d,%d,%d,%d) = [%d,%d), want [0,0)",
				c.cursor, c.prev, c.total, c.maxVisible, start, end)
		}
	}
}

// ── BuildTreeTokens ────────────────────────────────────────────────

// TestBuildTreeTokens_RootHasNoTokens — depth 0 (root item) has no
// tree prefix; return an empty slice.
func TestBuildTreeTokens_RootHasNoTokens(t *testing.T) {
	tokens := BuildTreeTokens(0, true, nil)
	if len(tokens) != 0 {
		t.Errorf("depth=0 ⇒ len(tokens) = %d, want 0", len(tokens))
	}
}

// TestBuildTreeTokens_LengthEqualsDepth — the number of tokens is
// exactly depth, across a range of depths. Columns and tree glyphs
// downstream rely on this to align correctly.
func TestBuildTreeTokens_LengthEqualsDepth(t *testing.T) {
	for _, depth := range []int{1, 2, 3, 5, 10} {
		tokens := BuildTreeTokens(depth, false, make([]bool, depth))
		if len(tokens) != depth {
			t.Errorf("depth=%d: len(tokens) = %d, want %d", depth, len(tokens), depth)
		}
	}
}

// TestBuildTreeTokens_LastChildUsesCorner — the final token (the
// branch glyph for this row itself) is TokenCorner when the item is
// the last child at its level, TokenTee otherwise.
func TestBuildTreeTokens_LastChildUsesCorner(t *testing.T) {
	cases := []struct {
		depth  int
		isLast bool
		want   TreeToken
	}{
		{1, true, TokenCorner},
		{1, false, TokenTee},
		{4, true, TokenCorner},
		{4, false, TokenTee},
	}
	for _, tc := range cases {
		tokens := BuildTreeTokens(tc.depth, tc.isLast, make([]bool, tc.depth))
		if got := tokens[tc.depth-1]; got != tc.want {
			t.Errorf("depth=%d isLast=%v: tokens[%d] = %v, want %v",
				tc.depth, tc.isLast, tc.depth-1, got, tc.want)
		}
	}
}

// TestBuildTreeTokens_AncestorsMapToConnectors — each ancestor slot
// decides one connector column: true (ancestor was last child) ⇒
// TokenSpace (empty); false ⇒ TokenVert (continuing line).
func TestBuildTreeTokens_AncestorsMapToConnectors(t *testing.T) {
	// depth 4: tokens[0..2] are connectors driven by ancestors[1..3].
	ancestors := []bool{false, true, false, true} // ancestors[0] ignored.
	tokens := BuildTreeTokens(4, false, ancestors)
	want := []TreeToken{TokenSpace, TokenVert, TokenSpace, TokenTee}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("tokens = %v, want %v", tokens, want)
	}
}

// TestBuildTreeTokens_KnownShapes — table-driven scenarios spelled out
// for the common cases, so a regression points at the exact shape
// that broke.
func TestBuildTreeTokens_KnownShapes(t *testing.T) {
	cases := []struct {
		name      string
		depth     int
		isLast    bool
		ancestors []bool
		want      []TreeToken
	}{
		{
			name:  "depth 1 not last",
			depth: 1, isLast: false, ancestors: nil,
			want: []TreeToken{TokenTee},
		},
		{
			name:  "depth 1 last",
			depth: 1, isLast: true, ancestors: nil,
			want: []TreeToken{TokenCorner},
		},
		{
			name:  "depth 2 parent not last, item last",
			depth: 2, isLast: true, ancestors: []bool{false, false},
			want: []TreeToken{TokenVert, TokenCorner},
		},
		{
			name: "depth 3 one ancestor closed",
			// ancestors[1]=true ⇒ tokens[0]=Space;
			// ancestors[2]=false ⇒ tokens[1]=Vert; branch=Tee.
			depth: 3, isLast: false, ancestors: []bool{false, true, false},
			want: []TreeToken{TokenSpace, TokenVert, TokenTee},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildTreeTokens(tc.depth, tc.isLast, tc.ancestors)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildTreeTokens_DoesNotMutate — the ancestors slice is read-only;
// the function must not mutate it, even through aliasing.
func TestBuildTreeTokens_DoesNotMutate(t *testing.T) {
	ancestors := []bool{false, true, false}
	snapshot := []bool{false, true, false}
	_ = BuildTreeTokens(3, true, ancestors)
	if !reflect.DeepEqual(ancestors, snapshot) {
		t.Errorf("ancestors was mutated: got %v, want %v", ancestors, snapshot)
	}
}

// TestBuildTreeTokens_ShortAncestorsDefaultsToVert — if the ancestors
// slice is shorter than depth (shouldn't happen in practice, but we
// defend against it), missing entries default to "not last" so the
// connector keeps going.
func TestBuildTreeTokens_ShortAncestorsDefaultsToVert(t *testing.T) {
	tokens := BuildTreeTokens(3, false, []bool{}) // no ancestor data
	want := []TreeToken{TokenVert, TokenVert, TokenTee}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("tokens = %v, want %v", tokens, want)
	}
}

// ── CalculateListLayout ────────────────────────────────────────────

// Sum of every non-summary, non-ID list column + inter-column padding.
// Duplicated here on purpose — the test recomputes the budget formula
// from scratch so an accidental tweak to any constant is caught.
const fixedListColsNonID = listPrioW + listTypeW + listStatusW + listAssigneeW + listInterColGaps

// TestListLayout_SummaryBudgetFormula — the summary budget is exactly
// contentW minus every other column. Drift here is how truncation bugs
// creep in.
func TestListLayout_SummaryBudgetFormula(t *testing.T) {
	cases := []struct {
		name          string
		contentW      int
		maxIDW        int
		wantReclaimed int // expected SummaryBudget
	}{
		{"wide terminal short keys", 160, 7, 160 - 7 - fixedListColsNonID},
		{"narrow terminal short keys", 80, 7, 80 - 7 - fixedListColsNonID},
		{"wide terminal long keys", 160, 20, 160 - 20 - fixedListColsNonID},
		{"id width zero", 100, 0, 100 - fixedListColsNonID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateListLayout(tc.contentW, 30, tc.maxIDW)
			if got.SummaryBudget != tc.wantReclaimed {
				t.Errorf("SummaryBudget = %d, want %d", got.SummaryBudget, tc.wantReclaimed)
			}
		})
	}
}

// TestListLayout_CardModeThreshold — card mode activates exactly when
// SummaryBudget falls below CardModeMinBudget. A single cell either
// side of the threshold flips the mode.
func TestListLayout_CardModeThreshold(t *testing.T) {
	// contentW so that budget == CardModeMinBudget (on the boundary).
	// budget = contentW - maxIDW - fixedListColsNonID = threshold.
	maxIDW := 7
	boundaryW := CardModeMinBudget + maxIDW + fixedListColsNonID

	atBoundary := CalculateListLayout(boundaryW, 30, maxIDW)
	if atBoundary.CardMode {
		t.Errorf("at boundary (budget=%d == threshold=%d): CardMode=true, want false",
			atBoundary.SummaryBudget, CardModeMinBudget)
	}

	belowBoundary := CalculateListLayout(boundaryW-1, 30, maxIDW)
	if !belowBoundary.CardMode {
		t.Errorf("below boundary (budget=%d < threshold=%d): CardMode=false, want true",
			belowBoundary.SummaryBudget, CardModeMinBudget)
	}
}

// TestListLayout_RowsPerItemMatchesMode — invariant binding the two
// fields: RowsPerItem is 1 iff !CardMode, else 2. No other values ever.
func TestListLayout_RowsPerItemMatchesMode(t *testing.T) {
	for _, contentW := range []int{40, 60, 80, 100, 120, 160, 200} {
		for _, maxIDW := range []int{3, 7, 15, 25} {
			l := CalculateListLayout(contentW, 30, maxIDW)
			wantRPI := 1
			if l.CardMode {
				wantRPI = 2
			}
			if l.RowsPerItem != wantRPI {
				t.Errorf("contentW=%d maxIDW=%d: RowsPerItem=%d, want %d (CardMode=%v)",
					contentW, maxIDW, l.RowsPerItem, wantRPI, l.CardMode)
			}
		}
	}
}

// TestListLayout_ItemsVisibleNeverZero — even at a degenerate height
// of 1 (or 0), ItemsVisible returns at least 1 so the cursor always
// has somewhere to land. If this fails the window slicing later will
// produce an empty [start, end) and the cursor disappears.
func TestListLayout_ItemsVisibleNeverZero(t *testing.T) {
	for _, h := range []int{0, 1, 2, 3} {
		for _, mode := range []int{160, 60} { // wide (table) and narrow (card)
			l := CalculateListLayout(mode, h, 7)
			if l.ItemsVisible < 1 {
				t.Errorf("contentH=%d mode_w=%d: ItemsVisible=%d, want >=1",
					h, mode, l.ItemsVisible)
			}
		}
	}
}

// TestListLayout_ItemsVisibleFormula — when there's enough height to
// do real math, ItemsVisible follows the documented formula:
// floor((contentH - 1) / RowsPerItem). The -1 reserves the header row.
func TestListLayout_ItemsVisibleFormula(t *testing.T) {
	cases := []struct {
		name       string
		contentW   int
		contentH   int
		wantItems  int
		wantCardMd bool
	}{
		{"table mode, 30 lines", 160, 30, 29, false},  // 29 = (30-1) / 1
		{"table mode, 10 lines", 160, 10, 9, false},   // 9 = (10-1) / 1
		{"card mode, 30 lines", 60, 30, 14, true},     // 14 = (30-1) / 2
		{"card mode, 11 lines", 60, 11, 5, true},      // 5 = (11-1) / 2
		{"card mode, 21 lines odd", 60, 21, 10, true}, // 10 = (21-1) / 2
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := CalculateListLayout(tc.contentW, tc.contentH, 7)
			if l.CardMode != tc.wantCardMd {
				t.Fatalf("CardMode = %v, want %v", l.CardMode, tc.wantCardMd)
			}
			if l.ItemsVisible != tc.wantItems {
				t.Errorf("ItemsVisible = %d, want %d", l.ItemsVisible, tc.wantItems)
			}
		})
	}
}

// TestListLayout_LongIDsForceCardMode — even on a wide terminal, if
// the dataset has very long IDs the summary budget shrinks enough to
// trip into card mode. This is the responsive-to-data guarantee.
func TestListLayout_LongIDsForceCardMode(t *testing.T) {
	// Pick a contentW that is comfortably table-mode for short IDs.
	contentW := 120
	short := CalculateListLayout(contentW, 30, 6)
	if short.CardMode {
		t.Fatalf("short IDs on 120w should be table mode, got CardMode=true (budget=%d)", short.SummaryBudget)
	}
	// Grow maxIDW until budget falls below threshold.
	longID := contentW - fixedListColsNonID - CardModeMinBudget + 1
	long := CalculateListLayout(contentW, 30, longID)
	if !long.CardMode {
		t.Errorf("maxIDW=%d on contentW=%d should trip card mode, got budget=%d",
			longID, contentW, long.SummaryBudget)
	}
}
