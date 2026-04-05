// Package encoding owns all serialization between core.WorkItem and
// external representations (YAML manifests, editor frontmatter, JSON Schema).
// The core package stays pure — this package is its boundary layer.
package encoding

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// ManifestStr is the schema name used for caching.
const ManifestStr = "manifest"

// Metadata holds session-wide context for the manifest.
type Metadata struct {
	Workspace  string         `json:"workspace" yaml:"workspace"`
	ExportedAt string         `json:"exported_at,omitempty" yaml:"exported_at,omitempty"`
	Context    map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

// Manifest is the root structure for a full file (e.g., a bulk export).
type Manifest struct {
	Metadata Metadata
	Items    []*core.WorkItem
}

// EncodeManifest writes a Manifest as YAML or JSON, using field defs to
// control field hoisting, visibility, and omission. The format parameter
// should be "yaml" or "json".
func EncodeManifest(w io.Writer, m *Manifest, defs []core.FieldDef, full bool, format string) error {
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
		enc := yaml.NewEncoder(w, yaml.UseLiteralStyleIfMultiline(true), yaml.IndentSequence(true))
		return enc.Encode(doc)
	}
}

// DecodeManifest reads YAML or JSON bytes into a Manifest, using field defs
// to route top-level keys into the Fields map on each WorkItem.
func DecodeManifest(data []byte, defs []core.FieldDef) (*Manifest, error) {
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

// workItemToMap converts a WorkItem to a yaml.MapSlice for manifest
// serialization with deterministic key ordering. Field defs control which
// fields are hoisted to the top level and which are omitted based on
// visibility and the full flag.
func workItemToMap(w *core.WorkItem, defs []core.FieldDef, full bool) yaml.MapSlice {
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

		// Rich text fields are rendered for display but not (yet)
		// serialised into the manifest format.
		if def.Type == core.FieldRichText {
			continue
		}

		if !def.ExportDefault() && !full {
			continue
		}

		val, ok := w.Fields[def.Key]
		if !ok {
			continue
		}

		if !full && core.IsZeroFieldValue(val) {
			continue
		}

		if def.Prominent() {
			// User fields export "none" instead of "" for clarity.
			if def.Type == core.FieldAssignee && core.IsZeroFieldValue(val) {
				val = "none"
			}
			key := def.Key
			if def.Informational() {
				key = "_" + key // informational only — ignored on import
			}
			s = append(s, yaml.MapItem{Key: key, Value: val})
		}
	}

	// Remaining fields (unclaimed by defs, or non-TopLevel) go in "fields" bag.
	var bagSlice yaml.MapSlice
	for _, def := range defs {
		if !def.Prominent() {
			if v, ok := w.Fields[def.Key]; ok {
				if !def.ExportDefault() && !full {
					continue
				}
				if !full && core.IsZeroFieldValue(v) {
					continue
				}
				key := def.Key
				if def.Informational() {
					key = "_" + key
				}
				bagSlice = append(bagSlice, yaml.MapItem{Key: key, Value: v})
			}
		}
	}
	// Unclaimed fields sorted alphabetically for stability.
	var unclaimed []string
	for k := range w.Fields {
		if !claimed[k] {
			if !core.IsZeroFieldValue(w.Fields[k]) || full {
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
func workItemFromMap(m map[string]any, defs []core.FieldDef) *core.WorkItem {
	w := &core.WorkItem{
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
	topLevelDefs := make(map[string]core.FieldDef, len(defs))
	for _, def := range defs {
		if def.Prominent() {
			topLevelDefs[def.Key] = def
		}
	}

	// Route top-level field-def keys into Fields map.
	// Reserved keys (core content, identity, structural containers) are skipped —
	// they are handled by dedicated decoders above and below.
	for k, v := range m {
		if core.IsReservedKey(k) {
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
func coerceFieldValue(v any, def core.FieldDef) any {
	if def.Type == core.FieldStringArray {
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
