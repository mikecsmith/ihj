package tui_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
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

func goldenAppModel(t *testing.T, items []*core.WorkItem) tui.AppModel {
	t.Helper()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, true)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	return m
}

// ── List View Golden Tests ───────────────────────────────────────

func TestGolden_ListView(t *testing.T) {
	items, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	lm := tui.NewListModel(registry, styles, ws.StatusOrderMap, ws.TypeOrderMap)
	lm.SetSize(160, 30)
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
	dm.SetSize(160, 60) // taller to fit children + comments
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
	dm.SetSize(160, 40)
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
	dm.SetSize(160, 40)
	dm.SetIssue(registry["ENG-101"])

	got := stripANSI(dm.View())
	assertGolden(t, "detail_story_children", got)
}

func TestGolden_DetailView_NoDescription(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(160, 30)
	dm.SetIssue(registry["ENG-102"]) // Has no description

	got := stripANSI(dm.View())
	assertGolden(t, "detail_no_description", got)
}

func TestGolden_DetailView_Empty(t *testing.T) {
	_, registry := testutil.RichTestItems()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(160, 30)

	got := stripANSI(dm.View())
	assertGolden(t, "detail_empty", got)
}

func TestGolden_DetailView_ManyChildren(t *testing.T) {
	// Build an issue with 12 children to exercise digit→letter hint crossover.
	parent := &core.WorkItem{
		ID: "PROJ-1", Summary: "Parent with many children",
		Type: "Epic", Status: "In Progress",
		Fields: map[string]any{
			"priority": "High",
			"assignee": "dev@example.com",
			"created":  "01 Jan 2025",
			"updated":  "15 Jan 2025",
		},
		DisplayFields: map[string]any{"assignee": "Dev User"},
	}
	items := []*core.WorkItem{parent}
	for i := range 12 {
		child := &core.WorkItem{
			ID:       fmt.Sprintf("PROJ-%d", 100+i),
			Summary:  fmt.Sprintf("Child task number %d", i),
			Type:     "Task",
			Status:   "To Do",
			ParentID: "PROJ-1",
			Fields:   map[string]any{"priority": "Medium"},
		}
		items = append(items, child)
	}
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(160, 60)
	dm.SetIssue(parent)

	got := stripANSI(dm.View())
	assertGolden(t, "detail_many_children", got)
}

func TestGolden_DetailView_ManyChildren_VimMode(t *testing.T) {
	// Vim mode: single-char action keys are taken, so letters like r, f, a, etc.
	// are excluded from hints. Verifies the gaps in letter hints.
	parent := &core.WorkItem{
		ID: "PROJ-1", Summary: "Parent with many children (vim)",
		Type: "Epic", Status: "In Progress",
		Fields: map[string]any{
			"priority": "High",
			"assignee": "dev@example.com",
			"created":  "01 Jan 2025",
			"updated":  "15 Jan 2025",
		},
		DisplayFields: map[string]any{"assignee": "Dev User"},
	}
	items := []*core.WorkItem{parent}
	for i := range 12 {
		child := &core.WorkItem{
			ID:       fmt.Sprintf("PROJ-%d", 100+i),
			Summary:  fmt.Sprintf("Child task number %d", i),
			Type:     "Task",
			Status:   "To Do",
			ParentID: "PROJ-1",
			Fields:   map[string]any{"priority": "Medium"},
		}
		items = append(items, child)
	}
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.VimKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(160, 60)
	dm.SetIssue(parent)

	got := stripANSI(dm.View())
	assertGolden(t, "detail_many_children_vim", got)
}

// ── Popup Golden Tests ───────────────────────────────────────────

func TestGolden_PopupSelect_Transition(t *testing.T) {
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(160, 40)
	p.ShowSelect("transition", "Transition: ENG-100", []string{
		"To Do",
		"In Progress",
		"In Review",
		"Done",
	})

	got := stripANSI(p.View())
	assertGolden(t, "popup_select_transition", got)
}

func TestGolden_PopupSelect_LongList(t *testing.T) {
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(160, 40)
	p.ShowSelect("transition", "Transition: ENG-200", []string{
		"Backlog",
		"Refinement",
		"Ready for Dev",
		"To Do",
		"In Progress",
		"In Review",
		"QA",
		"Staging",
		"Ready for Release",
		"Done",
		"Closed",
		"Won't Fix",
	})

	got := stripANSI(p.View())
	assertGolden(t, "popup_select_long", got)
}

func TestGolden_PopupSelect_Filter(t *testing.T) {
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(160, 40)
	p.ShowSelect("filter", "Switch Filter", []string{
		"My Issues",
		"Current Sprint",
		"Unassigned",
		"Recently Updated",
	})

	got := stripANSI(p.View())
	assertGolden(t, "popup_select_filter", got)
}

func TestGolden_PopupInput_Comment(t *testing.T) {
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(160, 40)
	p.ShowInput("comment", "Comment: ENG-100", "Type your comment...")

	got := stripANSI(p.View())
	assertGolden(t, "popup_input_comment", got)
}

func TestGolden_PopupInput_Extract(t *testing.T) {
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(160, 40)
	p.ShowInput("extract", "LLM Extract: ENG-100", "Describe the sub-tasks to extract...")

	got := stripANSI(p.View())
	assertGolden(t, "popup_input_extract", got)
}

// ── Full App Golden Tests ────────────────────────────────────────

func TestGolden_AppView(t *testing.T) {
	items, _ := testutil.RichTestItems()
	m := goldenAppModel(t, items)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_full", got)
}

func TestGolden_AppView_VimMode(t *testing.T) {
	items, _ := testutil.RichTestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, true, nil, 0, true)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_full_vim", got)
}

func TestGolden_AppView_NoHelpBar(t *testing.T) {
	items, _ := testutil.RichTestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, false)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_no_helpbar", got)
}

func TestGolden_AppView_NoHelpBar_VimMode(t *testing.T) {
	items, _ := testutil.RichTestItems()
	ui := tui.NewBubbleTeaUI()
	ui.EditorCmd = "vim"
	h := testutil.NewTestHarness(t, ui)

	m := tui.NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, true, nil, 0, false)

	initCmd := m.Init()
	drainCmds(t, &m, initCmd)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = result.(tui.AppModel)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_no_helpbar_vim", got)
}
