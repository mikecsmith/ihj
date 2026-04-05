package core

import (
	"testing"
)

func defsForChanges() FieldDefs {
	return FieldDefs{
		{Key: "priority", Label: "Priority", Type: FieldEnum, Enum: []string{"High", "Low"}, Primary: true},
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee, Primary: true},
		{Key: "labels", Label: "Labels", Type: FieldStringArray, Primary: true},
		{Key: "sprint", Label: "Sprint", Type: FieldString, Primary: true, WriteOnly: true},
		{Key: "created", Label: "Created", Type: FieldString, Derived: true, Immutable: true},
	}
}

func TestComputeChanges_OmitClearSet(t *testing.T) {
	defs := defsForChanges()
	orig := &WorkItem{
		ID: "ENG-1", Type: "Story", Summary: "Original", Status: "To Do", ParentID: "ENG-0",
		Fields: map[string]any{"priority": "High", "assignee": "alice@example.com"},
	}

	t.Run("no change when set empty", func(t *testing.T) {
		edited := &WorkItem{}
		ch, err := ComputeChanges(orig, edited, SetKeys{}, defs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch != nil {
			t.Errorf("expected nil changes, got %+v", ch)
		}
	})

	t.Run("set intent emits when differs", func(t *testing.T) {
		edited := &WorkItem{Summary: "Updated", Type: "Story", Status: "To Do", ParentID: "ENG-0"}
		set := SetKeys{"summary": true, "type": true, "status": true, "parent": true}
		ch, err := ComputeChanges(orig, edited, set, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch == nil || ch.Summary == nil || *ch.Summary != "Updated" {
			t.Errorf("expected Summary=Updated, got %+v", ch)
		}
		if ch.Type != nil || ch.Status != nil || ch.ParentID != nil {
			t.Errorf("expected unchanged fields to be nil, got %+v", ch)
		}
	})

	t.Run("omit emits nothing even if value differs", func(t *testing.T) {
		edited := &WorkItem{Summary: "Updated"}
		ch, err := ComputeChanges(orig, edited, SetKeys{}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch != nil {
			t.Errorf("expected nil changes (summary not in SetKeys), got %+v", ch)
		}
	})

	t.Run("clear parent succeeds", func(t *testing.T) {
		edited := &WorkItem{ParentID: ""}
		ch, err := ComputeChanges(orig, edited, SetKeys{"parent": true}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch == nil || ch.ParentID == nil || *ch.ParentID != "" {
			t.Errorf("expected cleared ParentID, got %+v", ch)
		}
	})

	t.Run("clear field via empty value", func(t *testing.T) {
		edited := &WorkItem{Fields: map[string]any{"assignee": ""}}
		ch, err := ComputeChanges(orig, edited, SetKeys{"assignee": true}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch == nil || ch.Fields["assignee"] != "" {
			t.Errorf("expected Fields[assignee]=\"\", got %+v", ch)
		}
	})

	t.Run("clear summary is rejected", func(t *testing.T) {
		edited := &WorkItem{Summary: ""}
		_, err := ComputeChanges(orig, edited, SetKeys{"summary": true}, defs)
		if err == nil {
			t.Error("expected error clearing summary")
		}
	})

	t.Run("clear type is rejected", func(t *testing.T) {
		edited := &WorkItem{Summary: "Original", Type: ""}
		_, err := ComputeChanges(orig, edited, SetKeys{"type": true}, defs)
		if err == nil {
			t.Error("expected error clearing type")
		}
	})

	t.Run("type change is case insensitive", func(t *testing.T) {
		edited := &WorkItem{Type: "story"}
		ch, err := ComputeChanges(orig, edited, SetKeys{"type": true}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch != nil {
			t.Error("case-only type change should not register")
		}
	})
}

func TestComputeChanges_WriteOnlyEmitsOnPresence(t *testing.T) {
	defs := defsForChanges()
	orig := &WorkItem{ID: "ENG-1", Type: "Story", Fields: map[string]any{"sprint": "Sprint 3"}}

	t.Run("WriteOnly present with value emits", func(t *testing.T) {
		edited := &WorkItem{Fields: map[string]any{"sprint": "active"}}
		ch, err := ComputeChanges(orig, edited, SetKeys{"sprint": true}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch == nil || ch.Fields["sprint"] != "active" {
			t.Errorf("expected sprint=active command, got %+v", ch)
		}
	})

	t.Run("WriteOnly present but empty does not emit", func(t *testing.T) {
		edited := &WorkItem{Fields: map[string]any{"sprint": ""}}
		ch, err := ComputeChanges(orig, edited, SetKeys{"sprint": true}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch != nil {
			t.Errorf("expected nil changes for empty action, got %+v", ch)
		}
	})

	t.Run("WriteOnly omitted does not emit", func(t *testing.T) {
		edited := &WorkItem{Fields: map[string]any{"sprint": "active"}}
		ch, err := ComputeChanges(orig, edited, SetKeys{}, defs)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if ch != nil {
			t.Errorf("expected nil changes when omitted, got %+v", ch)
		}
	})
}

func TestComputeChanges_NonWritableSkipped(t *testing.T) {
	defs := defsForChanges()
	orig := &WorkItem{Type: "Story", Fields: map[string]any{"created": "2024-01-01"}}
	edited := &WorkItem{Fields: map[string]any{"created": "2025-01-01"}}
	ch, err := ComputeChanges(orig, edited, SetKeys{"created": true}, defs)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ch != nil {
		t.Errorf("expected non-diffable Derived/Immutable field to be skipped, got %+v", ch)
	}
}

func TestComputeStateHash_Stability(t *testing.T) {
	defs := defsForChanges()
	item := &WorkItem{
		Type: "Story", Summary: "Add login",
		Fields: map[string]any{"priority": "High", "assignee": "alice@example.com"},
	}
	h1 := ComputeStateHash(item, "ENG-0", defs)

	// Provider injects a Derived field on retry — hash must be unchanged.
	item.Fields["created"] = "2025-10-14T10:00:00Z"
	h2 := ComputeStateHash(item, "ENG-0", defs)
	if h1 != h2 {
		t.Errorf("Derived field injection changed StateHash:\n  before: %s\n  after:  %s", h1, h2)
	}
}

func TestComputeStateHash_Sensitivity(t *testing.T) {
	defs := defsForChanges()
	item := &WorkItem{Type: "Story", Summary: "Add login"}
	base := ComputeStateHash(item, "ENG-0", defs)

	t.Run("summary change", func(t *testing.T) {
		item.Summary = "Add SSO"
		if ComputeStateHash(item, "ENG-0", defs) == base {
			t.Error("StateHash did not change when summary was updated")
		}
	})
	t.Run("parentID change", func(t *testing.T) {
		item.Summary = "Add login"
		if ComputeStateHash(item, "ENG-9", defs) == base {
			t.Error("StateHash did not change when parentID differed")
		}
	})
	t.Run("writable field change", func(t *testing.T) {
		item.Fields = map[string]any{"priority": "Low"}
		if ComputeStateHash(item, "ENG-0", defs) == base {
			t.Error("StateHash did not change when writable field updated")
		}
	})
}
