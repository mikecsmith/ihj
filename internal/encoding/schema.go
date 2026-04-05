package encoding

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/core"
)

// ManifestSchema generates the JSON Schema for bulk manifests.
// Field defs drive the item properties: top-level defs become item-level
// schema properties with appropriate types and enums.
func ManifestSchema(ws *core.Workspace, defs []core.FieldDef) *jsonschema.Schema {
	typeEnums := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeEnums = append(typeEnums, t.Name)
	}

	statusEnums := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusEnums = append(statusEnums, st.Name)
	}

	itemProps := map[string]*jsonschema.Schema{
		"key":         {Type: "string"},
		"summary":     {Type: "string"},
		"type":        {Type: "string", Enum: typeEnums},
		"status":      {Type: "string", Enum: statusEnums},
		"description": {Type: "string"},
		"fields":      {Type: "object"},
		"children": {
			Type:  "array",
			Items: &jsonschema.Schema{Ref: "#/$defs/item"},
		},
	}

	// Add field-def-driven properties for top-level fields.
	for _, def := range defs {
		if !def.Prominent() {
			continue
		}

		// Informational fields that aren't schema-eligible only appear as
		// "_"-prefixed read-only keys in full exports (e.g. _created).
		if !def.IncludeInSchema() {
			if def.Informational() {
				itemProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
			}
			continue
		}

		var schema *jsonschema.Schema
		switch def.Type {
		case core.FieldString:
			schema = &jsonschema.Schema{Type: "string"}
		case core.FieldEmail:
			schema = &jsonschema.Schema{Type: "string", Format: "email"}
		case core.FieldAssignee:
			schema = &jsonschema.Schema{
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
			schema = &jsonschema.Schema{Type: "string", Enum: enums}
		case core.FieldStringArray:
			schema = &jsonschema.Schema{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			}
		case core.FieldBool:
			schema = &jsonschema.Schema{Type: "boolean"}
		default:
			continue
		}

		itemProps[def.Key] = schema

		// Informational fields also get a "_"-prefixed read-only key for full exports
		// (e.g. sprint → _sprint). The unprefixed key remains for the action value.
		if def.Informational() {
			itemProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
		}
	}

	issueSchema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           itemProps,
		Required:             []string{"summary", "type"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}

	metadataSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"workspace":   {Type: "string"},
			"exported_at": {Type: "string"},
			"context":     {Type: "object"},
		},
		Required:             []string{"workspace"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}

	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"metadata": metadataSchema,
			"items": {
				Type:  "array",
				Items: &jsonschema.Schema{Ref: "#/$defs/item"},
			},
		},
		Required: []string{"metadata", "items"},
		Defs: map[string]*jsonschema.Schema{
			"item": issueSchema,
		},
	}
}

// FrontmatterSchema generates the JSON Schema for the editor's YAML frontmatter.
// Field defs drive provider-specific properties (e.g., sprint for scrum boards).
func FrontmatterSchema(ws *core.Workspace, defs []core.FieldDef) *jsonschema.Schema {
	typeNames := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeNames = append(typeNames, t.Name)
	}

	statusNames := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusNames = append(statusNames, st.Name)
	}

	properties := map[string]*jsonschema.Schema{
		"key":     {Type: "string", Description: "Existing issue key (e.g., ENG-123, 51). Omit if creating new."},
		"summary": {Type: "string"},
		"type":    {Type: "string", Enum: typeNames},
		"status":  {Type: "string", Enum: statusNames},
		"parent":  {Type: "string"},
	}

	// Add field-def-driven properties: prominent (Primary/Required/Pinned) fields.
	for _, def := range defs {
		if !def.IncludeInSchema() || !def.Prominent() {
			continue
		}
		switch def.Type {
		case core.FieldEnum:
			enums := make([]any, len(def.Enum))
			for i, e := range def.Enum {
				enums[i] = e
			}
			properties[def.Key] = &jsonschema.Schema{Type: "string", Enum: enums}
		case core.FieldString:
			properties[def.Key] = &jsonschema.Schema{Type: "string"}
		case core.FieldStringArray:
			properties[def.Key] = &jsonschema.Schema{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			}
		case core.FieldBool:
			properties[def.Key] = &jsonschema.Schema{Type: "boolean"}
		case core.FieldAssignee, core.FieldEmail:
			properties[def.Key] = &jsonschema.Schema{Type: "string"}
		}
	}

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"summary", "type"},
	}
}
