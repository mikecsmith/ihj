// field_types.go — Jira schema → core type mapping tables and converters.
// Stateless functions and lookup tables — no Provider dependency.

package jira

import (
	"encoding/json"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
)

// knownFieldIcons maps well-known custom field aliases to icons.
var knownFieldIcons = map[string]string{
	"story_points": core.IconStoryPoints,
	"sprint":       core.IconSprint,
	"team":         core.IconTeam,
}

// nameToKey derives a snake_case key from a Jira field name.
// e.g. "Epic Link" → "epic_link", "Story Points" → "story_points".
func nameToKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r == ' ' || r == '-':
			b.WriteByte('_')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// metaFieldToDef converts a createmeta field into a core.FieldDef.
func metaFieldToDef(mf createMetaField, pinned bool) core.FieldDef {
	def := core.FieldDef{
		Key:      mf.Key,
		Label:    mf.Name,
		FieldID:  mf.FieldID,
		Required: mf.Required,
		Pinned:   pinned,
		Role:     core.RoleCustom,
		Type:     schemaToFieldType(mf.Schema),
	}

	if icon, ok := knownFieldIcons[mf.Key]; ok {
		def.Icon = icon
	}

	if len(mf.AllowedValues) > 0 {
		names, _ := extractAllowedValues(mf.AllowedValues)
		if len(names) > 0 {
			def.Type = core.FieldEnum
			def.Enum = names
		}
	}

	return def
}

// knownCustomTypes maps Jira custom field plugin types to their core
// FieldType. Only fields whose Custom value appears here (or whose Custom
// is empty — meaning a system field) are included in FieldDefs. Unknown
// plugin types are excluded by default to avoid surfacing internal or
// display-only fields. Add new entries here as needed.
var knownCustomTypes = map[string]core.FieldType{
	// --- Built-in (com.atlassian.jira.plugin.system.customfieldtypes) ---
	"com.atlassian.jira.plugin.system.customfieldtypes:textfield":        core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:textarea":         core.FieldRichText, // ADF in v3
	"com.atlassian.jira.plugin.system.customfieldtypes:float":            core.FieldString,   // number as string
	"com.atlassian.jira.plugin.system.customfieldtypes:datepicker":       core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:datetime":         core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:url":              core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:select":           core.FieldEnum,
	"com.atlassian.jira.plugin.system.customfieldtypes:radiobuttons":     core.FieldEnum,
	"com.atlassian.jira.plugin.system.customfieldtypes:multiselect":      core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:multicheckboxes":  core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:userpicker":       core.FieldAssignee,
	"com.atlassian.jira.plugin.system.customfieldtypes:multiuserpicker":  core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:grouppicker":      core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:multigrouppicker": core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:project":          core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:version":          core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:atlassian-team":   core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:labels":           core.FieldStringArray,

	// --- Greenhopper / Jira Software ---
	"com.pyxis.greenhopper.jira:gh-sprint":    core.FieldEnum, // sprint picker
	"com.pyxis.greenhopper.jira:gh-epic-link": core.FieldString,

	// --- Tempo ---
	"com.tempoplugin.tempo-accounts:accounts.customfield": core.FieldString,
}

// isKnownCustomType reports whether a createmeta field's Custom value is
// one we know how to handle. System fields (Custom == "") are always known.
func isKnownCustomType(custom string) bool {
	if custom == "" {
		return true // system field, no plugin type
	}
	_, ok := knownCustomTypes[custom]
	return ok
}

// schemaToFieldType maps a Jira field schema to a core.FieldType.
// For known custom types the mapping comes from knownCustomTypes; for
// system fields (Custom == "") the schema.Type drives the mapping.
func schemaToFieldType(s fieldSchema) core.FieldType {
	// Check custom type first — it's more specific than schema.Type.
	if ft, ok := knownCustomTypes[s.Custom]; ok {
		return ft
	}
	// System fields (no Custom value) fall through to schema.Type.
	switch s.Type {
	case "string":
		return core.FieldString
	case "number", "integer":
		return core.FieldString // numbers represented as strings in manifests
	case "array":
		return core.FieldStringArray
	case "option", "priority":
		return core.FieldEnum
	default:
		return core.FieldString
	}
}

// extractAllowedValues parses a JSON allowedValues array and returns
// parallel slices of display names and IDs.
func extractAllowedValues(raw json.RawMessage) (names []string, ids []string) {
	var values []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, nil
	}
	for _, v := range values {
		name := v.Name
		if name == "" {
			name = v.Value
		}
		if name == "" {
			continue
		}
		names = append(names, name)
		ids = append(ids, v.ID)
	}
	return names, ids
}
