package commands_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
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
		keys := commands.CollectExtractKeys("C-1", commands.ScopeSelectedOnly, registry)
		if len(keys) != 1 || !keys["C-1"] {
			t.Errorf("keys = %v", keys)
		}
	})

	t.Run("target + children", func(t *testing.T) {
		keys := commands.CollectExtractKeys("P-1", commands.ScopeWithChildren, registry)
		if len(keys) != 4 { // P-1, C-1, C-2, S-1 are all children of P-1
			t.Errorf("keys = %v, want 4", keys)
		}
	})

	t.Run("full family", func(t *testing.T) {
		keys := commands.CollectExtractKeys("C-1", commands.ScopeFullFamily, registry)
		// Should include C-1, P-1 (parent), C-2 and S-1 (siblings sharing parent P-1)
		if !keys["C-1"] || !keys["P-1"] || !keys["C-2"] || !keys["S-1"] {
			t.Errorf("keys = %v, missing expected entries", keys)
		}
	})

	t.Run("with parent", func(t *testing.T) {
		keys := commands.CollectExtractKeys("C-1", commands.ScopeWithParent, registry)
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
		}
		if !keys["C-1"] || !keys["P-1"] {
			t.Errorf("keys = %v, expected C-1 and P-1", keys)
		}
	})

	t.Run("entire workspace", func(t *testing.T) {
		keys := commands.CollectExtractKeys("C-1", commands.ScopeEntireWorkspace, registry)
		if len(keys) != len(registry) {
			t.Fatalf("expected %d keys (all registry), got %d", len(registry), len(keys))
		}
	})

	t.Run("missing target returns single key", func(t *testing.T) {
		keys := commands.CollectExtractKeys("MISSING-99", commands.ScopeFullFamily, registry)
		if len(keys) != 1 || !keys["MISSING-99"] {
			t.Errorf("keys = %v, expected only MISSING-99", keys)
		}
	})
}

func TestScopeOptions(t *testing.T) {
	t.Run("with parent", func(t *testing.T) {
		opts := commands.ScopeOptions(true)
		if len(opts) != 5 {
			t.Fatalf("expected 5 scope options with parent, got %d", len(opts))
		}
		if opts[0] != commands.ScopeSelectedOnly {
			t.Errorf("first option should be %q, got %q", commands.ScopeSelectedOnly, opts[0])
		}
		if opts[4] != commands.ScopeEntireWorkspace {
			t.Errorf("last option should be %q, got %q", commands.ScopeEntireWorkspace, opts[4])
		}
	})

	t.Run("without parent", func(t *testing.T) {
		opts := commands.ScopeOptions(false)
		if len(opts) != 3 {
			t.Fatalf("expected 3 scope options without parent, got %d", len(opts))
		}
		for _, o := range opts {
			if o == commands.ScopeWithParent || o == commands.ScopeFullFamily {
				t.Errorf("should not contain %q when hasParent=false", o)
			}
		}
	})
}

