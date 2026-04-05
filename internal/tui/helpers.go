package tui

import (
	"math"

	"charm.land/lipgloss/v2"
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

// CompositeOverlay composites a rendered overlay onto the base screen at
// a given position using the lipgloss v2 Compositor. The base sits at
// Z=0; the overlay at (left, top, Z=1) so it always draws on top.
//
// Guarantees:
//   - An empty overlay returns the base string unchanged.
//   - Cells under the overlay's bounding box show the overlay's glyphs.
//   - Cells outside that box show the base's glyphs.
func CompositeOverlay(base, overlay string, top, left int) string {
	if overlay == "" {
		return base
	}
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(overlay).X(left).Y(top).Z(1),
	).Render()
}

// ── List layout (table vs card mode) ───────────────────────────────

// List column widths (in cells) and inter-column padding. These feed
// the summary-budget calculation and drive the threshold between the
// single-line table layout and the 2-line card layout.
const (
	listPrioW        = 1
	listTypeW        = 10
	listStatusW      = 16 // icon + space + 14-char name
	listAssigneeW    = 16
	listInterColGaps = 14 // sum of StyleFunc pads: 3+1+3+3+3+1

	// CardModeMinBudget is the smallest acceptable summary column
	// width in table mode. When the budget would fall below this, the
	// list switches to 2-line cards so summaries get room to breathe.
	CardModeMinBudget = 40
)

// ListLayout describes the list's rendering decisions for a given
// viewport and dataset: whether to render single-line table rows or
// 2-line cards, how many items the viewport can show, and how many
// cells are available for the summary column in table mode.
type ListLayout struct {
	CardMode      bool // true = 2-line cards (narrow), false = 1-line table rows
	ItemsVisible  int  // maximum items the window can display (≥ 1)
	SummaryBudget int  // cells for summary column (may be < CardModeMinBudget)
	RowsPerItem   int  // 1 in table mode, 2 in card mode
}

// CalculateListLayout is the pure decision function for list rendering.
// Given the inner content width/height and the widest issue ID in the
// dataset, it decides which mode to render in and how many items fit.
//
// Guarantees (see helpers_test.go):
//   - SummaryBudget == contentW - (maxIDW + fixed column widths + gaps).
//   - CardMode == (SummaryBudget < CardModeMinBudget).
//   - RowsPerItem is 1 iff !CardMode, else 2.
//   - ItemsVisible >= 1 for any contentH >= 1, and equals
//     floor((contentH-1) / RowsPerItem) otherwise (1 line reserved for
//     the header row).
func CalculateListLayout(contentW, contentH, maxIDW int) ListLayout {
	budget := contentW - maxIDW - listPrioW - listTypeW - listStatusW - listAssigneeW - listInterColGaps
	cardMode := budget < CardModeMinBudget

	rowsPerItem := 1
	if cardMode {
		rowsPerItem = 2
	}

	visibleLines := max(
		// reserve the header row
		contentH-1, 1)
	itemsVisible := max(visibleLines/rowsPerItem, 1)

	return ListLayout{
		CardMode:      cardMode,
		ItemsVisible:  itemsVisible,
		SummaryBudget: budget,
		RowsPerItem:   rowsPerItem,
	}
}

// ── Popup sliding window ───────────────────────────────────────────

// CalculateWindow returns the [start, end) slice bounds of the visible
// item window for a popup-style list that centres the cursor. Used
// when the entire viewport is dedicated to a short picker list and
// the expectation is "the cursor sits in the middle".
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
	start = max(cursor-maxVisible/2, 0)
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return start, end
}

// CalculateScrollWindow returns the [start, end) slice bounds of the
// visible item window for a long, scrollable list that follows FZF
// semantics: keep the cursor on-screen with minimal viewport movement.
// Unlike CalculateWindow it does NOT centre the cursor — the window
// only shifts when the cursor would otherwise fall off the top or
// bottom edge. The prevStart parameter is the viewport's previous
// offset, which anchors the window across successive frames so the
// list stays stable while the cursor moves within the visible range.
//
// Guarantees (see helpers_test.go):
//   - cursor is always within [start, end).
//   - start >= 0 and end <= total.
//   - end - start == min(total, maxVisible) when total > 0.
//   - If the cursor is already inside [prevStart, prevStart+maxVisible),
//     the window doesn't move (start == prevStart, end == min(prevStart+maxVisible, total)).
//   - Scrolling is monotone with the cursor: moving cursor up never
//     scrolls the viewport down, and vice versa.
func CalculateScrollWindow(cursor, prevStart, total, maxVisible int) (start, end int) {
	if total <= 0 || maxVisible <= 0 {
		return 0, 0
	}
	if total <= maxVisible {
		return 0, total
	}
	// Anchor to prevStart; if cursor moved above it, follow cursor up.
	start = min(cursor, prevStart)
	// If cursor moved past the bottom edge, shift start down so that
	// the cursor lands on the last visible row.
	if cursor >= start+maxVisible {
		start = cursor - maxVisible + 1
	}
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
