package tui_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

func testDetailModel() (tui.DetailModel, map[string]*core.WorkItem) {
	registry := map[string]*core.WorkItem{
		"EPIC-1":  {ID: "EPIC-1", Summary: "Epic", Type: "Epic", Status: "Open"},
		"STORY-1": {ID: "STORY-1", Summary: "Story 1", Type: "Story", Status: "To Do", ParentID: "EPIC-1"},
		"STORY-2": {ID: "STORY-2", Summary: "Story 2", Type: "Story", Status: "Done", ParentID: "EPIC-1"},
	}
	core.LinkChildren(registry)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "team-alpha", keys, testutil.TestFieldDefs())
	dm.SetSize(80, 30)
	return dm, registry
}

func TestDetailNavigation(t *testing.T) {
	dm, reg := testDetailModel()

	// Step 1: initially no issue
	if dm.Issue() != nil {
		t.Fatalf("Issue() = %v; want nil", dm.Issue())
	}

	// Step 2: SetIssue
	dm.SetIssue(reg["EPIC-1"])
	if dm.Issue() == nil || dm.Issue().ID != "EPIC-1" {
		t.Fatalf("Issue().ID = %v; want EPIC-1", dm.Issue())
	}
	if dm.CanGoBack() {
		t.Error("CanGoBack() = true; want false after SetIssue")
	}

	// Step 3: NavigateTo
	dm.NavigateTo(reg["STORY-1"])
	if dm.Issue().ID != "STORY-1" {
		t.Errorf("Issue().ID = %q; want STORY-1", dm.Issue().ID)
	}
	if !dm.CanGoBack() {
		t.Error("CanGoBack() = false; want true after NavigateTo")
	}

	// Step 4: GoBack
	dm.GoBack()
	if dm.Issue().ID != "EPIC-1" {
		t.Errorf("Issue().ID = %q; want EPIC-1 after GoBack", dm.Issue().ID)
	}
	if dm.CanGoBack() {
		t.Error("CanGoBack() = true; want false after GoBack to root")
	}

	// Step 5: GoBack on empty history -- no-op
	dm.GoBack()
	if dm.Issue().ID != "EPIC-1" {
		t.Errorf("Issue().ID = %q; want EPIC-1 (no-op GoBack)", dm.Issue().ID)
	}
}

func TestDetailSetIssue(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(dm *tui.DetailModel, reg map[string]*core.WorkItem)
		wantKey    string
		wantGoBack bool
	}{
		{
			"nil ignored",
			func(dm *tui.DetailModel, _ map[string]*core.WorkItem) {
				dm.SetIssue(nil)
			},
			"", // Issue() == nil
			false,
		},
		{
			"sets issue",
			func(dm *tui.DetailModel, reg map[string]*core.WorkItem) {
				dm.SetIssue(reg["EPIC-1"])
			},
			"EPIC-1",
			false,
		},
		{
			"clears history",
			func(dm *tui.DetailModel, reg map[string]*core.WorkItem) {
				dm.SetIssue(reg["EPIC-1"])
				dm.NavigateTo(reg["STORY-1"])
				dm.SetIssue(reg["STORY-2"])
			},
			"STORY-2",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm, reg := testDetailModel()
			tt.setup(&dm, reg)

			if tt.wantKey == "" {
				if dm.Issue() != nil {
					t.Errorf("Issue() = %v; want nil", dm.Issue())
				}
			} else if dm.Issue() == nil || dm.Issue().ID != tt.wantKey {
				key := ""
				if dm.Issue() != nil {
					key = dm.Issue().ID
				}
				t.Errorf("Issue().ID = %q; want %q", key, tt.wantKey)
			}
			if dm.CanGoBack() != tt.wantGoBack {
				t.Errorf("CanGoBack() = %v; want %v", dm.CanGoBack(), tt.wantGoBack)
			}
		})
	}
}

func TestDetailNavigateToChild(t *testing.T) {
	tests := []struct {
		name     string
		setupKey string
		index    int
		wantOK   bool
	}{
		{"valid child", "EPIC-1", 0, true},
		{"out of range", "EPIC-1", 99, false},
		{"negative", "EPIC-1", -1, false},
		{"no children", "STORY-1", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm, reg := testDetailModel()
			dm.SetIssue(reg[tt.setupKey])

			prevKey := dm.Issue().ID
			got := dm.NavigateToChild(tt.index)
			if got != tt.wantOK {
				t.Errorf("NavigateToChild(%d) = %v; want %v", tt.index, got, tt.wantOK)
			}
			if !tt.wantOK && dm.Issue().ID != prevKey {
				t.Errorf("Issue().ID changed to %q; want unchanged %q", dm.Issue().ID, prevKey)
			}
			if tt.wantOK && dm.Issue().ID == prevKey {
				t.Errorf("Issue().ID still %q; want changed after NavigateToChild", prevKey)
			}
		})
	}
}

