package tui

import (
	"image/color"
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

// ─────────────────────────────────────────────────────────────
// flattenTree integration (Data Structure Testing)
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

	if items[0].Depth != 0 {
		t.Errorf("root depth should be 0, got %d", items[0].Depth)
	}

	if items[1].Depth != 1 {
		t.Errorf("first child depth should be 1, got %d", items[1].Depth)
	}

	if !items[2].IsLast {
		t.Errorf("items[2].IsLast = false; want true")
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
		if item.Depth != 0 {
			t.Errorf("root item %s should have depth 0", item.Issue.Key)
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
// Theme tests
// ─────────────────────────────────────────────────────────────

func TestTypeColor(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme, nil) // Passing nil for BoardConfig is safe for fallbacks

	tests := []struct {
		input string
		want  color.Color
	}{
		{input: "Initiative", want: theme.TypeInitiative},
		{"Epic", theme.TypeEpic},
		{"epic", theme.TypeEpic},
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

func TestStatusStyle(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme, nil) // Passing nil for BoardConfig

	tests := []struct {
		input    string
		wantIcon string
		wantClr  color.Color
	}{
		{"Done", "✔", theme.StatusDone},
		{"Blocked", "✘", theme.StatusBlocked},
		{"In Review", "◉", theme.StatusReview},
		{"In Progress", "▶", theme.StatusActive},
		{"Refined", "★", theme.StatusReady},
		{"To Do", "○", theme.StatusDefault},
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
// Utility tests
// ─────────────────────────────────────────────────────────────

func TestContainsAny(t *testing.T) {
	if !containsAny("in progress", "progress", "active") {
		t.Error("containsAny(\"in progress\", \"progress\", \"active\") = false; want true")
	}
	if containsAny("to do", "progress", "active") {
		t.Error("containsAny(\"to do\", \"progress\", \"active\") = true; want false")
	}
}

func TestSplitShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"vim", []string{"vim"}},
		{"code --wait", []string{"code", "--wait"}},
		{`vim -c "set paste"`, []string{"vim", "-c", "set paste"}},
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

func TestIsVimLike(t *testing.T) {
	for _, name := range []string{"vim", "nvim", "vi", "Vim"} {
		if !isVimLike(name) {
			t.Errorf("isVimLike(%q) should be true", name)
		}
	}
	for _, name := range []string{"code", "nano"} {
		if isVimLike(name) {
			t.Errorf("isVimLike(%q) should be false", name)
		}
	}
}
