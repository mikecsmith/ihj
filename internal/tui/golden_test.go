package tui_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// stripANSI removes ANSI escape sequences for stable golden file comparison.
// This avoids false diffs from terminal capability differences.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip CSI sequences: ESC [ ... final byte (0x40–0x7E)
			if i+1 < len(s) && s[i+1] == '[' {
				j := i + 2
				for j < len(s) && s[j] < 0x40 {
					j++
				}
				if j < len(s) {
					j++ // skip final byte
				}
				i = j
				continue
			}
			// Skip OSC sequences: ESC ] ... ST (ESC \ or BEL)
			if i+1 < len(s) && s[i+1] == ']' {
				j := i + 2
				for j < len(s) {
					if s[j] == '\x07' { // BEL
						j++
						break
					}
					if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			}
			// Skip other ESC sequences (ESC + one byte)
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
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

func goldenAppModel(t *testing.T, items []*core.WorkItem) tui.AppModel {
	t.Helper()

	ws := testutil.TestWorkspace()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	rt := testutil.NewTestRuntime(ui)
	provider := testutil.NewMockProvider()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := testutil.NewTestFactory(provider)

	m := tui.NewAppModel(rt, wsSess, factory, ws, "default", items, time.Time{}, ui)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(tui.AppModel)

	return m
}

// ── List View Golden Tests ───────────────────────────────────────

func TestGolden_ListView(t *testing.T) {
	items, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	lm := tui.NewListModel(registry, styles, ws.StatusWeights, ws.TypeOrderMap)
	lm.SetSize(120, 30)
	_ = items // registry already linked

	got := stripANSI(lm.View())
	assertGolden(t, "list_view", got)
}

// ── Detail View Golden Tests ─────────────────────────────────────

func TestGolden_DetailView_Epic(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(80, 60) // taller to fit children + comments
	dm.SetIssue(registry["ENG-100"])

	got := stripANSI(dm.View())
	assertGolden(t, "detail_epic", got)
}

func TestGolden_DetailView_Bug(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(80, 40)
	dm.SetIssue(registry["ENG-300"])

	got := stripANSI(dm.View())
	assertGolden(t, "detail_bug", got)
}

func TestGolden_DetailView_StoryWithChildren(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(80, 40)
	dm.SetIssue(registry["ENG-101"])

	got := stripANSI(dm.View())
	assertGolden(t, "detail_story_children", got)
}

func TestGolden_DetailView_Empty(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(80, 30)

	got := stripANSI(dm.View())
	assertGolden(t, "detail_empty", got)
}

// ── Full App Golden Tests ────────────────────────────────────────

func TestGolden_AppView(t *testing.T) {
	items, _ := testutil.RichTestItems()
	m := goldenAppModel(t, items)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_full", got)
}