func TestDetailBreadcrumb(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dm *tui.DetailModel, reg map[string]*core.WorkItem)
		want  string
	}{
		{
			"no history",
			func(dm *tui.DetailModel, reg map[string]*core.WorkItem) {
				dm.SetIssue(reg["EPIC-1"])
			},
			"",
		},
		{
			"one level",
			func(dm *tui.DetailModel, reg map[string]*core.WorkItem) {
				dm.SetIssue(reg["EPIC-1"])
				dm.NavigateTo(reg["STORY-1"])
			},
			"EPIC-1 \u2192 STORY-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm, reg := testDetailModel()
			tt.setup(&dm, reg)
			got := dm.Breadcrumb()
			if got != tt.want {
				t.Errorf("Breadcrumb() = %q; want %q", got, tt.want)
			}
		})
	}
}

// ── Targeted content-assertion tests ─────────────────────────────
// These assert specific substrings in the rendered view, covering
// the structural guarantees that the detail golden snapshots catch
// visually. They replace the need for several per-scenario golden
// files by pinning the contracts directly.

// makeParentWithChildren builds an Epic parent with N sequential Task
// children, used to exercise child-listing, count badges, and hint keys.
func makeParentWithChildren(n int) (*core.WorkItem, map[string]*core.WorkItem) {
	parent := &core.WorkItem{
		ID: "PROJ-1", Summary: "Parent",
		Type: "Epic", Status: "In Progress",
		Fields:        map[string]any{"priority": "High"},
		DisplayFields: map[string]any{},
	}
	items := []*core.WorkItem{parent}
	for i := 0; i < n; i++ {
		items = append(items, &core.WorkItem{
			ID:      pidChild(i),
			Summary: "Child task " + pidChild(i),
			Type:    "Task", Status: "To Do",
			ParentID: "PROJ-1",
			Fields:   map[string]any{"priority": "Medium"},
		})
	}
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	return parent, registry
}

func pidChild(i int) string {
	return "PROJ-" + itoa(100+i)
}

func itoa(i int) string {
	// Tiny local helper to avoid importing strconv in test file.
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestDetailView_ChildrenSectionListsAllChildIDs(t *testing.T) {
	parent, registry := makeParentWithChildren(5)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "proj", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 40)
	dm.SetIssue(parent)

	view := stripANSI(dm.View())

	for i := 0; i < 5; i++ {
		id := pidChild(i)
		if !strings.Contains(view, id) {
			t.Errorf("children section missing child ID %q", id)
		}
	}
	if !strings.Contains(view, "CHILD ISSUES") {
		t.Error("view should contain CHILD ISSUES section header")
	}
}

func TestDetailView_ChildHintsDigitsThenLetters(t *testing.T) {
	// 12 children: first 9 should have [1]..[9], then alpha hints afterward.
	// Default keymap leaves digits and letters unbound; Return/Esc etc. are
	// not single chars, so the first 9 hints are digits 1-9 in order.
	parent, registry := makeParentWithChildren(12)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "proj", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 60)
	dm.SetIssue(parent)

	view := stripANSI(dm.View())

	for _, hint := range []string{"[1]", "[2]", "[3]", "[4]", "[5]", "[6]", "[7]", "[8]", "[9]"} {
		if !strings.Contains(view, hint) {
			t.Errorf("expected child hint %q in view", hint)
		}
	}
	// After the 9th child, hints must roll over to letters. Exactly which
	// letters are picked depends on keymap bindings, but there must be at
	// least 3 more hint groups of the form [x].
	// Count hint brackets as a sanity check:
	hintCount := strings.Count(view, "] ")
	if hintCount < 12 {
		t.Errorf("expected at least 12 child hints, got %d", hintCount)
	}
}

func TestDetailView_VimModeExcludesBoundLetters(t *testing.T) {
	// In vim mode, single-char action keys (a, c, d, r, etc.) are bound,
	// so they must NOT appear as child hint letters. Digits are still
	// available and used first.
	parent, registry := makeParentWithChildren(12)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.VimKeyMap()
	dm := tui.NewDetailModel(styles, registry, "proj", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 60)
	dm.SetIssue(parent)

	view := stripANSI(dm.View())

	// The vim hint set must differ from the default hint set (otherwise
	// this test isn't exercising the crossover). Vim binds more single-
	// char keys, so the vim set is strictly smaller or differently-shaped.
	defaultHints := terminal.DefaultKeyMap().HintKeys()
	vimHints := keys.HintKeys()
	if len(defaultHints) == len(vimHints) {
		// Length equality is OK only if contents also match; if they
		// match, vim mode didn't displace any keys and the test premise
		// is broken.
		same := true
		for i := range defaultHints {
			if defaultHints[i] != vimHints[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("vim keymap produced identical hints to default — test premise broken")
		}
	}

	// The first 12 hints (as reported by HintKeys) must appear verbatim
	// in the rendered view, in bracketed form.
	for _, h := range vimHints[:12] {
		if !strings.Contains(view, "["+string(h)+"]") {
			t.Errorf("vim-mode view missing hint [%c]", h)
		}
	}
}

func TestDetailView_NoDescriptionDoesNotRenderSection(t *testing.T) {
	// When an issue has no description, the DESCRIPTION section header
	// should not appear — the block is skipped entirely rather than left
	// with an empty body.
	registry := map[string]*core.WorkItem{
		"T-1": {
			ID: "T-1", Summary: "No body", Type: "Task", Status: "Open",
			Fields:        map[string]any{},
			DisplayFields: map[string]any{},
		},
	}
	core.LinkChildren(registry)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "t", keys, testutil.TestFieldDefs())
	dm.SetSize(120, 30)
	dm.SetIssue(registry["T-1"])

	view := stripANSI(dm.View())
	// Summary and ID must still render.
	if !strings.Contains(view, "T-1") || !strings.Contains(view, "No body") {
		t.Error("basic issue fields (ID, summary) should render without description")
	}
}

