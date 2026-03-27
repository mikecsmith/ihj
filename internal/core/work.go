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
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/document"
)

// --- Structs ---

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

// workItemJSON is the serialization shape for JSON/YAML.
type workItemJSON struct {
	Key         string         `json:"key,omitempty" yaml:"key,omitempty"`
	Type        string         `json:"type" yaml:"type"`
	Summary     string         `json:"summary" yaml:"summary"`
	Status      string         `json:"status" yaml:"status"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Fields      map[string]any `json:"fields,omitempty" yaml:"fields,omitempty"`
	Children    []*WorkItem    `json:"children,omitempty" yaml:"children,omitempty"`
}

func (w WorkItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(workItemJSON{
		Key:         w.ID,
		Type:        w.Type,
		Summary:     w.Summary,
		Status:      w.Status,
		Description: w.DescriptionMarkdown(),
		Fields:      w.Fields,
		Children:    w.Children,
	})
}

func (w *WorkItem) UnmarshalJSON(data []byte) error {
	var aux workItemJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	w.ID = aux.Key
	w.Type = aux.Type
	w.Summary = aux.Summary
	w.Status = aux.Status
	w.Fields = aux.Fields
	w.Children = aux.Children
	if aux.Description != "" {
		w.Description, _ = document.ParseMarkdownString(aux.Description)
	}
	return nil
}

func (w WorkItem) MarshalYAML() (interface{}, error) {
	return workItemJSON{
		Key:         w.ID,
		Type:        w.Type,
		Summary:     w.Summary,
		Status:      w.Status,
		Description: w.DescriptionMarkdown(),
		Fields:      w.Fields,
		Children:    w.Children,
	}, nil
}

func (w *WorkItem) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux workItemJSON
	if err := unmarshal(&aux); err != nil {
		return err
	}
	w.ID = aux.Key
	w.Type = aux.Type
	w.Summary = aux.Summary
	w.Status = aux.Status
	w.Fields = aux.Fields
	w.Children = aux.Children
	if aux.Description != "" {
		w.Description, _ = document.ParseMarkdownString(aux.Description)
	}
	return nil
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
	Backend    string         `json:"backend" yaml:"backend"`
	Target     string         `json:"target" yaml:"target"`
	ExportedAt string         `json:"exported_at" yaml:"exported_at"`
	Context    map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

// Manifest is the root structure for a full file (e.g., a bulk export).
type Manifest struct {
	Metadata Metadata    `json:"metadata" yaml:"metadata"`
	Items    []*WorkItem `json:"items" yaml:"items"`
}

const (
	Frontmatter = "frontmatter"
	ManifestStr = "manifest"
)

// --- Schemas ---

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
func ManifestSchema(ws *Workspace) *jsonschema.Schema {
	typeEnums := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeEnums = append(typeEnums, t.Name)
	}

	statusEnums := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusEnums = append(statusEnums, st)
	}

	issueSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"key":         {Type: "string"},
			"summary":     {Type: "string"},
			"type":        {Type: "string", Enum: typeEnums},
			"status":      {Type: "string", Enum: statusEnums},
			"description": {Type: "string"},
			"fields": {
				Type: "object",
			},
			"children": {
				Type:  "array",
				Items: &jsonschema.Schema{Ref: "#/$defs/item"},
			},
		},
		Required:             []string{"summary", "type"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}

	metadataSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"backend":     {Type: "string"},
			"target":      {Type: "string"},
			"exported_at": {Type: "string"},
			"context": {
				Type: "object",
			},
		},
		Required:             []string{"backend", "target"},
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

// WriteSchema writes a JSON schema to the cache directory.
func WriteSchema(cacheDir, workspaceSlug, name string, schema any) (string, error) {
	filename := fmt.Sprintf("%s.%s.schema.json", name, workspaceSlug)
	path := filepath.Join(cacheDir, filename)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling %s schema: %w", name, err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing %s schema: %w", name, err)
	}

	return path, nil
}

// --- Frontmatter String Manipulation ---

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
