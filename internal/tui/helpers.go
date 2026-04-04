package tui

import (
	"math"

	"github.com/mikecsmith/ihj/internal/core"
)

// This file collects the package's pure layout helpers — small,
// dependency-free functions that compute viewport sizes, sliding
// window bounds, and tree-glyph sequences. They exist so the layout
// math can be tested in complete isolation from Bubble Tea model
// state and lipgloss rendering. See helpers_test.go for the
// rule-based tests that pin down their guarantees.

// ── App layout ─────────────────────────────────────────────────────

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

// ── Popup sliding window ───────────────────────────────────────────

// CalculateWindow returns the [start, end) slice bounds of the visible
// item window in a scrolling list, given the cursor position, total
// item count, and the maximum number of items that can fit on-screen.
//
// Behaviour:
//   - If total <= maxVisible: the whole list is visible (start=0, end=total).
//   - Otherwise: the cursor is centred in the window where possible, and
//     the window is clamped so it never overflows either end of the list.
//
// Guarantees (see helpers_test.go):
//   - cursor is always within [start, end).
//   - end - start == maxVisible when total >= maxVisible.
//   - start is monotonically non-decreasing as cursor increases.
func CalculateWindow(cursor, total, maxVisible int) (start, end int) {
	if total <= 0 || maxVisible <= 0 {
		return 0, 0
	}
	if total <= maxVisible {
		return 0, total
	}
	start = cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return start, end
}

// ── Tree prefix glyphs ─────────────────────────────────────────────

// TreeToken is one column of a tree-prefix row. A sequence of tokens
// fully describes the shape of a row's branch/connector glyphs; the
// renderer maps each token to a concrete glyph string via Glyph() and
// then applies colour.
type TreeToken int

const (
	TokenSpace  TreeToken = iota // ancestor at this column was the last child (blank)
	TokenVert                    // ancestor had more siblings, connector continues
	TokenTee                     // this item has siblings after it
	TokenCorner                  // this item is the last child
)

// Glyph returns the two-cell-wide string that renders this token. All
// glyph values come from core.TreeColXxx, so the visual alphabet lives
// in one place and stays in sync across every tree renderer.
func (t TreeToken) Glyph() string {
	switch t {
	case TokenSpace:
		return core.TreeColSpace
	case TokenVert:
		return core.TreeColVert
	case TokenTee:
		return core.TreeColTee
	case TokenCorner:
		return core.TreeColCorner
	default:
		return core.TreeColSpace
	}
}

// BuildTreeTokens returns the column tokens for a tree row of the
// given depth. The returned slice has length == depth:
//
//   - tokens[0 .. depth-2] are connector columns for ancestors, driven
//     by the ancestors slice: ancestors[i] == true ⇒ TokenSpace at
//     tokens[i-1]; false ⇒ TokenVert. (ancestors[0] is unused — it
//     represents the root level.)
//   - tokens[depth-1] is the branch glyph for this item: TokenCorner
//     if isLast, otherwise TokenTee.
//
// Root items (depth == 0) have no tree prefix and produce an empty
// slice. The ancestors slice is read only and never mutated; callers
// may pass a shorter slice than depth, in which case missing entries
// are treated as false (draw a vertical connector).
func BuildTreeTokens(depth int, isLast bool, ancestors []bool) []TreeToken {
	if depth <= 0 {
		return nil
	}
	tokens := make([]TreeToken, depth)
	for i := 1; i < depth; i++ {
		if i < len(ancestors) && ancestors[i] {
			tokens[i-1] = TokenSpace
		} else {
			tokens[i-1] = TokenVert
		}
	}
	if isLast {
		tokens[depth-1] = TokenCorner
	} else {
		tokens[depth-1] = TokenTee
	}
	return tokens
}
