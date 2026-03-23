package tui

import (
	"image/color"
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

// ─────────────────────────────────────────────────────────────
// Tree prefix generation
// ─────────────────────────────────────────────────────────────

func TestBuildTreePrefix_RootDepth(t *testing.T) {
	got := buildTreePrefix(0, nil, true)
	if got != "" {
		t.Errorf("depth 0 should produce empty prefix, got %q", got)
	}
}

func TestBuildTreePrefix_FirstChild(t *testing.T) {
	// First child at depth 1 (not last): "  " indent + "├─ ".
	got := buildTreePrefix(1, nil, false)
	want := "  ├─ "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildTreePrefix_LastChild(t *testing.T) {
	got := buildTreePrefix(1, nil, true)
	want := "  └─ "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildTreePrefix_NestedWithContinuation(t *testing.T) {
	// Depth 2: 2 levels of indent (2 spaces each) + branch glyph.
	ancestors := []bool{false}
	got := buildTreePrefix(2, ancestors, true)
	want := "    └─ "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildTreePrefix_NestedLastParent(t *testing.T) {
	// Depth 2: 2 levels of indent + branch glyph.
	ancestors := []bool{true}
	got := buildTreePrefix(2, ancestors, false)
	want := "    ├─ "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildTreePrefix_DeeplyNested(t *testing.T) {
	// Depth 3: 3 levels of indent (2 spaces each = 6) + branch glyph.
	ancestors := []bool{false, true}
	got := buildTreePrefix(3, ancestors, true)
	want := "      └─ "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ─────────────────────────────────────────────────────────────
// flattenTree integration
// ─────────────────────────────────────────────────────────────

func TestFlattenTree_BasicHierarchy(t *testing.T) {
	child1 := &jira.IssueView{Key: "C-1", Summary: "Child 1", Type: "Task", Status: "To Do"}
	child2 := &jira.IssueView{Key: "C-2", Summary: "Child 2", Type: "Task", Status: "To Do"}
	parent := &jira.IssueView{
		Key:      "P-1",
		Summary:  "Parent",
		Type:     "Epic",
		Status:   "In Progress",
		Children: map[string]*jira.IssueView{"C-1": child1, "C-2": child2},
	}

	var items []listItem
	flattenTree([]*jira.IssueView{parent}, 0, nil, nil, &items, nil, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Root has no tree prefix.
	if items[0].TreePrefix != "" {
		t.Errorf("root should have empty prefix, got %q", items[0].TreePrefix)
	}
	if items[0].Depth != 0 {
		t.Errorf("root depth should be 0, got %d", items[0].Depth)
	}

	// First child at depth 1: "  " indent + branch glyph.
	wantFirst := "  ├─ "
	if items[1].TreePrefix != wantFirst {
		t.Errorf("first child prefix: got %q, want %q", items[1].TreePrefix, wantFirst)
	}
	if items[1].Depth != 1 {
		t.Errorf("first child depth should be 1, got %d", items[1].Depth)
	}

	// Last child at depth 1.
	wantLast := "  └─ "
	if items[2].TreePrefix != wantLast {
		t.Errorf("last child prefix: got %q, want %q", items[2].TreePrefix, wantLast)
	}
	if !items[2].IsLast {
		t.Error("last child should have IsLast=true")
	}
}

func TestFlattenTree_MultipleRoots(t *testing.T) {
	r1 := &jira.IssueView{Key: "R-1", Summary: "Root 1", Type: "Epic", Status: "Done"}
	r2 := &jira.IssueView{Key: "R-2", Summary: "Root 2", Type: "Epic", Status: "Done"}

	var items []listItem
	flattenTree([]*jira.IssueView{r1, r2}, 0, nil, nil, &items, nil, nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.TreePrefix != "" {
			t.Errorf("root item %s should have empty prefix", item.Issue.Key)
		}
	}
}

func TestFlattenTree_AncestorTypes(t *testing.T) {
	// Epic → Story → Sub-task: ancestor types track the chain.
	subtask := &jira.IssueView{Key: "S-1", Summary: "Sub", Type: "Sub-task", Status: "To Do"}
	story := &jira.IssueView{
		Key: "ST-1", Summary: "Story", Type: "Story", Status: "To Do",
		Children: map[string]*jira.IssueView{"S-1": subtask},
	}
	epic := &jira.IssueView{
		Key: "E-1", Summary: "Epic", Type: "Epic", Status: "In Progress",
		Children: map[string]*jira.IssueView{"ST-1": story},
	}

	var items []listItem
	flattenTree([]*jira.IssueView{epic}, 0, nil, nil, &items, nil, nil)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Root (Epic): no ancestors.
	if len(items[0].AncestorTypes) != 0 {
		t.Errorf("root AncestorTypes should be empty, got %v", items[0].AncestorTypes)
	}
	if items[0].ParentType != "" {
		t.Errorf("root ParentType should be empty, got %q", items[0].ParentType)
	}

	// Story: ancestor is Epic.
	if len(items[1].AncestorTypes) != 1 || items[1].AncestorTypes[0] != "Epic" {
		t.Errorf("story AncestorTypes should be [Epic], got %v", items[1].AncestorTypes)
	}
	if items[1].ParentType != "Epic" {
		t.Errorf("story ParentType should be Epic, got %q", items[1].ParentType)
	}

	// Sub-task: ancestors are [Epic, Story].
	if len(items[2].AncestorTypes) != 2 {
		t.Fatalf("subtask AncestorTypes should have 2 entries, got %v", items[2].AncestorTypes)
	}
	if items[2].AncestorTypes[0] != "Epic" || items[2].AncestorTypes[1] != "Story" {
		t.Errorf("subtask AncestorTypes should be [Epic, Story], got %v", items[2].AncestorTypes)
	}
	if items[2].ParentType != "Story" {
		t.Errorf("subtask ParentType should be Story, got %q", items[2].ParentType)
	}
}

// ─────────────────────────────────────────────────────────────
// Theme: TypeColor
// ─────────────────────────────────────────────────────────────

func TestTypeColor(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme)

	tests := []struct {
		input string
		want  color.Color
	}{
		{"Epic", theme.TypeEpic},
		{"epic", theme.TypeEpic},
		{"Initiative", theme.TypeInitiative},
		{"Story", theme.TypeStory},
		{"Bug", theme.TypeBug},
		{"Sub-task", theme.TypeSubtask},
		{"subtask", theme.TypeSubtask},
		{"Task", theme.TypeTask},
		{"Unknown", theme.TypeTask},
	}

	for _, tt := range tests {
		got := styles.TypeColor(tt.input)
		if got != tt.want {
			t.Errorf("TypeColor(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// Theme: StatusStyle
// ─────────────────────────────────────────────────────────────

func TestStatusStyle(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme)

	tests := []struct {
		input    string
		wantIcon string
		wantClr  color.Color
	}{
		{"Done", "✔", theme.StatusDone},
		{"Closed", "✔", theme.StatusDone},
		{"Blocked", "✘", theme.StatusBlocked},
		{"On Hold", "✘", theme.StatusBlocked},
		{"In Review", "◉", theme.StatusReview},
		{"QA", "◉", theme.StatusReview},
		{"In Progress", "▶", theme.StatusActive},
		{"Doing", "▶", theme.StatusActive},
		{"Refined", "★", theme.StatusReady},
		{"Ready", "★", theme.StatusReady},
		{"To Do", "○", theme.StatusDefault},
		{"Unknown Status", "○", theme.StatusDefault},
	}

	for _, tt := range tests {
		icon, clr := styles.StatusStyle(tt.input)
		if icon != tt.wantIcon {
			t.Errorf("StatusStyle(%q) icon = %q, want %q", tt.input, icon, tt.wantIcon)
		}
		if clr != tt.wantClr {
			t.Errorf("StatusStyle(%q) color = %v, want %v", tt.input, clr, tt.wantClr)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// containsAny helper
// ─────────────────────────────────────────────────────────────

func TestContainsAny(t *testing.T) {
	if !containsAny("in progress", "progress", "active") {
		t.Error("should match 'progress'")
	}
	if containsAny("to do", "progress", "active") {
		t.Error("should not match")
	}
	if containsAny("", "progress") {
		t.Error("empty string should not match")
	}
}

// ─────────────────────────────────────────────────────────────
// splitShellCommand (from bubbletea.go)
// ─────────────────────────────────────────────────────────────

func TestSplitShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"vim", []string{"vim"}},
		{"code --wait", []string{"code", "--wait"}},
		{`vim -c "set paste"`, []string{"vim", "-c", "set paste"}},
		{"  spaces  around  ", []string{"spaces", "around"}},
	}

	for _, tt := range tests {
		got := splitShellCommand(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitShellCommand(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitShellCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────
// isVimLike
// ─────────────────────────────────────────────────────────────

func TestIsVimLike(t *testing.T) {
	for _, name := range []string{"vim", "nvim", "vi", "Vim", "NVIM"} {
		if !isVimLike(name) {
			t.Errorf("isVimLike(%q) should be true", name)
		}
	}
	for _, name := range []string{"code", "nano", "emacs"} {
		if isVimLike(name) {
			t.Errorf("isVimLike(%q) should be false", name)
		}
	}
}
