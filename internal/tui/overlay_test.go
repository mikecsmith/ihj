package tui_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

// CompositeOverlay is a thin wrapper around lipgloss.NewCompositor with a
// Z-ordered two-layer setup: base at Z=0, overlay at Z=1. These rule
// tests pin the contract we rely on — overlay covers its bounding box,
// base shows through elsewhere, empty overlay is a no-op — without
// snapshotting the ANSI output. If the lipgloss compositor API changes
// in a way that breaks any of these, our overlay rendering is broken.

// baseGrid builds a `h x w` rectangle filled with '.' characters.
func baseGrid(w, h int) string {
	row := strings.Repeat(".", w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// cellAt returns the character at (row, col) in s, or ' ' if out of bounds.
func cellAt(s string, row, col int) byte {
	lines := strings.Split(s, "\n")
	if row < 0 || row >= len(lines) {
		return ' '
	}
	if col < 0 || col >= len(lines[row]) {
		return ' '
	}
	return lines[row][col]
}

func TestCompositeOverlay_EmptyOverlayIsNoOp(t *testing.T) {
	base := baseGrid(10, 5)
	got := tui.CompositeOverlay(base, "", 0, 0)
	if got != base {
		t.Errorf("empty overlay should return base unchanged\n got: %q\nwant: %q", got, base)
	}
}

func TestCompositeOverlay_OverlayReplacesCellsInBoundingBox(t *testing.T) {
	base := baseGrid(20, 6)
	overlay := "XXX\nXXX"
	got := testutil.StripANSI(tui.CompositeOverlay(base, overlay, 2, 5))

	// Overlay sits at rows 2..3, cols 5..7. Every cell inside that box
	// must be 'X'; every cell outside must be '.'.
	for r := 0; r < 6; r++ {
		for c := 0; c < 20; c++ {
			inBox := r >= 2 && r <= 3 && c >= 5 && c <= 7
			want := byte('.')
			if inBox {
				want = 'X'
			}
			if got := cellAt(got, r, c); got != want {
				t.Errorf("cell (%d,%d) = %q, want %q", r, c, got, want)
			}
		}
	}
}

func TestCompositeOverlay_TopLeftPositioning(t *testing.T) {
	base := baseGrid(10, 3)
	got := testutil.StripANSI(tui.CompositeOverlay(base, "#", 0, 0))
	if cellAt(got, 0, 0) != '#' {
		t.Errorf("overlay at (0,0) should set cell (0,0) to '#'; got %q", cellAt(got, 0, 0))
	}
	if cellAt(got, 0, 1) != '.' {
		t.Errorf("overlay should not bleed into (0,1); got %q", cellAt(got, 0, 1))
	}
}

func TestCompositeOverlay_ZIndexOverlayWins(t *testing.T) {
	// Put a multi-char overlay on a base that also has non-space content
	// at the same cells. The overlay (Z=1) must win over base (Z=0).
	base := "ABCDE\nFGHIJ\nKLMNO"
	overlay := "!!\n!!"
	got := testutil.StripANSI(tui.CompositeOverlay(base, overlay, 1, 1))

	// Expected: rows 1..2, cols 1..2 become '!'; everything else unchanged.
	want := []string{
		"ABCDE",
		"F!!IJ",
		"K!!NO",
	}
	lines := strings.Split(got, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(lines), got)
	}
	for i := 0; i < 3; i++ {
		if lines[i] != want[i] {
			t.Errorf("row %d: got %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestCompositeOverlay_OutOfBandsOverlayIsClipped(t *testing.T) {
	// An overlay positioned past the right edge of the base should not
	// crash and should not extend the base's visible width unexpectedly.
	base := baseGrid(5, 2)
	got := testutil.StripANSI(tui.CompositeOverlay(base, "Z", 0, 20))
	// First 5 cells of row 0 should still be base '.'.
	for c := 0; c < 5; c++ {
		if cellAt(got, 0, c) != '.' {
			t.Errorf("base cell (0,%d) should remain '.'; got %q", c, cellAt(got, 0, c))
		}
	}
}
