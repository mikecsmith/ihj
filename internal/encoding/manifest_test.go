package encoding_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/encoding"
)

func TestManifestSchema_Validation(t *testing.T) {
	ws := &core.Workspace{
		Types:    []core.TypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Statuses: []core.StatusConfig{{Name: "Backlog", Order: 10, Color: "default"}, {Name: "Done", Order: 20, Color: "green"}},
	}

	sch := encoding.ManifestSchema(ws, nil)

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

func TestManifestSchema_FieldAssignee(t *testing.T) {
	ws := &core.Workspace{
		Types:    []core.TypeConfig{{Name: "Task"}},
		Statuses: []core.StatusConfig{{Name: "To Do", Order: 10, Color: "default"}, {Name: "Done", Order: 20, Color: "green"}},
	}
	defs := []core.FieldDef{
		{Key: "assignee", Label: "Assignee", Type: core.FieldAssignee, Primary: true},
	}

	sch := encoding.ManifestSchema(ws, defs)
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
	ws := &core.Workspace{
		Types:    []core.TypeConfig{{Name: "Task"}},
		Statuses: []core.StatusConfig{{Name: "To Do", Order: 10, Color: "default"}},
	}
	defs := []core.FieldDef{
		{Key: "reporter", Label: "Reporter", Type: core.FieldEmail, Primary: true},
	}

	sch := encoding.ManifestSchema(ws, defs)
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
	ws := &core.Workspace{
		Types:    []core.TypeConfig{{Name: "Task"}},
		Statuses: []core.StatusConfig{{Name: "To Do", Order: 10, Color: "default"}},
	}
	defs := []core.FieldDef{
		{Key: "sprint", Label: "Sprint", Type: core.FieldString, Primary: true, WriteOnly: true},
		{Key: "created", Label: "Created", Type: core.FieldString, Primary: true, Derived: true, Immutable: true},
	}

	sch := encoding.ManifestSchema(ws, defs)
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
	defs := []core.FieldDef{
		{Key: "assignee", Label: "Assignee", Type: core.FieldAssignee, Primary: true},
		{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
	}

	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
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
		if err := encoding.EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		yaml := buf.String()
		if !strings.Contains(yaml, "assignee: none") {
			t.Errorf("expected 'assignee: none' in full export, got:\n%s", yaml)
		}
	})

	t.Run("default export omits empty assignee", func(t *testing.T) {
		var buf bytes.Buffer
		if err := encoding.EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
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
		if err := encoding.EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}
		yaml := buf.String()
		if !strings.Contains(yaml, "assignee: alice@example.com") {
			t.Errorf("expected 'assignee: alice@example.com', got:\n%s", yaml)
		}
	})
}

func TestDecodeManifest_AssigneeRoundtrip(t *testing.T) {
	defs := []core.FieldDef{
		{Key: "assignee", Label: "Assignee", Type: core.FieldAssignee, Primary: true},
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
	m, err := encoding.DecodeManifest([]byte(input), defs)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}

	if len(m.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(m.Items))
	}

	// "none" / "unassigned" sentinels are normalised to "" at decode time
	// so downstream diff logic treats them as clear-intent uniformly.
	assignee := m.Items[0].Fields["assignee"]
	if assignee != "" {
		t.Errorf("expected assignee to be normalised to empty, got %v", assignee)
	}
}

func TestEncodeManifest_InformationalFields(t *testing.T) {
	defs := []core.FieldDef{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum, Primary: true},
		{Key: "sprint", Label: "Sprint", Type: core.FieldString, Primary: true, WriteOnly: true},
		{Key: "created", Label: "Created", Type: core.FieldString, Primary: true, Derived: true, Immutable: true},
		{Key: "story_points", Label: "Story Points", Type: core.FieldString},
	}

	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
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
		if err := encoding.EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
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
		if err := encoding.EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
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
		decoded, err := encoding.DecodeManifest([]byte(input), defs)
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
	defs := []core.FieldDef{
		{Key: "labels", Label: "Labels", Type: core.FieldStringArray, Primary: true},
	}

	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{
					"labels": []string{"frontend", "auth"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := encoding.EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
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

func TestManifest_RichTextRoundtrip(t *testing.T) {
	defs := []core.FieldDef{
		{Key: "acceptance", Label: "Acceptance Criteria", Type: core.FieldRichText, Primary: true},
	}

	source := "## Goals\n\n- First item\n- Second item\n\n**Important** note."
	node, err := document.ParseMarkdownString(source)
	if err != nil {
		t.Fatalf("ParseMarkdownString: %v", err)
	}

	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{"acceptance": node},
			},
		},
	}

	var buf bytes.Buffer
	if err := encoding.EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	yamlOut := buf.String()

	if !strings.Contains(yamlOut, "## Goals") || !strings.Contains(yamlOut, "First item") {
		t.Errorf("expected markdown content in YAML output:\n%s", yamlOut)
	}

	decoded, err := encoding.DecodeManifest(buf.Bytes(), defs)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if len(decoded.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(decoded.Items))
	}

	gotNode, ok := decoded.Items[0].Fields["acceptance"].(*document.Node)
	if !ok || gotNode == nil {
		t.Fatalf("expected *document.Node for acceptance, got %T", decoded.Items[0].Fields["acceptance"])
	}

	reRendered := strings.TrimSpace(document.RenderMarkdown(gotNode))
	origRendered := strings.TrimSpace(document.RenderMarkdown(node))
	if reRendered != origRendered {
		t.Errorf("roundtrip mismatch:\n  orig:  %q\n  round: %q", origRendered, reRendered)
	}
}

func TestManifest_RichTextInBag(t *testing.T) {
	defs := []core.FieldDef{
		{Key: "notes", Label: "Notes", Type: core.FieldRichText}, // not Primary → bag
	}

	node, _ := document.ParseMarkdownString("Some **bold** text.")
	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{"notes": node},
			},
		},
	}

	var buf bytes.Buffer
	if err := encoding.EncodeManifest(&buf, m, defs, true, "yaml"); err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	if !strings.Contains(buf.String(), "**bold**") {
		t.Errorf("expected rendered markdown in bag:\n%s", buf.String())
	}

	decoded, err := encoding.DecodeManifest(buf.Bytes(), defs)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if _, ok := decoded.Items[0].Fields["notes"].(*document.Node); !ok {
		t.Errorf("expected bag-decoded RichText as *document.Node, got %T", decoded.Items[0].Fields["notes"])
	}
}

func TestManifest_RichTextEmptyOmitted(t *testing.T) {
	defs := []core.FieldDef{
		{Key: "acceptance", Label: "Acceptance", Type: core.FieldRichText, Primary: true},
	}
	m := &encoding.Manifest{
		Metadata: encoding.Metadata{Workspace: "test"},
		Items: []*core.WorkItem{
			{
				ID: "ENG-1", Type: "Task", Summary: "Test", Status: "To Do",
				Fields: map[string]any{"acceptance": (*document.Node)(nil)},
			},
		},
	}
	var buf bytes.Buffer
	if err := encoding.EncodeManifest(&buf, m, defs, false, "yaml"); err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	if strings.Contains(buf.String(), "acceptance") {
		t.Errorf("expected empty RichText to be omitted in default export:\n%s", buf.String())
	}
}
