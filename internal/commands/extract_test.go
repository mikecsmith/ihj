package commands_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// ── Key collection ──────────────────────────────────────────────

func TestCollectExtractKeys(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent", Type: "Epic", Status: "Open"}
	child1 := &core.WorkItem{ID: "C-1", Summary: "Child 1", Type: "Story", Status: "Open", ParentID: "P-1"}
	child2 := &core.WorkItem{ID: "C-2", Summary: "Child 2", Type: "Story", Status: "Open", ParentID: "P-1"}
	sibling := &core.WorkItem{ID: "S-1", Summary: "Sibling", Type: "Story", Status: "Open", ParentID: "P-1"}

	registry := map[string]*core.WorkItem{
		"P-1": parent, "C-1": child1, "C-2": child2, "S-1": sibling,
	}
	core.LinkChildren(registry)

	tests := []struct {
		name     string
		issueKey string
		scope    string
		wantKeys []string
	}{
		{"selected only", "C-1", commands.ScopeSelectedOnly, []string{"C-1"}},
		{"with children", "P-1", commands.ScopeWithChildren, []string{"P-1", "C-1", "C-2", "S-1"}},
		{"with parent", "C-1", commands.ScopeWithParent, []string{"C-1", "P-1"}},
		{"full family", "C-1", commands.ScopeFullFamily, []string{"C-1", "P-1", "C-2", "S-1"}},
		{"entire workspace", "C-1", commands.ScopeEntireWorkspace, []string{"P-1", "C-1", "C-2", "S-1"}},
		{"entire workspace without issue key", "", commands.ScopeEntireWorkspace, []string{"P-1", "C-1", "C-2", "S-1"}},
		{"missing target returns single key", "MISSING-99", commands.ScopeFullFamily, []string{"MISSING-99"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collected := commands.CollectExtractKeys(test.issueKey, test.scope, registry)
			if len(collected) != len(test.wantKeys) {
				t.Fatalf("got %d keys %v, want %d keys %v", len(collected), collected, len(test.wantKeys), test.wantKeys)
			}
			for _, wantKey := range test.wantKeys {
				if !collected[wantKey] {
					t.Errorf("missing expected key %q in %v", wantKey, collected)
				}
			}
		})
	}
}

