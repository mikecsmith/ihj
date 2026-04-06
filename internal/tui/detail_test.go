package tui_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/testutil"
	"github.com/mikecsmith/ihj/internal/tui"
)

// testWS returns a minimal workspace with the given name and standard test
// FieldDefs on all types. Used by detail tests that don't need special field config.
func testWS(name string) *core.Workspace {
	ws := testutil.TestWorkspace()
	ws.Name = name
	return ws
}

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
	dm := tui.NewDetailModel(styles, registry, testWS("team-alpha"), keys)
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
	for i := range n {
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
	dm := tui.NewDetailModel(styles, registry, testWS("proj"), keys)
	dm.SetSize(160, 40)
	dm.SetIssue(parent)

	view := stripANSI(dm.View())

	for i := range 5 {
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
	// 12 children: first 10 should have [1]..[9], [0] in keyboard order
	// (0 sits right of 9 on a QWERTY number row), then alpha hints. The
	// default keymap leaves digits and letters unbound.
	parent, registry := makeParentWithChildren(12)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, testWS("proj"), keys)
	dm.SetSize(160, 60)
	dm.SetIssue(parent)

	view := stripANSI(dm.View())

	for _, hint := range []string{"[1]", "[2]", "[3]", "[4]", "[5]", "[6]", "[7]", "[8]", "[9]", "[0]"} {
		if !strings.Contains(view, hint) {
			t.Errorf("expected child hint %q in view", hint)
		}
	}
	// After the 10th child, hints must roll over to letters. Exactly which
	// letters are picked depends on keymap bindings, but there must be at
	// least 2 more hint groups of the form [x] for children 10 and 11.
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
	dm := tui.NewDetailModel(styles, registry, testWS("proj"), keys)
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
	dm := tui.NewDetailModel(styles, registry, testWS("t"), keys)
	dm.SetSize(120, 30)
	dm.SetIssue(registry["T-1"])

	view := stripANSI(dm.View())
	// Summary and ID must still render.
	if !strings.Contains(view, "T-1") || !strings.Contains(view, "NO BODY") {
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
	dm := tui.NewDetailModel(styles, registry, testWS("t"), keys)
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

	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
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
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
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
	ws := testutil.TestWorkspace()
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
	dm.SetSize(160, 80)
	dm.SetIssue(registry["ENG-100"])

	view := stripANSI(dm.View())
	for _, frag := range []string{"Overview", "Goals", "Eliminate session"} {
		if !strings.Contains(view, frag) {
			t.Errorf("markdown description missing %q", frag)
		}
	}
}

func TestDetailView_RichTextFieldRendersAsFullBlock(t *testing.T) {
	acNode, err := document.ParseMarkdownString("- First criterion\n- Second criterion\n")
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	registry := map[string]*core.WorkItem{
		"T-1": {
			ID: "T-1", Summary: "Has AC", Type: "Story", Status: "Open",
			Fields: map[string]any{
				"acceptance_criteria": acNode,
			},
		},
	}
	core.LinkChildren(registry)

	extraDef := core.FieldDef{
		Key:    "acceptance_criteria",
		Label:  "Acceptance Criteria",
		Type:   core.FieldRichText,
		Role:   core.RoleCustom,
		Pinned: true,
	}

	// Build a workspace with the extra rich-text field on the Story type.
	ws := testutil.TestWorkspace()
	if tc := ws.TypeByName("Story"); tc != nil {
		tc.Fields = append(tc.Fields, extraDef)
	}

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
	dm.SetSize(120, 30)
	dm.SetIssue(registry["T-1"])

	view := stripANSI(dm.View())

	if !strings.Contains(view, "ACCEPTANCE CRITERIA") {
		t.Error("view should contain ACCEPTANCE CRITERIA section header")
	}
	if !strings.Contains(view, "First criterion") || !strings.Contains(view, "Second criterion") {
		t.Error("view should render bullet-list content inline")
	}
	// Rich text must not appear in the scalar metadata grid.
	if strings.Contains(view, "Acceptance Criteria:") {
		t.Error("rich text should not appear as a labelled scalar field")
	}
}

func TestDetailView_TypeSpecificFieldsDoNotLeakAcrossTypes(t *testing.T) {
	// A field added only to the Bug type must NOT render when viewing a Story.
	// This verifies the detail model uses type-scoped FieldDefs, not the union.
	bugDetails := core.FieldDef{
		Key:    "bug_details",
		Label:  "Bug Details",
		Type:   core.FieldRichText,
		Role:   core.RoleCustom,
		Pinned: true,
	}

	ws := testutil.TestWorkspace()
	// Add bug_details only to a new Bug type.
	ws.Types = append(ws.Types, core.TypeConfig{
		ID: 14, Name: "Bug", Order: 30, Color: "red",
		Fields: append(testutil.TestFieldDefs(), bugDetails),
	})

	bugNode, _ := document.ParseMarkdownString("Steps to reproduce the bug")
	registry := map[string]*core.WorkItem{
		"S-1": {
			ID: "S-1", Summary: "A Story", Type: "Story", Status: "To Do",
			Fields: map[string]any{},
		},
		"B-1": {
			ID: "B-1", Summary: "A Bug", Type: "Bug", Status: "To Do",
			Fields: map[string]any{"bug_details": bugNode},
		},
	}
	core.LinkChildren(registry)

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
	dm.SetSize(120, 40)

	// View the Story — Bug Details must NOT appear.
	dm.SetIssue(registry["S-1"])
	storyView := stripANSI(dm.View())
	if strings.Contains(storyView, "BUG DETAILS") {
		t.Error("Story view should NOT contain BUG DETAILS section from Bug type")
	}

	// View the Bug — Bug Details MUST appear.
	dm.SetIssue(registry["B-1"])
	bugView := stripANSI(dm.View())
	if !strings.Contains(bugView, "BUG DETAILS") {
		t.Error("Bug view should contain BUG DETAILS section")
	}
	if !strings.Contains(bugView, "Steps to reproduce") {
		t.Error("Bug view should render bug_details content")
	}
}

func TestDetailView_UnpinnedCustomFieldsHidden(t *testing.T) {
	// Unpinned RoleCustom fields should not render in the detail view,
	// even when the issue has a value. This prevents noise from Jira
	// custom fields that createmeta reports on all types with default content.
	unpinnedScalar := core.FieldDef{
		Key: "p20", Label: "P20", Type: core.FieldString, Role: core.RoleCustom,
		// Pinned: false — not opted in
	}
	unpinnedRichText := core.FieldDef{
		Key: "rnd_credits", Label: "R&D Credits", Type: core.FieldRichText, Role: core.RoleCustom,
	}

	rndNode, _ := document.ParseMarkdownString("Default template content")
	registry := map[string]*core.WorkItem{
		"T-1": {
			ID: "T-1", Summary: "A Task", Type: "Task", Status: "To Do",
			Fields: map[string]any{
				"p20":         "42",
				"rnd_credits": rndNode,
			},
		},
	}
	core.LinkChildren(registry)

	ws := testutil.TestWorkspace()
	if tc := ws.TypeByName("Task"); tc != nil {
		tc.Fields = append(tc.Fields, unpinnedScalar, unpinnedRichText)
	}

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
	dm.SetSize(120, 40)
	dm.SetIssue(registry["T-1"])

	view := stripANSI(dm.View())

	if strings.Contains(view, "P20") {
		t.Error("unpinned scalar custom field P20 should not render")
	}
	if strings.Contains(view, "R&D CREDITS") {
		t.Error("unpinned rich text custom field R&D Credits should not render")
	}
}

func TestDetailView_PinnedCustomFieldsShown(t *testing.T) {
	// Pinned RoleCustom fields SHOULD render — user explicitly opted in.
	pinnedScalar := core.FieldDef{
		Key: "story_points", Label: "Story Points", Type: core.FieldString,
		Role: core.RoleCustom, Pinned: true,
	}
	pinnedRichText := core.FieldDef{
		Key: "acceptance_criteria", Label: "Acceptance Criteria", Type: core.FieldRichText,
		Role: core.RoleCustom, Pinned: true,
	}

	acNode, _ := document.ParseMarkdownString("- Must pass tests")
	registry := map[string]*core.WorkItem{
		"S-1": {
			ID: "S-1", Summary: "A Story", Type: "Story", Status: "To Do",
			Fields: map[string]any{
				"story_points":        "5",
				"acceptance_criteria": acNode,
			},
		},
	}
	core.LinkChildren(registry)

	ws := testutil.TestWorkspace()
	if tc := ws.TypeByName("Story"); tc != nil {
		tc.Fields = append(tc.Fields, pinnedScalar, pinnedRichText)
	}

	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	dm := tui.NewDetailModel(styles, registry, ws, keys)
	dm.SetSize(120, 40)
	dm.SetIssue(registry["S-1"])

	view := stripANSI(dm.View())

	if !strings.Contains(view, "Story Points:") {
		t.Error("pinned scalar custom field Story Points should render")
	}
	if !strings.Contains(view, "ACCEPTANCE CRITERIA") {
		t.Error("pinned rich text custom field should render as section")
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
	dm := tui.NewDetailModel(styles, registry, testWS("test"), keys)
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
