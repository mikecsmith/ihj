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
		Statuses: []StatusConfig{{Name: "Backlog", Order: 10, Color: "default"}, {Name: "Done", Order: 20, Color: "green"}},
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
		Statuses: []StatusConfig{{Name: "To Do", Order: 10, Color: "default"}, {Name: "Done", Order: 20, Color: "green"}},
	}
	defs := []FieldDef{
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee, Primary: true},
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
		Statuses: []StatusConfig{{Name: "To Do", Order: 10, Color: "default"}},
	}
	defs := []FieldDef{
		{Key: "reporter", Label: "Reporter", Type: FieldEmail, Primary: true},
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

func TestManifestSchema_InformationalFields(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Task"}},
		Statuses: []StatusConfig{{Name: "To Do", Order: 10, Color: "default"}},
	}
	defs := []FieldDef{
		{Key: "sprint", Label: "Sprint", Type: FieldString, Primary: true, WriteOnly: true},
		{Key: "created", Label: "Created", Type: FieldString, Primary: true, Derived: true, Immutable: true},
	}

	sch := ManifestSchema(ws, defs)
	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// WriteOnly fields keep the unprefixed action key AND get the _-prefixed informational key.
	validAction := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test", "sprint": "active",
		}},
	}
	if err := resolved.Validate(validAction); err != nil {
		t.Errorf("sprint action key should be valid: %v", err)
	}

	validPrefixed := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test", "_sprint": "Sprint 5",
		}},
	}
	if err := resolved.Validate(validPrefixed); err != nil {
		t.Errorf("_sprint informational key should be valid: %v", err)
	}

	// Immutable fields only appear as _-prefixed; the bare key is not in the schema.
	validCreated := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test", "_created": "2026-03-30T19:34:19+01:00",
		}},
	}
	if err := resolved.Validate(validCreated); err != nil {
		t.Errorf("_created informational key should be valid: %v", err)
	}

	// Bare "created" must be rejected — it's immutable and not actionable.
	invalidBare := map[string]any{
		"metadata": map[string]any{"workspace": "eng"},
		"items": []any{map[string]any{
			"type": "Task", "summary": "Test", "created": "2026-03-30",
		}},
	}
	if err := resolved.Validate(invalidBare); err == nil {
		t.Error("bare 'created' key should be rejected by schema but was accepted")
	}
}

func TestEncodeManifest_AssigneeNoneExport(t *testing.T) {
	defs := []FieldDef{
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee, Primary: true},
		{Key: "priority", Label: "Priority", Type: FieldEnum, Primary: true},
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
		{Key: "assignee", Label: "Assignee", Type: FieldAssignee, Primary: true},
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

func TestEncodeManifest_InformationalFields(t *testing.T) {
	defs := []FieldDef{
		{Key: "priority", Label: "Priority", Type: FieldEnum, Primary: true},
		{Key: "sprint", Label: "Sprint", Type: FieldString, Primary: true, WriteOnly: true},
		{Key: "created", Label: "Created", Type: FieldString, Primary: true, Derived: true, Immutable: true},
		{Key: "story_points", Label: "Story Points", Type: FieldString},
	}

	m := &Manifest{
		Metadata: Metadata{Workspace: "test"},
		Items: []*WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{
					"priority":     "High",
					"sprint":       "Sprint 3",
					"created":      "2024-01-15",
					"story_points": "5",
				},
			},
		},
	}

	t.Run("default export omits informational fields", func(t *testing.T) {
		var buf bytes.Buffer
		if err := EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "sprint") {
			t.Errorf("default export should omit sprint, got:\n%s", out)
		}
		if strings.Contains(out, "created") {
			t.Errorf("default export should omit created, got:\n%s", out)
		}
		if !strings.Contains(out, "priority: High") {
			t.Errorf("default export should include priority, got:\n%s", out)
		}
	})

	t.Run("full export prefixes informational fields with underscore", func(t *testing.T) {
		var buf bytes.Buffer
		if err := EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "_sprint: Sprint 3") {
			t.Errorf("full export should contain '_sprint: Sprint 3', got:\n%s", out)
		}
		if !strings.Contains(out, "_created: \"2024-01-15\"") && !strings.Contains(out, "_created: 2024-01-15") {
			t.Errorf("full export should contain '_created: 2024-01-15', got:\n%s", out)
		}
		if !strings.Contains(out, "priority: High") {
			t.Errorf("full export should contain 'priority: High', got:\n%s", out)
		}
		// Non-primary fields go in the fields bag, also with _ prefix if informational.
		if strings.Contains(out, "_story") {
			t.Errorf("story_points is not informational and should not be prefixed, got:\n%s", out)
		}
	})

	t.Run("decode ignores underscore-prefixed keys", func(t *testing.T) {
		input := `
metadata:
  workspace: test
items:
  - key: ENG-1
    type: Task
    summary: Test
    status: To Do
    priority: High
    _sprint: Sprint 3
    _created: "2024-01-15"
`
		decoded, err := DecodeManifest([]byte(input), defs)
		if err != nil {
			t.Fatalf("DecodeManifest: %v", err)
		}
		item := decoded.Items[0]
		if _, ok := item.Fields["sprint"]; ok {
			t.Errorf("_sprint should be ignored on decode, but sprint is in Fields")
		}
		if _, ok := item.Fields["_sprint"]; ok {
			t.Errorf("_sprint should be ignored on decode, but _sprint is in Fields")
		}
		if _, ok := item.Fields["created"]; ok {
			t.Errorf("_created should be ignored on decode, but created is in Fields")
		}
		if item.Fields["priority"] != "High" {
			t.Errorf("expected priority=High, got %v", item.Fields["priority"])
		}
	})
}

func TestEncodeManifest_SequenceIndentation(t *testing.T) {
	defs := []FieldDef{
		{Key: "labels", Label: "Labels", Type: FieldStringArray, Primary: true},
	}

	m := &Manifest{
		Metadata: Metadata{Workspace: "test"},
		Items: []*WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{
					"labels": []string{"frontend", "auth"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	out := buf.String()
	// Verify sequence items are indented under their key, not at the same level.
	// Bad:  "labels:\n- frontend"  (same indent)
	// Good: "labels:\n  - frontend" (deeper indent)
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "labels:" && i+1 < len(lines) {
			labelIndent := len(line) - len(strings.TrimLeft(line, " "))
			itemLine := lines[i+1]
			itemIndent := len(itemLine) - len(strings.TrimLeft(itemLine, " "))
			if itemIndent <= labelIndent {
				t.Errorf("sequence items should be indented deeper than key, got:\n%s\n%s", line, itemLine)
			}
			break
		}
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
