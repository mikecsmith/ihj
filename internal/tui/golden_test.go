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
	"github.com/mikecsmith/ihj/internal/document"
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

// goldenFixtures returns a rich set of work items for golden file tests.
// Uses fixed strings (no time.Now) for deterministic output.
func goldenFixtures() ([]*core.WorkItem, map[string]*core.WorkItem) {
	md := func(text string) *document.Node {
		doc, _ := document.ParseMarkdownString(text)
		return doc
	}

	items := []*core.WorkItem{
		{
			ID: "ENG-100", Summary: "User Authentication Overhaul",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "sarah@example.com",
				"reporter":   "mike@example.com",
				"created":    "15 Jan 2025",
				"updated":    "28 Jan 2025",
				"labels":     []string{"security", "q1-priority"},
				"components": []string{"Auth"},
			},
			DisplayFields: map[string]any{
				"assignee": "Sarah Chen",
				"reporter": "Mike Smith",
			},
			Description: md("## Overview\n\nReplace legacy session-based auth with **OAuth 2.0 + PKCE**.\n\n## Goals\n\n- Eliminate session token storage issues\n- Support SSO via SAML/OIDC\n- Reduce login friction by 40%"),
		},
		{
			ID: "ENG-101", Summary: "Implement OAuth 2.0 PKCE login flow",
			Type: "Story", Status: "In Review", ParentID: "ENG-100",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "alex@example.com",
				"reporter": "sarah@example.com",
				"created":  "18 Jan 2025",
				"updated":  "27 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
				"reporter": "Sarah Chen",
			},
			Description: md("Implement the full OAuth 2.0 Authorization Code flow with PKCE.\n\n## Acceptance Criteria\n\n- User redirected to IdP on Sign In\n- Valid access token issued on callback\n- PKCE verifier validated"),
		},
		{
			ID: "ENG-102", Summary: "Add SSO configuration admin panel",
			Type: "Story", Status: "To Do", ParentID: "ENG-100",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "jordan@example.com",
				"created":  "20 Jan 2025",
				"updated":  "25 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Jordan Lee",
			},
		},
		{
			ID: "ENG-103", Summary: "Write unit tests for token exchange",
			Type: "Sub-task", Status: "In Progress", ParentID: "ENG-101",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "alex@example.com",
				"created":  "22 Jan 2025",
				"updated":  "28 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
			},
		},
		{
			ID: "ENG-104", Summary: "Handle refresh token rotation",
			Type: "Sub-task", Status: "To Do", ParentID: "ENG-101",
			Fields: map[string]any{
				"priority": "Low",
				"assignee": "",
				"created":  "22 Jan 2025",
				"updated":  "22 Jan 2025",
			},
		},
		{
			ID: "ENG-200", Summary: "API Performance Improvements",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "mike@example.com",
				"created":  "10 Jan 2025",
				"updated":  "26 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Mike Smith",
			},
			Description: md("Improve API response times across all endpoints."),
		},
		{
			ID: "ENG-201", Summary: "Add Redis caching layer for hot paths",
			Type: "Task", Status: "Done", ParentID: "ENG-200",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "alex@example.com",
				"created":  "12 Jan 2025",
				"updated":  "24 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
			},
		},
		{
			ID: "ENG-300", Summary: "Login fails silently on expired IdP cert",
			Type: "Bug", Status: "To Do",
			Fields: map[string]any{
				"priority": "Highest",
				"assignee": "sarah@example.com",
				"created":  "25 Jan 2025",
				"updated":  "28 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Sarah Chen",
			},
			Description: md("When the IdP certificate expires, the login page shows a blank screen.\n\n**Steps to reproduce:**\n1. Set IdP cert to expired\n2. Navigate to /login\n3. Click Sign In\n\n**Expected:** Error message shown\n**Actual:** Blank screen"),
		},
	}

	registry := make(map[string]*core.WorkItem, len(items))
	for _, item := range items {
		registry[item.ID] = item
	}
	core.LinkChildren(registry)

	return items, registry
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
	items, registry := goldenFixtures()
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
	_, registry := goldenFixtures()
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys)
	dm.SetSize(80, 40)
	dm.SetIssue(registry["ENG-100"])

	got := stripANSI(dm.View())
	assertGolden(t, "detail_epic", got)
}

func TestGolden_DetailView_Bug(t *testing.T) {
	_, registry := goldenFixtures()
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
	_, registry := goldenFixtures()
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
	_, registry := goldenFixtures()
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
	items, _ := goldenFixtures()
	m := goldenAppModel(t, items)

	got := stripANSI(m.View().Content)
	assertGolden(t, "app_full", got)
}
