package commands

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestCollectExtractKeys(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent", Type: "Epic", Status: "Open"}
	child1 := &core.WorkItem{ID: "C-1", Summary: "Child 1", Type: "Story", Status: "Open", ParentID: "P-1"}
	child2 := &core.WorkItem{ID: "C-2", Summary: "Child 2", Type: "Story", Status: "Open", ParentID: "P-1"}
	sibling := &core.WorkItem{ID: "S-1", Summary: "Sibling", Type: "Story", Status: "Open", ParentID: "P-1"}

	registry := map[string]*core.WorkItem{
		"P-1": parent, "C-1": child1, "C-2": child2, "S-1": sibling,
	}
	core.LinkChildren(registry)

	t.Run("target only", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeSelectedOnly, registry)
		if len(keys) != 1 || !keys["C-1"] {
			t.Errorf("keys = %v", keys)
		}
	})

	t.Run("target + children", func(t *testing.T) {
		keys := CollectExtractKeys("P-1", ScopeWithChildren, registry)
		if len(keys) != 4 { // P-1, C-1, C-2, S-1 are all children of P-1
			t.Errorf("keys = %v, want 4", keys)
		}
	})

	t.Run("full family", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeFullFamily, registry)
		// Should include C-1, P-1 (parent), C-2 and S-1 (siblings sharing parent P-1)
		if !keys["C-1"] || !keys["P-1"] || !keys["C-2"] || !keys["S-1"] {
			t.Errorf("keys = %v, missing expected entries", keys)
		}
	})

	t.Run("with parent", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeWithParent, registry)
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
		}
		if !keys["C-1"] || !keys["P-1"] {
			t.Errorf("keys = %v, expected C-1 and P-1", keys)
		}
	})

	t.Run("entire board", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeEntireBoard, registry)
		if len(keys) != len(registry) {
			t.Fatalf("expected %d keys (all registry), got %d", len(registry), len(keys))
		}
	})

	t.Run("missing target returns single key", func(t *testing.T) {
		keys := CollectExtractKeys("MISSING-99", ScopeFullFamily, registry)
		if len(keys) != 1 || !keys["MISSING-99"] {
			t.Errorf("keys = %v, expected only MISSING-99", keys)
		}
	})
}

func TestScopeOptions(t *testing.T) {
	t.Run("with parent", func(t *testing.T) {
		opts := ScopeOptions(true)
		if len(opts) != 5 {
			t.Fatalf("expected 5 scope options with parent, got %d", len(opts))
		}
		if opts[0] != ScopeSelectedOnly {
			t.Errorf("first option should be %q, got %q", ScopeSelectedOnly, opts[0])
		}
		if opts[4] != ScopeEntireBoard {
			t.Errorf("last option should be %q, got %q", ScopeEntireBoard, opts[4])
		}
	})

	t.Run("without parent", func(t *testing.T) {
		opts := ScopeOptions(false)
		if len(opts) != 3 {
			t.Fatalf("expected 3 scope options without parent, got %d", len(opts))
		}
		// Should not contain parent-related options.
		for _, o := range opts {
			if o == ScopeWithParent || o == ScopeFullFamily {
				t.Errorf("should not contain %q when hasParent=false", o)
			}
		}
	})
}

func testExtractWorkspace() *core.Workspace {
	return &core.Workspace{
		Slug:     "eng",
		Name:     "Engineering",
		Provider: "jira",
		Types: []core.TypeConfig{
			{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
			{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true},
			{ID: 11, Name: "Task", Order: 30, Color: "default"},
			{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
			{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
		},
		Statuses: []string{"To Do", "In Progress", "Done"},
	}
}

func TestBuildExtractXML(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent Epic", Type: "Epic", Status: "Open"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child Story", Type: "Story", Status: "To Do", ParentID: "P-1"}

	registry := map[string]*core.WorkItem{"P-1": parent, "C-1": child}
	core.LinkChildren(registry)

	ws := testExtractWorkspace()

	t.Run("single issue uses markdown format", func(t *testing.T) {
		keys := map[string]bool{"C-1": true}
		xml := BuildExtractXML("Describe this issue", keys, registry, ws)
		if !strings.Contains(xml, "<instruction>") {
			t.Fatal("missing <instruction> tag")
		}
		if !strings.Contains(xml, "Describe this issue") {
			t.Fatal("missing prompt text")
		}
		if !strings.Contains(xml, "Markdown") {
			t.Fatal("single issue should use Markdown output format")
		}
		if !strings.Contains(xml, `key="C-1"`) {
			t.Fatal("missing issue key in XML")
		}
	})

	t.Run("multiple issues uses schema format", func(t *testing.T) {
		keys := map[string]bool{"P-1": true, "C-1": true}
		xml := BuildExtractXML("Refine these issues", keys, registry, ws)
		if !strings.Contains(xml, "json_schema") {
			t.Fatal("multiple issues should include JSON schema")
		}
		if !strings.Contains(xml, `key="P-1"`) || !strings.Contains(xml, `key="C-1"`) {
			t.Fatal("missing issue keys in XML")
		}
	})

	t.Run("parent key attribute included", func(t *testing.T) {
		keys := map[string]bool{"C-1": true}
		xml := BuildExtractXML("test", keys, registry, ws)
		if !strings.Contains(xml, `parent="P-1"`) {
			t.Fatal("child issue should include parent attribute")
		}
	})
}
