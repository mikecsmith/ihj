package tui_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

// stripANSI delegates to testutil.StripANSI for local convenience.
var stripANSI = testutil.StripANSI

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// drainCmds executes a cmd (which may be a batch) and feeds each resulting
// message back through Update. Used to initialize models for golden tests.
func drainCmds(t *testing.T, m *tui.AppModel, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			drainCmds(t, m, sub)
		}
		return
	}
	result, _ := m.Update(msg)
	*m = result.(tui.AppModel)
}

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s (run with -update-golden to create): %v", path, err)
	}

	if got != string(want) {
		// Find first differing line for a helpful error message.
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			g, w := "", ""
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Errorf("golden mismatch at line %d (run with -update-golden to accept):\n  got:  %q\n  want: %q", i+1, g, w)
				return
			}
		}
		t.Errorf("golden mismatch (lengths differ): got %d lines, want %d lines", len(gotLines), len(wantLines))
	}
}

// ── Integration smoke test ───────────────────────────────────────
//
// app_full is the single retained integration snapshot. Per-subcomponent
// rendering (detail sections, popups, list rows) is asserted semantically
// in *_test.go content-assertion tests + pure-function rule tests, which
// give meaningful failure messages. This golden exists only as a smoke
// test that the whole UI composes — if it changes, regenerate with
// `-update-golden` after reviewing the diff.

func TestGolden_AppView(t *testing.T) {
	items, _ := testutil.RichTestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, true)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_full", got)
}