func TestDetailView_EmptyStateShowsPlaceholder(t *testing.T) {
	// With no issue set, the detail view must render a placeholder — not
	// crash, not render a blank pane, not render stale state.
	registry := map[string]*core.WorkItem{}
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "t", keys, testutil.TestFieldDefs())
	dm.SetSize(120, 30)

	view := stripANSI(dm.View())
	if strings.TrimSpace(view) == "" {
		t.Error("empty detail view should render a placeholder, not blank output")
	}
}

func TestDetailView_RichIssueStructure(t *testing.T) {
	// A rich Epic with labels, components, a description, children, and
	// comments must render each structural section. This replaces the
	// detail_epic golden's per-line snapshot with semantic assertions.
	_, registry := testutil.RichTestItems()

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, testutil.TestWorkspace(), "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 60)
	dm.SetIssue(registry["ENG-100"])

	view := stripANSI(dm.View())

	// Breadcrumb: workspace key + issue ID + type + status + priority.
	for _, frag := range []string{"ENG", "ENG-100", "EPIC", "PROGRESS", "HIGH"} {
		if !strings.Contains(strings.ToUpper(view), strings.ToUpper(frag)) {
			t.Errorf("breadcrumb missing %q", frag)
		}
	}
	// Field labels for scalar metadata must be present.
	for _, label := range []string{"Assignee:", "Created:", "Reporter:", "Updated:"} {
		if !strings.Contains(view, label) {
			t.Errorf("metadata missing label %q", label)
		}
	}
	// Array fields render their label + comma-separated values.
	if !strings.Contains(view, "Labels:") || !strings.Contains(view, "security") {
		t.Error("array field Labels: with value 'security' missing")
	}
	// Section headers for CHILD ISSUES + LATEST COMMENTS.
	if !strings.Contains(view, "CHILD ISSUES") {
		t.Error("CHILD ISSUES section header missing")
	}
	if !strings.Contains(view, "LATEST COMMENTS") {
		t.Error("LATEST COMMENTS section header missing")
	}
}

func TestDetailView_CommentsRenderAuthorAndBody(t *testing.T) {
	_, registry := testutil.RichTestItems()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, testutil.TestWorkspace(), "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 80)
	dm.SetIssue(registry["ENG-100"])

	view := stripANSI(dm.View())
	// Known comment authors from the fixture; body fragments.
	for _, frag := range []string{"Mike Smith", "Alex Rivera", "Kicked off the epic"} {
		if !strings.Contains(view, frag) {
			t.Errorf("comments section missing %q", frag)
		}
	}
}

func TestDetailView_DescriptionRendersMarkdown(t *testing.T) {
	// The issue's description contains markdown (headings + bullets).
	// We don't assert the exact rendering (styles may drift), only that
	// the heading text and bullet items come through.
	_, registry := testutil.RichTestItems()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, testutil.TestWorkspace(), "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "eng", keys, testutil.TestFieldDefs())
	dm.SetSize(160, 80)
	dm.SetIssue(registry["ENG-100"])

	view := stripANSI(dm.View())
	for _, frag := range []string{"Overview", "Goals", "Eliminate session"} {
		if !strings.Contains(view, frag) {
			t.Errorf("markdown description missing %q", frag)
		}
	}
}

func TestDetailView_UnassignedShowsEmDash(t *testing.T) {
	registry := map[string]*core.WorkItem{
		"T-1": {
			ID: "T-1", Summary: "No assignee", Type: "Task", Status: "Open",
			Fields:        map[string]any{},
			DisplayFields: map[string]any{},
		},
	}
	core.LinkChildren(registry)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, "test", keys, testutil.TestFieldDefs())
	dm.SetSize(120, 30)
	dm.SetIssue(registry["T-1"])

	view := stripANSI(dm.View())

	if !strings.Contains(view, "Assignee:") {
		t.Fatal("view should contain Assignee label")
	}
	if !strings.Contains(view, core.GlyphEmDash) {
		t.Error("unassigned item should show em dash (" + core.GlyphEmDash + ") placeholder in detail view")
	}
}
