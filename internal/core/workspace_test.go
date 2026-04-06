package core

import (
	"testing"
)

func TestWorkspace_TypeByName(t *testing.T) {
	ws := &Workspace{
		Types: []TypeConfig{
			{Name: "Story"},
			{Name: "Bug"},
			{Name: "Task"},
		},
	}

	t.Run("exact match", func(t *testing.T) {
		tc := ws.TypeByName("Bug")
		if tc == nil || tc.Name != "Bug" {
			t.Fatalf("TypeByName(Bug) = %v", tc)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		tc := ws.TypeByName("bug")
		if tc == nil || tc.Name != "Bug" {
			t.Fatalf("TypeByName(bug) = %v", tc)
		}
	})

	t.Run("not found", func(t *testing.T) {
		tc := ws.TypeByName("Epic")
		if tc != nil {
			t.Fatalf("TypeByName(Epic) = %v, want nil", tc)
		}
	})

	t.Run("returns pointer into slice", func(t *testing.T) {
		tc := ws.TypeByName("Story")
		tc.Color = "blue"
		if ws.Types[0].Color != "blue" {
			t.Fatal("TypeByName should return pointer into ws.Types slice")
		}
	})
}

func TestWorkspace_AllFieldDefs(t *testing.T) {
	ws := &Workspace{
		Types: []TypeConfig{
			{Name: "Story", Fields: FieldDefs{
				{Key: "priority", Label: "Priority"},
				{Key: "story_points", Label: "Story Points"},
			}},
			{Name: "Bug", Fields: FieldDefs{
				{Key: "priority", Label: "Priority"},
				{Key: "severity", Label: "Severity"},
			}},
		},
	}

	defs := ws.AllFieldDefs()

	if len(defs) != 3 {
		t.Fatalf("AllFieldDefs() len = %d, want 3", len(defs))
	}

	keys := make(map[string]bool)
	for _, d := range defs {
		keys[d.Key] = true
	}
	for _, want := range []string{"priority", "story_points", "severity"} {
		if !keys[want] {
			t.Errorf("AllFieldDefs() missing %q", want)
		}
	}
}

func TestWorkspace_AllFieldDefs_Empty(t *testing.T) {
	ws := &Workspace{}
	defs := ws.AllFieldDefs()
	if len(defs) != 0 {
		t.Fatalf("AllFieldDefs() len = %d, want 0", len(defs))
	}
}

func TestWorkspace_AllFieldDefs_FirstTypeWins(t *testing.T) {
	ws := &Workspace{
		Types: []TypeConfig{
			{Name: "Story", Fields: FieldDefs{
				{Key: "priority", Label: "Priority", Enum: []string{"High", "Low"}},
			}},
			{Name: "Bug", Fields: FieldDefs{
				{Key: "priority", Label: "Bug Priority", Enum: []string{"P1", "P2", "P3"}},
			}},
		},
	}

	defs := ws.AllFieldDefs()
	if len(defs) != 1 {
		t.Fatalf("AllFieldDefs() len = %d, want 1", len(defs))
	}
	if defs[0].Label != "Priority" {
		t.Errorf("first-type-wins: Label = %q, want Priority", defs[0].Label)
	}
	if len(defs[0].Enum) != 2 {
		t.Errorf("first-type-wins: Enum len = %d, want 2", len(defs[0].Enum))
	}
}
