// Package core defines the universal domain model for work orchestration.
//
// It abstracts backend-specific concepts (like Jira issues, Trello cards,
// or GitHub issues) into a standardized WorkItem structure. This allows
// the core application to validate, diff, and manipulate hierarchies of
// tasks without needing to understand the underlying tracking provider.
package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/document"
)

// WorkItem represents a universal unit of work (Issue, Card, Task, etc.)
type WorkItem struct {
	ID       string `json:"id" yaml:"-"`
	Type     string `json:"type" yaml:"-"`
	Summary  string `json:"summary" yaml:"-"`
	Status   string `json:"status" yaml:"-"`
	ParentID string `json:"parentId" yaml:"-"`

	// Description is the AST representation — the interchange format.
	// Manifest serialization uses EncodeManifest/DecodeManifest, not json tags.
	Description *document.Node `json:"-" yaml:"-"`

	// Comments on this work item.
	Comments []Comment `json:"-" yaml:"-"`

	// Fields holds arbitrary backend-specific data (Priority, Sprint, Team, etc.)
	Fields map[string]any `json:"fields" yaml:"-"`

	// DisplayFields holds display-only values (e.g., user display names)
	// that should appear in the UI but never in exports or diffs.
	DisplayFields map[string]any `json:"displayFields" yaml:"-"`

	Children []*WorkItem `json:"children" yaml:"-"`
}

// Field accessors for common Fields entries.

func (w *WorkItem) StringField(key string) string {
	if v, ok := w.Fields[key].(string); ok {
		return v
	}
	return ""
}

// DisplayStringField returns the display-friendly value for a field.
// It checks DisplayFields first, then falls back to Fields.
func (w *WorkItem) DisplayStringField(key string) string {
	if v, ok := w.DisplayFields[key].(string); ok && v != "" {
		return v
	}
	return w.StringField(key)
}

func (w *WorkItem) StringSliceField(key string) []string {
	if v, ok := w.Fields[key].([]string); ok {
		return v
	}
	return nil
}

// DescriptionMarkdown returns the description rendered as markdown text.
// Returns empty string if Description is nil.
func (w *WorkItem) DescriptionMarkdown() string {
	if w.Description == nil {
		return ""
	}
	return strings.TrimSpace(document.RenderMarkdown(w.Description))
}

// workItemToMap converts a WorkItem to a yaml.MapSlice for manifest
// serialization with deterministic key ordering. Field defs control which
// fields are hoisted to the top level and which are omitted based on
// visibility and the full flag.
func workItemToMap(w *WorkItem, defs []FieldDef, full bool) yaml.MapSlice {
	var s yaml.MapSlice

	if w.ID != "" {
		s = append(s, yaml.MapItem{Key: "key", Value: w.ID})
	}
	s = append(s, yaml.MapItem{Key: "type", Value: w.Type})
	s = append(s, yaml.MapItem{Key: "summary", Value: w.Summary})
	s = append(s, yaml.MapItem{Key: "status", Value: w.Status})

	// Collect top-level fields in definition order.
	claimed := make(map[string]bool, len(defs))
	for _, def := range defs {
		claimed[def.Key] = true

		if def.Visibility != FieldDefault && !full {
			continue
		}

		val, ok := w.Fields[def.Key]
		if !ok {
			continue
		}

		if !full && IsZeroFieldValue(val) {
			continue
		}

		if def.TopLevel {
			// User fields export "none" instead of "" for clarity.
			if def.Type == FieldAssignee && IsZeroFieldValue(val) {
				val = "none"
			}
			s = append(s, yaml.MapItem{Key: def.Key, Value: val})
		}
	}

	// Remaining fields (unclaimed by defs, or non-TopLevel) go in "fields" bag.
	var bagSlice yaml.MapSlice
	for _, def := range defs {
		if !def.TopLevel {
			if v, ok := w.Fields[def.Key]; ok {
				if def.Visibility != FieldDefault && !full {
					continue
				}
				if !full && IsZeroFieldValue(v) {
					continue
				}
				bagSlice = append(bagSlice, yaml.MapItem{Key: def.Key, Value: v})
			}
		}
	}
	// Unclaimed fields sorted alphabetically for stability.
	var unclaimed []string
	for k := range w.Fields {
		if !claimed[k] {
			if !IsZeroFieldValue(w.Fields[k]) || full {
				unclaimed = append(unclaimed, k)
			}
		}
	}
	slices.Sort(unclaimed)
	for _, k := range unclaimed {
		bagSlice = append(bagSlice, yaml.MapItem{Key: k, Value: w.Fields[k]})
	}
	if len(bagSlice) > 0 {
		s = append(s, yaml.MapItem{Key: "fields", Value: bagSlice})
	}

	if desc := w.DescriptionMarkdown(); desc != "" {
		s = append(s, yaml.MapItem{Key: "description", Value: desc})
	}

	if len(w.Children) > 0 {
		children := make([]any, len(w.Children))
		for i, child := range w.Children {
			children[i] = workItemToMap(child, defs, full)
		}
		s = append(s, yaml.MapItem{Key: "children", Value: children})
	}

	return s
}