// ── Scope resolution ────────────────────────────────────────────

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
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := commands.ResolveScopeName(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("ResolveScopeName(%q) error = %v, wantErr = %v", test.input, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("ResolveScopeName(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

// ── Preset resolution ───────────────────────────────────────────

func TestResolvePreset(t *testing.T) {
	workspace := &core.Workspace{}

	tests := []struct {
		name         string
		presetName   string
		wantGuidance string
		wantFormat   bool
		wantErr      bool
	}{
		{"refine", "refine", commands.DefaultRefineGuidance, true, false},
		{"triage", "triage", commands.DefaultTriageGuidance, true, false},
		{"bare", "bare", "", false, false},
		{"invalid", "unknown", "", false, true},
		{"empty", "", "", false, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preset, err := commands.ResolvePreset(test.presetName, workspace)
			if (err != nil) != test.wantErr {
				t.Fatalf("ResolvePreset(%q) error = %v, wantErr = %v", test.presetName, err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if preset.Guidance != test.wantGuidance {
				t.Errorf("guidance mismatch for %q", test.presetName)
			}
			if preset.IncludeFormat != test.wantFormat {
				t.Errorf("IncludeFormat = %v, want %v", preset.IncludeFormat, test.wantFormat)
			}
		})
	}

	t.Run("workspace guidance overrides refine preset", func(t *testing.T) {
		customWS := &core.Workspace{
			ExtractGuidance: map[string]string{"refine": "Custom team guidance."},
		}
		preset, err := commands.ResolvePreset("refine", customWS)
		if err != nil {
			t.Fatal(err)
		}
		if preset.Guidance != "Custom team guidance." {
			t.Errorf("guidance = %q, want workspace override", preset.Guidance)
		}
	})

	t.Run("workspace guidance does not override bare preset", func(t *testing.T) {
		customWS := &core.Workspace{
			ExtractGuidance: map[string]string{"bare": "Should not apply"},
		}
		preset, err := commands.ResolvePreset("bare", customWS)
		if err != nil {
			t.Fatal(err)
		}
		if preset.Guidance != "" {
			t.Errorf("bare guidance = %q, want empty", preset.Guidance)
		}
	})
}

func TestScopeOptions(t *testing.T) {
	tests := []struct {
		name       string
		hasParent  bool
		wantCount  int
		wantFirst  string
		wantLast   string
		wantAbsent []string
	}{
		{
			name:      "with parent includes all scopes",
			hasParent: true,
			wantCount: 5,
			wantFirst: commands.ScopeSelectedOnly,
			wantLast:  commands.ScopeEntireWorkspace,
		},
		{
			name:       "without parent excludes parent scopes",
			hasParent:  false,
			wantCount:  3,
			wantFirst:  commands.ScopeSelectedOnly,
			wantLast:   commands.ScopeEntireWorkspace,
			wantAbsent: []string{commands.ScopeWithParent, commands.ScopeFullFamily},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := commands.ScopeOptions(test.hasParent)
			if len(options) != test.wantCount {
				t.Fatalf("got %d options, want %d", len(options), test.wantCount)
			}
			if options[0] != test.wantFirst {
				t.Errorf("first option = %q, want %q", options[0], test.wantFirst)
			}
			if options[len(options)-1] != test.wantLast {
				t.Errorf("last option = %q, want %q", options[len(options)-1], test.wantLast)
			}
			for _, absent := range test.wantAbsent {
				for _, option := range options {
					if option == absent {
						t.Errorf("should not contain %q", absent)
					}
				}
			}
		})
	}
}

// ── XML generation ──────────────────────────────────────────────

func TestBuildExtractXML(t *testing.T) {
	workspace := &core.Workspace{
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

	defaultRegistry := map[string]*core.WorkItem{
		"E-1": {ID: "E-1", Summary: "Epic One", Type: "Epic", Status: "In Progress"},
		"S-1": {ID: "S-1", Summary: "Story One", Type: "Story", Status: "To Do", ParentID: "E-1"},
	}

	refinePreset := commands.ExtractPreset{
		Name:             "refine",
		Guidance:         commands.DefaultRefineGuidance,
		IncludeFormat:    true,
		IncludeTemplates: true,
	}
	barePreset := commands.ExtractPreset{Name: "bare"}

	tests := []struct {
		name         string
		prompt       string
		preset       commands.ExtractPreset
		issueKeys    map[string]bool
		registry     map[string]*core.WorkItem
		fieldDefs    []core.FieldDef
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:      "includes prompt and issue keys",
			prompt:    "Summarize this epic",
			issueKeys: map[string]bool{"E-1": true, "S-1": true},
			wantContains: []string{
				"Summarize this epic",
				"E-1",
				"S-1",
			},
		},
		{
			name:         "single issue subset",
			prompt:       "Detail this story",
			issueKeys:    map[string]bool{"S-1": true},
			wantContains: []string{"S-1"},
		},
		{
			name:         "empty keys produces minimal output",
			prompt:       "No issues",
			issueKeys:    map[string]bool{},
			wantContains: []string{"No issues"},
		},
		{
			name:      "refine preset includes guidance",
			prompt:    "Test",
			preset:    refinePreset,
			issueKeys: map[string]bool{"E-1": true},
			wantContains: []string{
				"<guidance>",
				"interactive conversation",
				"supporting materials",
			},
		},
		{
			name:   "triage preset includes triage guidance",
			prompt: "Test",
			preset: commands.ExtractPreset{
				Name:     "triage",
				Guidance: commands.DefaultTriageGuidance,
			},
			issueKeys: map[string]bool{"E-1": true},
			wantContains: []string{
				"<guidance>",
				"assess completeness",
			},
			wantAbsent: []string{
				"interactive conversation",
			},
		},
		{
			name:      "bare preset omits guidance and format",
			prompt:    "Test",
			preset:    barePreset,
			issueKeys: map[string]bool{"E-1": true},
			wantContains: []string{
				"<instruction>",
				"<issues>",
			},
			wantAbsent: []string{
				"<guidance>",
				"<output_format>",
				"<templates>",
			},
		},
		{
			name:   "custom preset guidance overrides default",
			prompt: "Test",
			preset: commands.ExtractPreset{
				Name:     "refine",
				Guidance: "Be concise.\nUse bullet points.",
			},
			issueKeys: map[string]bool{"E-1": true},
			wantContains: []string{
				"Be concise.",
				"Use bullet points.",
			},
			wantAbsent: []string{
				"interactive conversation",
			},
		},
		{
			name:   "escapes XML special characters in summary",
			prompt: "Test",
			registry: map[string]*core.WorkItem{
				"X-1": {ID: "X-1", Summary: "Fix <script> & \"quotes\"", Type: "Task", Status: "To Do"},
			},
			issueKeys: map[string]bool{"X-1": true},
			wantContains: []string{
				"&lt;script&gt;",
				"&amp;",
			},
			wantAbsent: []string{
				"<script>",
			},
		},
		{
			name:      "escapes XML special characters in prompt",
			prompt:    "Use <b>bold</b> & stuff",
			issueKeys: map[string]bool{"E-1": true},
			wantContains: []string{
				"&lt;b&gt;bold&lt;/b&gt;",
			},
			wantAbsent: []string{
				"<b>bold</b>",
			},
		},
		{
			name:   "includes diffable field values",
			prompt: "Test",
			registry: map[string]*core.WorkItem{
				"F-1": {ID: "F-1", Summary: "With fields", Type: "Task", Status: "To Do",
					Fields: map[string]any{"priority": "High", "labels": []string{"frontend", "urgent"}}},
			},
			fieldDefs: []core.FieldDef{
				{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
				{Key: "labels", Label: "Labels", Type: core.FieldStringArray, Primary: true},
				{Key: "created", Label: "Created", Type: core.FieldString, Derived: true, Immutable: true},
			},
			issueKeys: map[string]bool{"F-1": true},
			wantContains: []string{
				"<priority>High</priority>",
				"<labels>frontend, urgent</labels>",
			},
			wantAbsent: []string{
				"<created>",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := test.registry
			if registry == nil {
				registry = defaultRegistry
			}

			// Default to refine preset when not specified.
			preset := test.preset
			if preset.Name == "" {
				preset = refinePreset
			}

			output := commands.BuildExtractXML(test.prompt, preset, test.issueKeys, registry, workspace, test.fieldDefs)

			for _, want := range test.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q", want)
				}
			}
			for _, absent := range test.wantAbsent {
				if strings.Contains(output, absent) {
					t.Errorf("output should not contain %q", absent)
				}
			}
		})
	}

	t.Run("deterministic ordering", func(t *testing.T) {
		issueKeys := map[string]bool{"E-1": true, "S-1": true}
		first := commands.BuildExtractXML("Test", refinePreset, issueKeys, defaultRegistry, workspace, nil)
		for range 10 {
			again := commands.BuildExtractXML("Test", refinePreset, issueKeys, defaultRegistry, workspace, nil)
			if again != first {
				t.Fatal("BuildExtractXML output is not deterministic")
			}
		}
	})
}
