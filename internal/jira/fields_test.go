package jira

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestWellKnownFields_ToFieldDefs(t *testing.T) {
	// Use a provider with scrum board and team UUID to include all action fields.
	p := &Provider{
		cfg: &Config{BoardType: "scrum", TeamUUID: "test-uuid"},
		ws:  &core.Workspace{},
	}
	p.wellKnown = p.buildWellKnownFields()
	defs := p.wellKnown.ToFieldDefs()

	// Every well-known field with a FieldType should produce a FieldDef.
	var expectedCount int
	for _, f := range p.wellKnown {
		if f.FieldType != "" {
			expectedCount++
		}
	}
	if len(defs) != expectedCount {
		t.Errorf("ToFieldDefs() returned %d defs; want %d", len(defs), expectedCount)
	}

	// Every FieldDef must have Label and Role set.
	for _, d := range defs {
		if d.Label == "" {
			t.Errorf("FieldDef %q has empty Label", d.Key)
		}
		if d.Role == "" {
			t.Errorf("FieldDef %q has empty Role", d.Key)
		}
	}

	// Action fields with Enum type must have hardcoded values (semantic commands).
	// Regular enum fields (like priority) get values from createmeta at runtime,
	// so they're empty in ToFieldDefs().
	for _, d := range defs {
		if d.Type == core.FieldEnum && d.WriteOnly && len(d.Enum) == 0 {
			t.Errorf("action field %q is FieldEnum but has no Enum values", d.Key)
		}
	}
}

func TestWellKnownFields_ActionFields(t *testing.T) {
	p := &Provider{
		cfg: &Config{BoardType: "scrum", TeamUUID: "test-uuid"},
		ws:  &core.Workspace{},
	}
	p.wellKnown = p.buildWellKnownFields()
	defs := p.wellKnown.ToFieldDefs()

	// Sprint and team should both be WriteOnly action fields.
	for _, key := range []string{"sprint", "team"} {
		d := defs.WithKey(key)
		if d == nil {
			t.Errorf("missing action field %q", key)
			continue
		}
		if !d.WriteOnly {
			t.Errorf("%q should be WriteOnly", key)
		}
		if !d.Primary {
			t.Errorf("%q should be Primary", key)
		}
	}
}

func TestWellKnownFields_ConditionalEntries(t *testing.T) {
	// Without scrum board or team UUID, sprint and team should be absent.
	p := &Provider{cfg: &Config{BoardType: "kanban"}, ws: &core.Workspace{}}
	p.wellKnown = p.buildWellKnownFields()
	defs := p.wellKnown.ToFieldDefs()

	if defs.WithKey("sprint") != nil {
		t.Error("sprint should not appear for kanban boards")
	}
	if defs.WithKey("team") != nil {
		t.Error("team should not appear without TeamUUID")
	}
}

func TestWellKnownFields_ExcludedIDs(t *testing.T) {
	wk := defaultWellKnownFields()
	excluded := wk.ExcludedIDs()

	// Core structural fields must be excluded.
	for _, key := range []string{"summary", "description", "issuetype", "status", "parent", "project"} {
		if !excluded[key] {
			t.Errorf("core field %q should be excluded", key)
		}
	}

	// Standard well-known fields with FieldDefs must also be excluded.
	for _, key := range []string{"priority", "assignee", "labels", "components", "reporter", "created", "updated"} {
		if !excluded[key] {
			t.Errorf("standard field %q should be excluded", key)
		}
	}
}

func TestWellKnownFields_SearchFields(t *testing.T) {
	wk := defaultWellKnownFields()
	fields := wk.SearchFields()
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}

	// Expected standard search fields.
	expected := []string{
		"summary", "issuetype", "status", "priority", "parent",
		"subtasks", "description", "assignee", "comment", "reporter",
		"created", "updated", "labels", "components",
	}
	for _, key := range expected {
		if !fieldSet[key] {
			t.Errorf("SearchFields() missing %q", key)
		}
	}
}

func TestWellKnownFields_KnownJSONKeys(t *testing.T) {
	wk := defaultWellKnownFields()
	keys := wk.KnownJSONKeys()

	// Must match the set of fields with struct backing on issueFields.
	expected := []string{
		"summary", "description", "issuetype", "status",
		"priority", "assignee", "reporter", "parent",
		"labels", "components", "comment", "created",
		"updated", "subtasks",
	}
	for _, key := range expected {
		if !keys[key] {
			t.Errorf("KnownJSONKeys() missing %q", key)
		}
	}
	if len(keys) != len(expected) {
		t.Errorf("KnownJSONKeys() has %d keys; want %d", len(keys), len(expected))
	}
}

func TestWellKnownFields_ApplyOverrides(t *testing.T) {
	p := &Provider{
		cfg: &Config{TeamUUID: "test-uuid"},
		ws:  &core.Workspace{},
	}
	p.wellKnown = p.buildWellKnownFields()

	// A createmeta-derived FieldDef with key "team" should get action field overrides.
	def := core.FieldDef{
		Key:  "team",
		Type: core.FieldEnum,
		Enum: []string{"Platform", "Frontend"},
	}
	p.wellKnown.ApplyOverrides(&def)

	if def.Type != core.FieldBool {
		t.Errorf("after override, type = %q; want bool", def.Type)
	}
	if !def.WriteOnly {
		t.Error("after override, should be WriteOnly")
	}
	if !def.Primary {
		t.Error("after override, should be Primary")
	}

	// A non-action field should not be overridden.
	def2 := core.FieldDef{Key: "priority", Type: core.FieldEnum}
	p.wellKnown.ApplyOverrides(&def2)
	if def2.Type != core.FieldEnum {
		t.Errorf("priority should not be overridden, got type %q", def2.Type)
	}
}

func TestTranslatedCases_MatchWellKnownFields(t *testing.T) {
	// Every explicit case in TranslateFields must correspond to a
	// well-known field with a non-empty FieldType. This catches cases
	// added to the switch without a matching registry entry.
	p := &Provider{
		cfg: &Config{BoardType: "scrum", TeamUUID: "test-uuid"},
		ws:  &core.Workspace{},
	}
	p.wellKnown = p.buildWellKnownFields()

	for _, key := range translatedCases {
		wf, ok := p.wellKnown[key]
		if !ok {
			t.Errorf("translatedCases has %q but it's not in wellKnownFields", key)
			continue
		}
		if wf.FieldType == "" {
			t.Errorf("translatedCases has %q but its FieldType is empty (system-only field)", key)
		}
	}
}
