package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestSchema_Validation(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Statuses: []string{"Backlog", "Done"},
	}

	sch := ManifestSchema(ws, nil)

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// TEST: Valid Nested JSON payload (using the new Manifest structure)
	validJSON := `{
		"metadata": {
			"workspace": "eng"
		},
		"items": [
			{
				"type": "Epic",
				"summary": "Main Epic",
				"status": "Backlog",
				"children": [
					{
						"type": "Story",
						"summary": "Child Story",
						"status": "Backlog"
					}
				]
			}
		]
	}`

	var inst any
	if err := json.Unmarshal([]byte(validJSON), &inst); err != nil {
		t.Fatalf("Failed to unmarshal valid JSON setup: %v", err)
	}

	if err := resolved.Validate(inst); err != nil {
		t.Errorf("Expected valid JSON to pass, got error: %v", err)
	}
}

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

func TestManifestSchema_FieldAssignee(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Task"}},
		Statuses: []string{"To Do", "Done"},
	}
	defs := []FieldDef{
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee, TopLevel: true},
	}

	sch := ManifestSchema(ws, defs)
	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// Valid: email
	valid := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test",
			"assignee": "alice@example.com",
		}},
	}
	if err := resolved.Validate(valid); err != nil {
		t.Errorf("email assignee should be valid: %v", err)
	}

	// Valid: "unassigned" sentinel
	valid["items"] = []any{map[string]any{
		"type": "Task", "summary": "Test",
		"assignee": "unassigned",
	}}
	if err := resolved.Validate(valid); err != nil {
		t.Errorf("'unassigned' sentinel should be valid: %v", err)
	}

	// Valid: "none" sentinel
	valid["items"] = []any{map[string]any{
		"type": "Task", "summary": "Test",
		"assignee": "none",
	}}
	if err := resolved.Validate(valid); err != nil {
		t.Errorf("'none' sentinel should be valid: %v", err)
	}
}

func TestManifestSchema_FieldEmail(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Task"}},
		Statuses: []string{"To Do"},
	}
	defs := []FieldDef{
		{Key: "reporter", Label: "Reporter", Type: FieldEmail, TopLevel: true},
	}

	sch := ManifestSchema(ws, defs)
	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// Valid: email format
	valid := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test",
			"reporter": "alice@example.com",
		}},
	}
	if err := resolved.Validate(valid); err != nil {
		t.Errorf("email reporter should be valid: %v", err)
	}

	// Valid: omitted entirely
	validOmitted := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test",
		}},
	}
	if err := resolved.Validate(validOmitted); err != nil {
		t.Errorf("omitted reporter should be valid: %v", err)
	}
}

func TestEncodeManifest_AssigneeNoneExport(t *testing.T) {
	defs := []FieldDef{
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee,
			Visibility: FieldDefault, TopLevel: true},
		{Key: "priority", Label: "Priority", Type: FieldEnum,
			Visibility: FieldDefault, TopLevel: true},
	}

	m := &Manifest{
		Metadata: Metadata{Workspace: "test"},
		Items: []*WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test",
				Status: "To Do",
				Fields: map[string]any{
					"assignee": "", // empty = unassigned
					"priority": "High",
				},
			},
		},
	}

	t.Run("full export writes assignee as none", func(t *testing.T) {
		var buf bytes.Buffer
		if err := EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		yaml := buf.String()
		if !strings.Contains(yaml, "assignee: none") {
			t.Errorf("expected 'assignee: none' in full export, got:\n%s", yaml)
		}
	})

	t.Run("default export omits empty assignee", func(t *testing.T) {
		var buf bytes.Buffer
		if err := EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		yaml := buf.String()
		if strings.Contains(yaml, "assignee") {
			t.Errorf("expected no assignee in default export, got:\n%s", yaml)
		}
	})

	t.Run("assigned user exports email normally", func(t *testing.T) {
		m.Items[0].Fields["assignee"] = "alice@example.com"
		var buf bytes.Buffer
		if err := EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		yaml := buf.String()
		if !strings.Contains(yaml, "assignee: alice@example.com") {
			t.Errorf("expected 'assignee: alice@example.com', got:\n%s", yaml)
		}
	})
}

func TestDecodeManifest_AssigneeRoundtrip(t *testing.T) {
	defs := []FieldDef{
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee,
			Visibility: FieldDefault, TopLevel: true},
	}

	input := `
metadata:
  workspace: test
items:
  - key: ENG-1
    type: Task
    summary: Test
    status: To Do
    assignee: none
`
	m, err := DecodeManifest([]byte(input), defs)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}

	if len(m.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(m.Items))
	}

	// "none" comes through as the literal string — normalisation
	// happens in ComputeDiff, not at decode time.
	assignee := m.Items[0].Fields["assignee"]
	if assignee != "none" {
		t.Errorf("expected assignee to be 'none' after decode, got %v", assignee)
	}
}
