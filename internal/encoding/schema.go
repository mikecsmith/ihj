package encoding

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/core"
)

// buildItemSchema generates the JSON Schema properties for a single work
// item. Returns the item-level properties and a typed fields bag schema.
// Prominent fields go top-level; everything else goes into the bag.
func buildItemSchema(ws *core.Workspace, defs []core.FieldDef) (itemProps map[string]*jsonschema.Schema, bagProps map[string]*jsonschema.Schema) {
	typeEnums, statusEnums := workspaceEnums(ws)

	itemProps = map[string]*jsonschema.Schema{
		core.KeyKey:     {Type: "string"},
		core.KeySummary: {Type: "string"},
		core.KeyType:    {Type: "string", Enum: typeEnums},
		core.KeyStatus:  {Type: "string", Enum: statusEnums},
		core.KeyFields:  {Type: "object"},
	}

	bagProps = map[string]*jsonschema.Schema{}
	for _, def := range defs {
		schema := fieldDefToSchema(def)
		if schema == nil {
			continue
		}

		if def.Informational() {
			if def.Prominent() {
				itemProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
			} else {
				bagProps["_"+def.Key] = &jsonschema.Schema{Type: "string"}
			}
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

	if len(bagProps) > 0 {
		itemProps[core.KeyFields] = &jsonschema.Schema{
			Type:       "object",
			Properties: bagProps,
		}
	}

	return itemProps, bagProps
}

// ManifestSchema generates the JSON Schema for bulk manifests.
// Top-level items may have parent and children; children may not have parent.
func ManifestSchema(ws *core.Workspace, defs []core.FieldDef) *jsonschema.Schema {
	// Build two independent property sets — the jsonschema library requires
	// each schema node to appear at exactly one location in the tree.
	childProps, _ := buildItemSchema(ws, defs)
	childProps[core.KeyDescription] = &jsonschema.Schema{Type: "string"}
	childProps[core.KeyChildren] = &jsonschema.Schema{
		Type:  "array",
		Items: &jsonschema.Schema{Ref: "#/$defs/item"},
	}

	topProps, _ := buildItemSchema(ws, defs)
	topProps[core.KeyDescription] = &jsonschema.Schema{Type: "string"}
	topProps[core.KeyChildren] = &jsonschema.Schema{
		Type:  "array",
		Items: &jsonschema.Schema{Ref: "#/$defs/item"},
	}
	topProps[core.KeyParent] = &jsonschema.Schema{
		Type:        "string",
		Description: "Parent issue key, or 'none' to clear",
	}

	childSchema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           childProps,
		Required:             []string{core.KeySummary, core.KeyType},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}

	topSchema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           topProps,
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
				Items: topSchema,
			},
		},
		Required: []string{"metadata", "items"},
		Defs: map[string]*jsonschema.Schema{
			"item": childSchema,
		},
	}
}

// FrontmatterSchema generates the JSON Schema for the editor frontmatter.
// Uses the same item structure as manifests (with fields bag), plus parent.
// Omits children and description (description is the markdown body).
func FrontmatterSchema(ws *core.Workspace, defs []core.FieldDef) *jsonschema.Schema {
	itemProps, _ := buildItemSchema(ws, defs)

	// Frontmatter supports parent but not children or description.
	itemProps[core.KeyParent] = &jsonschema.Schema{
		Type:        "string",
		Description: "Parent issue key, or 'none' to clear",
	}

	return &jsonschema.Schema{
		Type:                 "object",
		Properties:           itemProps,
		Required:             []string{core.KeySummary, core.KeyType},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}