// workItemFromMap reconstructs a WorkItem from a raw map, routing top-level
// keys into the Fields map based on field defs.
func workItemFromMap(m map[string]any, defs []FieldDef) *WorkItem {
	w := &WorkItem{
		Fields: make(map[string]any),
	}

	if v, ok := m["key"].(string); ok {
		w.ID = v
	}
	if v, ok := m["type"].(string); ok {
		w.Type = v
	}
	if v, ok := m["summary"].(string); ok {
		w.Summary = v
	}
	if v, ok := m["status"].(string); ok {
		w.Status = v
	}
	if v, ok := m["description"].(string); ok && v != "" {
		w.Description, _ = document.ParseMarkdownString(v)
	}

	// Build lookup for top-level field defs.
	topLevelDefs := make(map[string]FieldDef, len(defs))
	for _, def := range defs {
		if def.TopLevel {
			topLevelDefs[def.Key] = def
		}
	}

	// Core keys that are not field-def candidates.
	coreKeys := map[string]bool{
		"key": true, "type": true, "summary": true,
		"status": true, "description": true, "children": true, "fields": true,
	}

	// Route top-level field-def keys into Fields map.
	for k, v := range m {
		if coreKeys[k] {
			continue
		}
		if _, isDef := topLevelDefs[k]; isDef {
			w.Fields[k] = coerceFieldValue(v, topLevelDefs[k])
		}
	}

	// Route nested fields bag into Fields map.
	if bag, ok := m["fields"].(map[string]any); ok {
		maps.Copy(w.Fields, bag)
	}

	// Recursively decode children.
	if rawChildren, ok := m["children"].([]any); ok {
		for _, rc := range rawChildren {
			if cm, ok := rc.(map[string]any); ok {
				child := workItemFromMap(cm, defs)
				child.ParentID = w.ID
				w.Children = append(w.Children, child)
			}
		}
	}

	return w
}

