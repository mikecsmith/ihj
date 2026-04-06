package tui

// This file contains generic sliding-window helpers used by multiple
// TUI models (popup and list). They are pure functions with no
// dependencies on Bubble Tea model state.

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

// ── List scroll window ────────────────────────────────────────────

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
