package encoding

import (
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/core"
)

// workspaceEnums extracts type and status enum values from a Workspace
// as []any for use in jsonschema.Schema Enum fields.
func workspaceEnums(ws *core.Workspace) (types, statuses []any) {
	types = make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		types = append(types, t.Name)
	}
	statuses = make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statuses = append(statuses, st.Name)
	}
	return
}

// normalizeAssignee collapses user-facing "unassigned"/"none" sentinels to the
// empty string so downstream diffing treats them as clear-intent.
func normalizeAssignee(s string) string {
	switch strings.ToLower(s) {
	case "unassigned", "none":
		return ""
	}
	return s
}

// renderField prepares a field value from Fields for external
// representation. RichText is rendered to Markdown; all other types
// pass through as-is. Used by the manifest path which needs native
// types for YAML encoding (e.g. []string → YAML array).
func renderField(v any, def core.FieldDef) any {
	if def.Type == core.FieldRichText {
		return core.RenderRichText(v)
	}
	return v
}

// renderFieldAsString renders a field value from Fields as a string.
// Used by the frontmatter path (map[string]string). Applies the same
// RichText coercion as renderField, then stringifies the result
// (joining string slices with ", ").
func renderFieldAsString(v any, def core.FieldDef) string {
	switch val := renderField(v, def).(type) {
	case string:
		return val
	case []string:
		if len(val) > 0 {
			return strings.Join(val, ", ")
		}
	}
	return ""
}

// fieldDefToSchema maps a FieldDef to its JSON-Schema representation.
// Returns nil for types without a schema mapping.
func fieldDefToSchema(def core.FieldDef) *jsonschema.Schema {
	switch def.Type {
	case core.FieldString, core.FieldRichText:
		return &jsonschema.Schema{Type: "string"}
	case core.FieldEmail:
		return &jsonschema.Schema{Type: "string", Format: "email"}
	case core.FieldAssignee:
		return &jsonschema.Schema{
			AnyOf: []*jsonschema.Schema{
				{Type: "string", Enum: []any{"unassigned", "none"}},
				{Type: "string", Format: "email"},
			},
		}
	case core.FieldEnum:
		enums := make([]any, len(def.Enum))
		for i, e := range def.Enum {
			enums[i] = e
		}
		return &jsonschema.Schema{Type: "string", Enum: enums}
	case core.FieldStringArray:
		return &jsonschema.Schema{
			Type:  "array",
			Items: &jsonschema.Schema{Type: "string"},
		}
	case core.FieldBool:
		return &jsonschema.Schema{Type: "boolean"}
	}
	return nil
}
