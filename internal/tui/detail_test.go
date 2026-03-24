package tui

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

func testDetailModel() (DetailModel, map[string]*jira.IssueView) {
	registry := map[string]*jira.IssueView{
		"EPIC-1":  {Key: "EPIC-1", Summary: "Epic", Type: "Epic", Status: "Open", Children: make(map[string]*jira.IssueView)},
		"STORY-1": {Key: "STORY-1", Summary: "Story 1", Type: "Story", Status: "To Do", ParentKey: "EPIC-1", Children: make(map[string]*jira.IssueView)},
		"STORY-2": {Key: "STORY-2", Summary: "Story 2", Type: "Story", Status: "Done", ParentKey: "EPIC-1", Children: make(map[string]*jira.IssueView)},
	}
	jira.LinkChildren(registry)

	theme := DefaultTheme()
	styles := NewStyles(theme, nil)
	keys := DefaultKeyMap()
	dm := NewDetailModel(styles, registry, "team-alpha", keys)
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
	if dm.Issue() == nil || dm.Issue().Key != "EPIC-1" {
		t.Fatalf("Issue().Key = %v; want EPIC-1", dm.Issue())
	}
	if dm.CanGoBack() {
		t.Error("CanGoBack() = true; want false after SetIssue")
	}

	// Step 3: NavigateTo
	dm.NavigateTo(reg["STORY-1"])
	if dm.Issue().Key != "STORY-1" {
		t.Errorf("Issue().Key = %q; want STORY-1", dm.Issue().Key)
	}
	if !dm.CanGoBack() {
		t.Error("CanGoBack() = false; want true after NavigateTo")
	}

	// Step 4: GoBack
	dm.GoBack()
	if dm.Issue().Key != "EPIC-1" {
		t.Errorf("Issue().Key = %q; want EPIC-1 after GoBack", dm.Issue().Key)
	}
	if dm.CanGoBack() {
		t.Error("CanGoBack() = true; want false after GoBack to root")
	}

	// Step 5: GoBack on empty history — no-op
	dm.GoBack()
	if dm.Issue().Key != "EPIC-1" {
		t.Errorf("Issue().Key = %q; want EPIC-1 (no-op GoBack)", dm.Issue().Key)
	}
}

func TestDetailSetIssue(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(dm *DetailModel, reg map[string]*jira.IssueView)
		wantKey    string
		wantGoBack bool
	}{
		{
			"nil ignored",
			func(dm *DetailModel, _ map[string]*jira.IssueView) {
				dm.SetIssue(nil)
			},
			"", // Issue() == nil
			false,
		},
		{
			"sets issue",
			func(dm *DetailModel, reg map[string]*jira.IssueView) {
				dm.SetIssue(reg["EPIC-1"])
			},
			"EPIC-1",
			false,
		},
		{
			"clears history",
			func(dm *DetailModel, reg map[string]*jira.IssueView) {
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
			} else if dm.Issue() == nil || dm.Issue().Key != tt.wantKey {
				key := ""
				if dm.Issue() != nil {
					key = dm.Issue().Key
				}
				t.Errorf("Issue().Key = %q; want %q", key, tt.wantKey)
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

			prevKey := dm.Issue().Key
			got := dm.NavigateToChild(tt.index)
			if got != tt.wantOK {
				t.Errorf("NavigateToChild(%d) = %v; want %v", tt.index, got, tt.wantOK)
			}
			if !tt.wantOK && dm.Issue().Key != prevKey {
				t.Errorf("Issue().Key changed to %q; want unchanged %q", dm.Issue().Key, prevKey)
			}
			if tt.wantOK && dm.Issue().Key == prevKey {
				t.Errorf("Issue().Key still %q; want changed after NavigateToChild", prevKey)
			}
		})
	}
}

func TestDetailBreadcrumb(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dm *DetailModel, reg map[string]*jira.IssueView)
		want  string
	}{
		{
			"no history",
			func(dm *DetailModel, reg map[string]*jira.IssueView) {
				dm.SetIssue(reg["EPIC-1"])
			},
			"",
		},
		{
			"one level",
			func(dm *DetailModel, reg map[string]*jira.IssueView) {
				dm.SetIssue(reg["EPIC-1"])
				dm.NavigateTo(reg["STORY-1"])
			},
			"EPIC-1 → STORY-1",
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

func TestDetailCommentMode(t *testing.T) {
	dm, reg := testDetailModel()
	dm.SetIssue(reg["EPIC-1"])

	// Step 1: Start comment
	dm.StartComment()
	if dm.Mode() != DetailComment {
		t.Fatalf("Mode() = %d; want DetailComment (%d)", dm.Mode(), DetailComment)
	}

	// Step 2: Cancel
	dm.CancelInput()
	if dm.Mode() != DetailBrowse {
		t.Fatalf("Mode() = %d; want DetailBrowse (%d) after cancel", dm.Mode(), DetailBrowse)
	}

	// Step 3: Start again and get value
	dm.StartComment()
	val := dm.InputValue()
	if dm.Mode() != DetailBrowse {
		t.Errorf("Mode() = %d; want DetailBrowse (%d) after InputValue", dm.Mode(), DetailBrowse)
	}
	if val != "" {
		t.Errorf("InputValue() = %q; want empty", val)
	}
}
