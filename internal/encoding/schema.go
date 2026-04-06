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

	// Add all provider-known fields. Prominent fields go top-level;
	// everything else goes into the "fields" bag schema. Informational
	// fields (WriteOnly actions, Immutable read-only) get a "_"-prefixed
	// key accepted for full-export round-trip validation.
	bagProps := map[string]*jsonschema.Schema{}
	for _, def := range defs {
		schema := fieldDefToSchema(def)
		if schema == nil {
			continue
		}

		if def.Informational() {
			// Informational fields are accepted with "_" prefix (ignored on import).
			if def.Prominent() {
				itemProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
			} else {
				bagProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
			}
			// WriteOnly action fields also keep the unprefixed actionable key.
			if !def.WriteOnly {
				continue
			}
		}

		if def.Prominent() {
			itemProps[def.Key] = schema
		} else {
			bagProps[def.Key] = schema
		}
	}

	// Replace the untyped "fields" placeholder with a typed bag schema.
	if len(bagProps) > 0 {
		itemProps[core.KeyFields] = &jsonschema.Schema{
			Type:       "object",
			Properties: bagProps,
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

	// Add all schema-eligible fields — users may want to set fields they
	// haven't explicitly opted into via workspace config.
	for _, def := range defs {
		if !def.IncludeInSchema() {
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
