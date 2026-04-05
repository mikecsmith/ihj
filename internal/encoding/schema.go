package encoding

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/core"
)

// ManifestSchema generates the JSON Schema for bulk manifests.
// Field defs drive the item properties: top-level defs become item-level
// schema properties with appropriate types and enums.
func ManifestSchema(ws *core.Workspace, defs []core.FieldDef) *jsonschema.Schema {
	typeEnums, statusEnums := workspaceEnums(ws)

	itemProps := map[string]*jsonschema.Schema{
		core.KeyKey:         {Type: "string"},
		core.KeySummary:     {Type: "string"},
		core.KeyType:        {Type: "string", Enum: typeEnums},
		core.KeyStatus:      {Type: "string", Enum: statusEnums},
		core.KeyDescription: {Type: "string"},
		core.KeyFields:      {Type: "object"},
		core.KeyChildren: {
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

		schema := fieldDefToSchema(def)
		if schema == nil {
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
		Required:             []string{core.KeySummary, core.KeyType},
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
	typeNames, statusNames := workspaceEnums(ws)

	properties := map[string]*jsonschema.Schema{
		core.KeyKey:     {Type: "string", Description: "Existing issue key (e.g., ENG-123, 51). Omit if creating new."},
		core.KeySummary: {Type: "string"},
		core.KeyType:    {Type: "string", Enum: typeNames},
		core.KeyStatus:  {Type: "string", Enum: statusNames},
		core.KeyParent:  {Type: "string"},
	}

	// Add field-def-driven properties for user-authored top-level fields.
	for _, def := range defs {
		if !def.Authored() {
			continue
		}
		if schema := fieldDefToSchema(def); schema != nil {
			properties[def.Key] = schema
		}
	}

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{core.KeySummary, core.KeyType},
	}
}