// coerceFieldValue ensures YAML-decoded values match the expected FieldDef type.
// YAML decoders often produce []any for arrays; this converts to []string for
// FieldStringArray defs.
func coerceFieldValue(v any, def FieldDef) any {
	if def.Type == FieldStringArray {
		switch arr := v.(type) {
		case []any:
			strs := make([]string, 0, len(arr))
			for _, item := range arr {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			return strs
		case []string:
			return arr
		}
	}
	return v
}

// IsZeroFieldValue reports whether a field value is considered empty.
func IsZeroFieldValue(v any) bool {
	switch val := v.(type) {
	case string:
		return val == ""
	case []string:
		return len(val) == 0
	case []any:
		return len(val) == 0
	case bool:
		return !val
	case nil:
		return true
	default:
		return false
	}
}

// ContentHash generates a hash of the item's core data and flex fields.
// This is used during export and diffing to detect changes.
func (w *WorkItem) ContentHash() string {
	payload := map[string]any{
		"id":          w.ID,
		"type":        w.Type,
		"summary":     w.Summary,
		"status":      w.Status,
		"description": w.DescriptionMarkdown(),
		"fields":      w.Fields,
	}

	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// StateHash generates a unique signature for an item before it has an ID.
// Used during the Apply flow to safely recover from crashes.
func (w *WorkItem) StateHash(parentID string) string {
	payload := map[string]any{
		"parent":      parentID,
		"type":        w.Type,
		"summary":     w.Summary,
		"description": w.DescriptionMarkdown(),
		"fields":      w.Fields,
	}

	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// Metadata holds session-wide context for the manifest.
type Metadata struct {
	Workspace  string         `json:"workspace" yaml:"workspace"`
	ExportedAt string         `json:"exported_at,omitempty" yaml:"exported_at,omitempty"`
	Context    map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

// Manifest is the root structure for a full file (e.g., a bulk export).
type Manifest struct {
	Metadata Metadata
	Items    []*WorkItem
}

// EncodeManifest writes a Manifest as YAML or JSON, using field defs to
// control field hoisting, visibility, and omission. The format parameter
// should be "yaml" or "json".
func EncodeManifest(w io.Writer, m *Manifest, defs []FieldDef, full bool, format string) error {
	items := make([]any, len(m.Items))
	for i, item := range m.Items {
		items[i] = workItemToMap(item, defs, full)
	}

	meta := yaml.MapSlice{
		{Key: "workspace", Value: m.Metadata.Workspace},
	}
	if m.Metadata.ExportedAt != "" {
		meta = append(meta, yaml.MapItem{Key: "exported_at", Value: m.Metadata.ExportedAt})
	}
	if len(m.Metadata.Context) > 0 {
		meta = append(meta, yaml.MapItem{Key: "context", Value: m.Metadata.Context})
	}

	doc := yaml.MapSlice{
		{Key: "metadata", Value: meta},
		{Key: "items", Value: items},
	}

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(mapSliceToMap(doc))
	default: // yaml
		enc := yaml.NewEncoder(w, yaml.UseLiteralStyleIfMultiline(true))
		return enc.Encode(doc)
	}
}

// mapSliceToMap recursively converts yaml.MapSlice to map[string]any for
// JSON encoding, which doesn't understand MapSlice.
func mapSliceToMap(v any) any {
	switch val := v.(type) {
	case yaml.MapSlice:
		m := make(map[string]any, len(val))
		for _, item := range val {
			m[fmt.Sprint(item.Key)] = mapSliceToMap(item.Value)
		}
		return m
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = mapSliceToMap(elem)
		}
		return out
	default:
		return v
	}
}

// DecodeManifest reads YAML or JSON bytes into a Manifest, using field defs
// to route top-level keys into the Fields map on each WorkItem.
func DecodeManifest(data []byte, defs []FieldDef) (*Manifest, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	m := &Manifest{}

	// Decode metadata.
	if meta, ok := raw["metadata"].(map[string]any); ok {
		if v, ok := meta["workspace"].(string); ok {
			m.Metadata.Workspace = v
		}
		if v, ok := meta["exported_at"].(string); ok {
			m.Metadata.ExportedAt = v
		}
		if v, ok := meta["context"].(map[string]any); ok {
			m.Metadata.Context = v
		}
	}

	// Decode items.
	if rawItems, ok := raw["items"].([]any); ok {
		for _, ri := range rawItems {
			if itemMap, ok := ri.(map[string]any); ok {
				m.Items = append(m.Items, workItemFromMap(itemMap, defs))
			}
		}
	}

	return m, nil
}

const ManifestStr = "manifest"

// ManifestSchema generates the JSON Schema for bulk manifests.
// Field defs drive the item properties: top-level defs become item-level
// schema properties with appropriate types and enums.
func ManifestSchema(ws *Workspace, defs []FieldDef) *jsonschema.Schema {
	typeEnums := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeEnums = append(typeEnums, t.Name)
	}

	statusEnums := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusEnums = append(statusEnums, st)
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
		if !def.TopLevel {
			continue
		}
		switch def.Type {
		case FieldString:
			itemProps[def.Key] = &jsonschema.Schema{Type: "string"}
		case FieldEmail:
			itemProps[def.Key] = &jsonschema.Schema{Type: "string", Format: "email"}
		case FieldAssignee:
			itemProps[def.Key] = &jsonschema.Schema{
				AnyOf: []*jsonschema.Schema{
					{Type: "string", Enum: []any{"unassigned", "none"}},
					{Type: "string", Format: "email"},
				},
			}
		case FieldEnum:
			enums := make([]any, len(def.Enum))
			for i, e := range def.Enum {
				enums[i] = e
			}
			itemProps[def.Key] = &jsonschema.Schema{Type: "string", Enum: enums}
		case FieldStringArray:
			itemProps[def.Key] = &jsonschema.Schema{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			}
		case FieldBool:
			itemProps[def.Key] = &jsonschema.Schema{Type: "boolean"}
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
