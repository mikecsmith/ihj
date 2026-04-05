package tui_test

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

// stripANSI is shared across _test.go files in this package.
var stripANSI = testutil.StripANSI

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// TestGolden_AppView is the single integration smoke test — it verifies
// that the full UI (chrome + list + detail + overlays) composes without
// drift. Per-subcomponent rendering is asserted semantically in other
// *_test.go files, which give meaningful failure messages. This golden
// exists only as a "the whole thing renders" canary — if it changes,
// regenerate with `-update-golden` after reviewing the diff.
func TestGolden_AppView(t *testing.T) {
	const goldenPath = "testdata/app_full.golden"

	items, _ := testutil.RichTestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, true)

	// Drain Init's batched cmds so deferred state (e.g. workspace load)
	// settles before we snapshot the view.
	var drain func(cmd tea.Cmd)
	drain = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		msg := cmd()
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				drain(sub)
			}
			return
		}
		result, _ := m.Update(msg)
		m = result.(tui.AppModel)
	}
	drain(m.Init())

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	got := stripANSI(m.View().Content)

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading %s (run with -update-golden to create): %v", goldenPath, err)
	}
	if got == string(want) {
		return
	}
	// Report the first differing line.
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("golden mismatch at line %d (run with -update-golden to accept):\n  got:  %q\n  want: %q", i+1, g, w)
		}
	}
	t.Fatalf("golden mismatch (lengths differ): got %d lines, want %d", len(gotLines), len(wantLines))
}