func TestResolveScopeName(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"selected", commands.ScopeSelectedOnly, false},
		{"children", commands.ScopeWithChildren, false},
		{"parent", commands.ScopeWithParent, false},
		{"family", commands.ScopeFullFamily, false},
		{"workspace", commands.ScopeEntireWorkspace, false},
		{"invalid", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := commands.ResolveScopeName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveScopeName(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ResolveScopeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildExtractXML(t *testing.T) {
	ws := &core.Workspace{
		Slug:     "eng",
		Name:     "Engineering",
		Provider: "test",
		Types: []core.TypeConfig{
			{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
			{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true},
			{ID: 11, Name: "Task", Order: 30, Color: "default"},
			{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
			{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
		},
		Statuses: []core.StatusConfig{
			{Name: "To Do", Order: 10, Color: "cyan"},
			{Name: "In Progress", Order: 20, Color: "blue"},
			{Name: "Done", Order: 30, Color: "green"},
		},
	}

	registry := map[string]*core.WorkItem{
		"E-1": {ID: "E-1", Summary: "Epic One", Type: "Epic", Status: "In Progress"},
		"S-1": {ID: "S-1", Summary: "Story One", Type: "Story", Status: "To Do", ParentID: "E-1"},
	}

	t.Run("includes prompt and issues", func(t *testing.T) {
		keys := map[string]bool{"E-1": true, "S-1": true}
		xml := commands.BuildExtractXML("Summarize this epic", keys, registry, ws, nil)
		if !strings.Contains(xml, "Summarize this epic") {
			t.Errorf("XML should contain the prompt")
		}
		if !strings.Contains(xml, "E-1") || !strings.Contains(xml, "S-1") {
			t.Errorf("XML should contain both issue keys")
		}
	})

	t.Run("single issue subset", func(t *testing.T) {
		keys := map[string]bool{"S-1": true}
		xml := commands.BuildExtractXML("Detail this story", keys, registry, ws, nil)
		if !strings.Contains(xml, "S-1") {
			t.Errorf("XML should contain S-1")
		}
	})

	t.Run("empty keys produces minimal output", func(t *testing.T) {
		keys := map[string]bool{}
		xml := commands.BuildExtractXML("No issues", keys, registry, ws, nil)
		if !strings.Contains(xml, "No issues") {
			t.Errorf("XML should still contain the prompt")
		}
	})

	t.Run("includes guidance section", func(t *testing.T) {
		keys := map[string]bool{"E-1": true}
		xml := commands.BuildExtractXML("Test", keys, registry, ws, nil)
		if !strings.Contains(xml, "<guidance>") {
			t.Error("XML should contain <guidance> section")
		}
		if !strings.Contains(xml, "interactive conversation") {
			t.Error("guidance should mention interactive conversation")
		}
		if !strings.Contains(xml, "supporting materials") {
			t.Error("guidance should mention supporting materials")
		}
	})

	t.Run("custom guidance overrides default", func(t *testing.T) {
		customWS := *ws
		customWS.Guidance = "Be concise.\nUse bullet points."
		keys := map[string]bool{"E-1": true}
		xml := commands.BuildExtractXML("Test", keys, registry, &customWS, nil)
		if !strings.Contains(xml, "Be concise.") {
			t.Error("XML should contain custom guidance")
		}
		if !strings.Contains(xml, "Use bullet points.") {
			t.Error("XML should contain custom guidance")
		}
		if strings.Contains(xml, "interactive conversation") {
			t.Error("XML should not contain default guidance when custom is set")
		}
	})

	t.Run("escapes XML special characters in summary", func(t *testing.T) {
		reg := map[string]*core.WorkItem{
			"X-1": {ID: "X-1", Summary: "Fix <script> & \"quotes\"", Type: "Task", Status: "To Do"},
		}
		keys := map[string]bool{"X-1": true}
		xml := commands.BuildExtractXML("Test", keys, reg, ws, nil)
		if strings.Contains(xml, "<script>") {
			t.Error("summary should have escaped <script> tag")
		}
		if !strings.Contains(xml, "&lt;script&gt;") {
			t.Error("summary should contain escaped form")
		}
		if !strings.Contains(xml, "&amp;") {
			t.Error("summary should escape ampersands")
		}
	})

	t.Run("escapes prompt text", func(t *testing.T) {
		keys := map[string]bool{"E-1": true}
		xml := commands.BuildExtractXML("Use <b>bold</b> & stuff", keys, registry, ws, nil)
		if strings.Contains(xml, "<b>bold</b>") {
			t.Error("prompt should have escaped HTML tags")
		}
		if !strings.Contains(xml, "&lt;b&gt;bold&lt;/b&gt;") {
			t.Error("prompt should contain escaped form")
		}
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		keys := map[string]bool{"E-1": true, "S-1": true}
		first := commands.BuildExtractXML("Test", keys, registry, ws, nil)
		for i := 0; i < 10; i++ {
			again := commands.BuildExtractXML("Test", keys, registry, ws, nil)
			if again != first {
				t.Fatal("BuildExtractXML output is not deterministic")
			}
		}
	})

	t.Run("includes field values", func(t *testing.T) {
		reg := map[string]*core.WorkItem{
			"F-1": {ID: "F-1", Summary: "With fields", Type: "Task", Status: "To Do",
				Fields: map[string]any{"priority": "High", "labels": []string{"frontend", "urgent"}}},
		}
		defs := []core.FieldDef{
			{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
			{Key: "labels", Label: "Labels", Type: core.FieldStringArray, Primary: true},
			{Key: "created", Label: "Created", Type: core.FieldString, Derived: true, Immutable: true},
		}
		keys := map[string]bool{"F-1": true}
		xml := commands.BuildExtractXML("Test", keys, reg, ws, defs)
		if !strings.Contains(xml, "<priority>High</priority>") {
			t.Error("should include priority field")
		}
		if !strings.Contains(xml, "<labels>frontend, urgent</labels>") {
			t.Error("should include labels field")
		}
		if strings.Contains(xml, "<created>") {
			t.Error("should not include read-only fields")
		}
	})
}
