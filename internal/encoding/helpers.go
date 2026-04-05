package encoding

import (
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
