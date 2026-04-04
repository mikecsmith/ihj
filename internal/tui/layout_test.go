package tui

import "testing"

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
