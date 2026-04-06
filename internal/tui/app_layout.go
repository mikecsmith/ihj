package tui

import "math"

// ── Pure layout computation ───────────────────────────────────────

// LayoutInputs captures everything CalculateLayout needs to produce a
// LayoutBounds. Keeping this as a plain struct (rather than a long
// parameter list) makes call sites self-documenting and the pure
// function trivially testable.
type LayoutInputs struct {
	TermW       int
	TermH       int
	DetailPct   int // 0-100, share of body given to the detail pane
	View        ViewState
	ShowHelpBar bool
	VimMode     bool
	CanGoBack   bool // detail pane has a breadcrumb trail to surface
}

// LayoutBounds is the complete set of viewport dimensions derived from a
// set of LayoutInputs. All fields are non-negative. The split between
// DetailTotalH (border + content) and DetailContentH (content only) and
// DetailH (content minus breadcrumb reserve) mirrors how Bubble Tea
// sub-models consume sizes.
type LayoutBounds struct {
	InnerW         int
	InnerH         int
	DetailContentW int
	DetailTotalH   int
	DetailContentH int
	DetailH        int // size passed to detail.SetSize (post-breadcrumb reserve)
	ListH          int
	ChromeH        int

	// Mouse click zones — absolute row offsets within the terminal.
	DetailTop    int
	DetailBottom int
	ListTop      int
	ListBottom   int
}

// chrome constants — these match the outer AppBorder style
// (RoundedBorder + Padding(1,2)) and the detail sub-pane border.
const (
	outerBorderV  = 2 // top + bottom
	outerPadV     = 1 // 1 top + 0 bottom
	outerBorderH  = 2 // left + right
	outerPadH     = 4 // 2 left + 2 right
	detailBorderV = 2
	detailBorderH = 2
	detailPadH    = 4 // 2 left + 2 right

	minInnerW = 20
	minInnerH = 8
	minListH  = 3
)

// CalculateLayout is the pure layout function. Given a set of terminal
// dimensions and view state, it returns every viewport size and mouse
// zone the model needs. It performs no I/O and has no dependencies on
// Bubble Tea or model state beyond the fields on LayoutInputs.
func CalculateLayout(in LayoutInputs) LayoutBounds {
	var b LayoutBounds

	b.InnerW = max(in.TermW-outerBorderH-outerPadH, minInnerW)
	b.DetailContentW = b.InnerW - detailBorderH - detailPadH
	b.InnerH = max(in.TermH-outerBorderV-outerPadV, minInnerH)

	// Chrome height: visible bars above/below the body.
	if in.ShowHelpBar {
		b.ChromeH += 2 // help bar (1) + divider (1)
	} else if in.VimMode {
		b.ChromeH++ // vim mode indicator only, no divider
	}
	if in.View != ViewFullscreen {
		b.ChromeH += 2 // search bar (1) + divider below detail (1)
	}

	body := b.InnerH - b.ChromeH
	if in.View == ViewFullscreen {
		b.DetailTotalH = body
		b.ListH = 0
	} else {
		pct := float64(in.DetailPct) / 100.0
		b.DetailTotalH = int(math.Ceil(float64(body) * pct))
		b.ListH = body - b.DetailTotalH
		if b.ListH < minListH {
			b.ListH = minListH
			b.DetailTotalH = body - b.ListH
		}
	}

	b.DetailContentH = max(b.DetailTotalH-detailBorderV, 2)

	b.DetailH = b.DetailContentH
	if in.View >= ViewDetail && in.CanGoBack {
		b.DetailH = max(b.DetailH-1, 1)
	}

	// Mouse zones. DetailTop = outer border top (1) + outer pad top (1)
	// + detail border top (1) = 3 absolute rows from the terminal origin.
	b.DetailTop = 3
	b.DetailBottom = b.DetailTop + b.DetailContentH

	searchH := 0
	if in.View != ViewFullscreen {
		searchH = 1
	}
	b.ListTop = b.DetailBottom + detailBorderV - 1 + searchH
	b.ListBottom = b.ListTop + b.ListH

	return b
}

// ── View state transitions ────────────────────────────────────────

// recalcLayout recomputes all layout dimensions from the current terminal
// size and view state, then propagates sizes to sub-models.
func (m *AppModel) recalcLayout() {
	b := CalculateLayout(LayoutInputs{
		TermW:       m.width,
		TermH:       m.height,
		DetailPct:   m.detailPct,
		View:        m.view,
		ShowHelpBar: m.showHelpBar,
		VimMode:     m.vimMode,
		CanGoBack:   m.detail.CanGoBack(),
	})

	m.innerW = b.InnerW
	m.detailContentW = b.DetailContentW
	m.detailTotalH = b.DetailTotalH
	m.detailContentH = b.DetailContentH
	m.listH = b.ListH
	m.detailTop = b.DetailTop
	m.detailBottom = b.DetailBottom
	m.listTop = b.ListTop
	m.listBottom = b.ListBottom

	m.detail.SetSize(m.detailContentW, b.DetailH)
	m.list.SetSize(m.innerW, m.listH)
	m.help.SetWidth(m.innerW)
}

// View state transitions. These centralise side effects (search focus,
// layout recalculation, event emission) so callers don't manage them.

func (m *AppModel) enterFullscreen() {
	m.view = ViewFullscreen
	m.list.search.Blur()
	m.recalcLayout()
	m.ui.Emit(EventViewFullscreen)
}

func (m *AppModel) focusDetail() {
	m.view = ViewDetail
	m.list.search.Blur()
	m.ui.Emit(EventViewDetail)
}

func (m *AppModel) focusList() {
	m.view = ViewList
	if !m.vimMode {
		m.list.search.Focus()
	}
	m.ui.Emit(EventViewList)
}

// exitDetailView exits the current detail view state (fullscreen or pane focus).
// Returns true if an action was taken.
func (m *AppModel) exitDetailView() bool {
	if m.view < ViewDetail {
		return false
	}
	m.view = ViewList
	m.detail.ClearHistory()
	if !m.vimMode {
		m.list.search.Focus()
	}
	m.recalcLayout()
	m.syncDetail()
	m.ui.Emit(EventViewList)
	return true
}
