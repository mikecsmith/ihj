package core

import (
	"testing"
)

func TestWorkItem_Hashing(t *testing.T) {
	item1 := &WorkItem{
		ID:      "ENG-1",
		Type:    "Story",
		Summary: "Hash Test",
		Status:  "To Do",
		Fields:  map[string]any{"priority": "High", "sprint": 1},
	}

	// 1. Test Determinism
	hashA := item1.ContentHash()
	hashB := item1.ContentHash()
	if hashA != hashB {
		t.Errorf("ContentHash is not deterministic: %s != %s", hashA, hashB)
	}

	// 2. Test Core Field Change Detection
	item1.Summary = "Updated Hash Test"
	hashC := item1.ContentHash()
	if hashA == hashC {
		t.Error("ContentHash did not change when Summary was updated")
	}

	// 3. Test Flex Bucket Change Detection
	item1.Summary = "Hash Test" // revert
	item1.Fields["priority"] = "Low"
	hashD := item1.ContentHash()
	if hashA == hashD {
		t.Error("ContentHash did not change when Fields map was updated")
	}

	// 4. Test StateHash (Idempotency)
	state1 := item1.StateHash("PARENT-A")
	state2 := item1.StateHash("PARENT-B")
	if state1 == state2 {
		t.Error("StateHash did not change when parentID was different")
	}

	// Ensure ID does NOT affect StateHash (since ID doesn't exist during creation)
	item1.ID = "NEW-ID-2"
	state3 := item1.StateHash("PARENT-A")
	if state1 != state3 {
		t.Error("StateHash should remain identical even if ID changes")
	}
}

func TestIsZeroFieldValue(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"empty string slice", []string{}, true},
		{"non-empty string slice", []string{"a"}, false},
		{"empty any slice", []any{}, true},
		{"non-empty any slice", []any{"a"}, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"integer (non-zero)", 42, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZeroFieldValue(tt.val); got != tt.want {
				t.Errorf("IsZeroFieldValue(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestDisplayStringField(t *testing.T) {
	tests := []struct {
		name          string
		fields        map[string]any
		displayFields map[string]any
		key           string
		want          string
	}{
		{
			name:   "string field",
			fields: map[string]any{"assignee": "alice"},
			key:    "assignee",
			want:   "alice",
		},
		{
			name:          "display override",
			fields:        map[string]any{"assignee": "alice@example.com"},
			displayFields: map[string]any{"assignee": "Alice"},
			key:           "assignee",
			want:          "Alice",
		},
		{
			name:   "string slice joined",
			fields: map[string]any{"labels": []string{"security", "q1"}},
			key:    "labels",
			want:   "security, q1",
		},
		{
			name:   "empty string slice",
			fields: map[string]any{"labels": []string{}},
			key:    "labels",
			want:   "",
		},
		{
			name:   "missing field",
			fields: map[string]any{},
			key:    "labels",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WorkItem{Fields: tt.fields, DisplayFields: tt.displayFields}
			if got := w.DisplayStringField(tt.key); got != tt.want {
				t.Errorf("DisplayStringField(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
