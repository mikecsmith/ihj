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
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/document"
)

// BaseFrontmatter defines the static fields for the editor's YAML block.
type BaseFrontmatter struct {
	Key      string `json:"key,omitempty" jsonschema:"Existing issue key (e.g., ENG-123). Omit if creating new."`
	Summary  string `json:"summary"`
	Type     string `json:"type"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
	Parent   string `json:"parent,omitempty"`
	Sprint   bool   `json:"sprint,omitempty"`
}

// WorkItem represents a universal unit of work (Issue, Card, Task, etc.)
type WorkItem struct {
	// We use ID in Go, but keep 'key' in JSON/YAML for safe migration with existing logic.
	ID       string `json:"-" yaml:"-"`
	Type     string `json:"-" yaml:"-"`
	Summary  string `json:"-" yaml:"-"`
	Status   string `json:"-" yaml:"-"`
	ParentID string `json:"-" yaml:"-"`

	// Description is the AST representation — the interchange format.
	// Serialized as markdown text via custom marshal/unmarshal.
	Description *document.Node `json:"-" yaml:"-"`

	// Comments on this work item.
	Comments []Comment `json:"-" yaml:"-"`

	// Fields holds arbitrary backend-specific data (Priority, Sprint, Team, etc.)
	Fields map[string]any `json:"-" yaml:"-"`

	Children []*WorkItem `json:"-" yaml:"-"`
}

// Field accessors for common Fields entries.

func (w *WorkItem) StringField(key string) string {
	if v, ok := w.Fields[key].(string); ok {
		return v
	}
	return ""
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

// workItemToMap converts a WorkItem to a map[string]any for manifest
// serialization. Field defs control which fields are hoisted to the top
// level and which are omitted based on visibility and the full flag.
func workItemToMap(w *WorkItem, defs []FieldDef, full bool) map[string]any {
	m := make(map[string]any)

	if w.ID != "" {
		m["key"] = w.ID
	}
	m["type"] = w.Type
	m["summary"] = w.Summary
	m["status"] = w.Status

	// Build a set of keys claimed by field defs so we know what's "remaining".
	claimed := make(map[string]bool, len(defs))
	for _, def := range defs {
		claimed[def.Key] = true

		// Visibility filter: extended and readonly only appear with --full.
		if def.Visibility != FieldDefault && !full {
			continue
		}

		val, ok := w.Fields[def.Key]
		if !ok {
			continue
		}

		// Skip zero values unless full mode.
		if !full && IsZeroFieldValue(val) {
			continue
		}

		if def.TopLevel {
			m[def.Key] = val
		}
		// Non-TopLevel fields fall through to the fields bag below.
	}

	// Remaining fields (unclaimed by defs, or non-TopLevel) go in "fields" bag.
	bag := make(map[string]any)
	for k, v := range w.Fields {
		if claimed[k] {
			// Already handled above; if non-TopLevel, put in bag.
			for _, def := range defs {
				if def.Key == k && !def.TopLevel {
					if def.Visibility != FieldDefault && !full {
						continue
					}
					if !full && IsZeroFieldValue(v) {
						continue
					}
					bag[k] = v
				}
			}
			continue
		}
		// Unclaimed fields always go in the bag.
		if !IsZeroFieldValue(v) || full {
			bag[k] = v
		}
	}
	if len(bag) > 0 {
		m["fields"] = bag
	}

	if desc := w.DescriptionMarkdown(); desc != "" {
		m["description"] = desc
	}

	if len(w.Children) > 0 {
		children := make([]any, len(w.Children))
		for i, child := range w.Children {
			children[i] = workItemToMap(child, defs, full)
		}
		m["children"] = children
	}

	return m
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
		for k, v := range bag {
			w.Fields[k] = v
		}
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

	meta := map[string]any{
		"workspace": m.Metadata.Workspace,
	}
	if m.Metadata.ExportedAt != "" {
		meta["exported_at"] = m.Metadata.ExportedAt
	}
	if len(m.Metadata.Context) > 0 {
		meta["context"] = m.Metadata.Context
	}

	doc := map[string]any{
		"metadata": meta,
		"items":    items,
	}

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	default: // yaml
		enc := yaml.NewEncoder(w, yaml.UseLiteralStyleIfMultiline(true))
		return enc.Encode(doc)
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

const (
	Frontmatter = "frontmatter"
	ManifestStr = "manifest"
)

// FrontmatterSchema generates the JSON Schema for the editor's YAML frontmatter.
func FrontmatterSchema(ws *Workspace) *jsonschema.Schema {
	typeNames := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeNames = append(typeNames, t.Name)
	}

	priorityNames := []any{"Highest", "High", "Medium", "Low", "Lowest", "Unprioritised"}

	statusNames := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusNames = append(statusNames, st)
	}

	properties := map[string]*jsonschema.Schema{
		"key":      {Type: "string", Description: "Existing issue key (e.g., ENG-123). Omit if creating new."},
		"summary":  {Type: "string"},
		"type":     {Type: "string", Enum: typeNames},
		"priority": {Type: "string", Enum: priorityNames},
		"status":   {Type: "string", Enum: statusNames},
		"parent":   {Type: "string"},
		"sprint":   {Type: "boolean"},
	}

	subTaskConst := any("Sub-task")

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"summary", "type"},
		AllOf: []*jsonschema.Schema{
			{
				If: &jsonschema.Schema{
					Properties: map[string]*jsonschema.Schema{
						"type": {Const: &subTaskConst},
					},
				},
				Then: &jsonschema.Schema{
					Required: []string{"parent"},
				},
			},
		},
	}
}

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

// BuildFrontmatterDoc assembles a YAML-frontmatter document for the editor.
func BuildFrontmatterDoc(schemaPath string, metadata map[string]string, bodyText string) string {
	var lines []string
	lines = append(lines, "---")
	lines = append(lines, fmt.Sprintf("# yaml-language-server: $schema=file://%s", schemaPath))

	order := []string{"key", "type", "priority", "status", "parent"}
	for _, k := range order {
		val := metadata[k]
		if val == "" {
			continue
		}
		if k == "key" {
			lines = append(lines, fmt.Sprintf("key: %s", val))
		} else {
			lines = append(lines, fmt.Sprintf("%s: \"%s\"", k, val))
		}
	}

	for k, v := range metadata {
		if k == "summary" || slices.Contains(order, k) || v == "" {
			continue
		}
		lower := strings.ToLower(v)
		if lower == "true" || lower == "false" {
			lines = append(lines, fmt.Sprintf("%s: %s", k, lower))
		} else {
			lines = append(lines, fmt.Sprintf("%s: \"%s\"", k, v))
		}
	}

	if v := metadata["summary"]; v != "" {
		lines = append(lines, fmt.Sprintf("summary: \"%s\"", v))
	} else {
		lines = append(lines, "summary: \"\"")
	}

	lines = append(lines, "---", "", bodyText)
	return strings.Join(lines, "\n")
}

// ValidateFrontmatter checks domain rules on parsed frontmatter.
// Returns an error message string, or "" if valid.
func ValidateFrontmatter(fm map[string]string) string {
	if fm["summary"] == "" {
		return "Summary is required."
	}
	if strings.EqualFold(fm["type"], "sub-task") && fm["parent"] == "" {
		return "Sub-tasks require a parent issue key."
	}
	return ""
}

// ParseFrontmatter splits a YAML-frontmatter document into metadata and body.
func ParseFrontmatter(raw string) (map[string]string, string, error) {
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return nil, strings.TrimSpace(raw), nil
	}

	yamlStr := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		return nil, body, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	result := make(map[string]string)
	for k, v := range parsed {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result, body, nil
}
