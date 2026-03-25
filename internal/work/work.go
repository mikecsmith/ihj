// Package work defines the universal domain model for work orchestration.
//
// It abstracts backend-specific concepts (like Jira issues, Trello cards,
// or GitHub issues) into a standardized WorkItem structure. This allows
// the core application to validate, diff, and manipulate hierarchies of
// tasks without needing to understand the underlying tracking provider.
//
// TODO(refactor): Separate Infrastructure from Domain
//   - Move WriteSchema and any other file I/O operations to a dedicated
//     `file` or `storage` package. The `work` package should only generate
//     the schema, not decide where it lives on disk.
//   - Consider moving BuildFrontmatterDoc and ParseFrontmatter into a
//     separate package. Serialization/Formatting
//     logic should ideally be decoupled from the core structs.
//
// TODO(refactor): Formalize the Backend Provider
//   - Define a `work.Provider` interface
//     that the `jira` package will implement. This will fully sever the
//     commands package from knowing about Jira specifically.
//   - Standardize the `Fields` flex-bucket keys across backends if possible,
//     or clearly document the expected `Context` and `Fields` payloads
//     for the current Jira adapter.
package work

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
	"github.com/mikecsmith/ihj/internal/config"
)

// --- Structs ---

// BaseFrontmatter defines the static fields for the editor's YAML block.
// (Left untouched for now as requested)
type BaseFrontmatter struct {
	Key      string `json:"key,omitempty" jsonschema:"Existing Jira issue key (e.g., INFRA-123). Omit if creating new."`
	Summary  string `json:"summary"`
	Type     string `json:"type"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
	Parent   string `json:"parent,omitempty"`
	Sprint   bool   `json:"sprint,omitempty"`
}

// WorkItem represents a universal unit of work (Issue, Card, Task, etc.)
type WorkItem struct {
	// We use ID in Go, but keep 'key' in JSON/YAML for safe migration with existing Jira logic
	ID          string `json:"key,omitempty" yaml:"key,omitempty" jsonschema:"Existing Jira issue key. Omit for new issues."`
	Type        string `json:"type" yaml:"type"`
	Summary     string `json:"summary" yaml:"summary"`
	Status      string `json:"status" yaml:"status"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Fields holds arbitrary backend-specific data (Priority, Sprint, Team, etc.)
	Fields map[string]any `json:"fields,omitempty" yaml:"fields,omitempty"`

	Children []*WorkItem `json:"children,omitempty" yaml:"children,omitempty" jsonschema:"Nested child issues or sub-tasks."`
}

// ContentHash generates a hash of the item's core data and flex fields.
// This is used during export and diffing to detect changes.
func (w *WorkItem) ContentHash() string {
	payload := map[string]any{
		"id":          w.ID,
		"type":        w.Type,
		"summary":     w.Summary,
		"status":      w.Status,
		"description": w.Description,
		"fields":      w.Fields,
	}

	// json.Marshal guarantees stable sorting of the 'fields' map keys
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
		"description": w.Description,
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

// FrontmatterSchema generates the JSON Schema used by the editors yaml-language-server schema comment.
func FrontmatterSchema(cfg *config.Config, board *config.BoardConfig) *jsonschema.Schema {
	typeNames := make([]any, 0, len(board.Types))
	for _, t := range board.Types {
		typeNames = append(typeNames, t.Name)
	}

	priorityNames := []any{"Highest", "High", "Medium", "Low", "Lowest", "Unprioritised"}

	transitionNames := make([]any, 0, len(board.Transitions))
	for _, st := range board.Transitions {
		transitionNames = append(transitionNames, st)
	}

	properties := map[string]*jsonschema.Schema{
		"key":      {Type: "string", Description: "Existing Jira issue key (e.g., INFRA-123). Omit if creating new."},
		"summary":  {Type: "string"},
		"type":     {Type: "string", Enum: typeNames},
		"priority": {Type: "string", Enum: priorityNames},
		"status":   {Type: "string", Enum: transitionNames},
		"parent":   {Type: "string"},
		"sprint":   {Type: "boolean"},
	}

	for cfName := range cfg.CustomFields {
		if cfName == "team" {
			properties[cfName] = &jsonschema.Schema{Types: []string{"boolean", "string"}}
		} else {
			properties[cfName] = &jsonschema.Schema{Type: "string"}
		}
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

func ManifestSchema(board *config.BoardConfig) *jsonschema.Schema {
	// 1. Prepare dynamic enums
	typeEnums := make([]any, 0, len(board.Types))
	for _, t := range board.Types {
		typeEnums = append(typeEnums, t.Name)
	}

	statusEnums := make([]any, 0, len(board.Transitions))
	for _, st := range board.Transitions {
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
			"item": issueSchema, // Reference matches '#/$defs/item'
		},
	}
}

// WriteSchema writes a JSON schema to the cache directory.
func WriteSchema(cacheDir, boardSlug, name string, schema any) (string, error) {
	filename := fmt.Sprintf("%s.%s.schema.json", name, boardSlug)
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
// (Leaving these mostly as-is until we update them to use the Flex Bucket)

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
