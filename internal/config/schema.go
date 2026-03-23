// Package config manages the application configuration and dynamic JSON schema generation.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
)

// BaseFrontmatter defines the static fields for the editor's YAML block.
type BaseFrontmatter struct {
	Key      string `json:"key,omitempty" jsonschema:"Existing Jira issue key (e.g., INFRA-123). Omit if creating new."`
	Summary  string `json:"summary"`
	Type     string `json:"type"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
	Parent   string `json:"parent,omitempty"`
	Sprint   bool   `json:"sprint,omitempty"`
}

// ExportRoot represents a top-level issue (e.g., Epic).
type ExportRoot struct {
	Key         string         `json:"key,omitempty" jsonschema:"Existing Jira issue key (e.g., INFRA-123). Omit if creating new."`
	Type        string         `json:"type"`
	Summary     string         `json:"summary"`
	Status      string         `json:"status,omitempty"`
	Description string         `json:"description,omitempty" jsonschema:"Markdown formatted description."`
	Parent      string         `json:"parent,omitempty" jsonschema:"Optional parent issue key."`
	Children    []*ExportChild `json:"children,omitempty" jsonschema:"Child issues (e.g., Stories, Tasks)."`
}

// ExportChild represents a standard issue (e.g., Story, Task) that belongs to a root.
type ExportChild struct {
	Key         string           `json:"key,omitempty" jsonschema:"Existing Jira issue key."`
	Type        string           `json:"type"`
	Summary     string           `json:"summary"`
	Status      string           `json:"status,omitempty"`
	Description string           `json:"description,omitempty" jsonschema:"Markdown formatted description."`
	Children    []*ExportSubtask `json:"children,omitempty" jsonschema:"Nested sub-tasks."`
}

// ExportSubtask represents a sub-task at the bottom of the hierarchy.
type ExportSubtask struct {
	Key         string `json:"key,omitempty" jsonschema:"Existing Jira issue key."`
	Type        string `json:"type"`
	Summary     string `json:"summary"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty" jsonschema:"Markdown formatted description."`
}

// FrontmatterSchema generates the JSON Schema used by yaml-language-server.
func FrontmatterSchema(cfg *Config, board *BoardConfig) map[string]any {
	schema, err := jsonschema.For[BaseFrontmatter](&jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to generate Frontmatter schema: %w", err))
	}

	b, _ := json.Marshal(schema)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		panic(fmt.Errorf("failed to unmarshal Frontmatter schema: %w", err))
	}

	properties, ok := result["properties"].(map[string]any)
	if !ok {
		properties = make(map[string]any)
		result["properties"] = properties
	}

	// Inject Enums
	typeNames := make([]string, 0, len(board.Types))
	for _, t := range board.Types {
		typeNames = append(typeNames, t.Name)
	}

	if properties["type"] == nil {
		properties["type"] = map[string]any{"type": "string"}
	}
	properties["type"].(map[string]any)["enum"] = typeNames

	if properties["priority"] == nil {
		properties["priority"] = map[string]any{"type": "string"}
	}
	properties["priority"].(map[string]any)["enum"] = []string{"Highest", "High", "Medium", "Low", "Lowest"}

	if properties["status"] == nil {
		properties["status"] = map[string]any{"type": "string"}
	}
	if len(board.Transitions) > 0 {
		properties["status"].(map[string]any)["enum"] = board.Transitions
	}

	// Inject Custom Fields
	for cfName := range cfg.CustomFields {
		if cfName == "team" {
			properties["team"] = map[string]any{"type": []string{"boolean", "string"}}
		} else {
			properties[cfName] = map[string]any{"type": "string"}
		}
	}

	// Add Required fields and Conditional Validation
	result["required"] = []string{"summary", "type"}
	result["allOf"] = []any{
		map[string]any{
			"if":   map[string]any{"properties": map[string]any{"type": map[string]any{"const": "Sub-task"}}},
			"then": map[string]any{"required": []string{"parent"}},
		},
	}

	return result
}

// injectEnums applies dynamic enum values to a given schema definition object.
func injectEnums(issueDef map[string]any, board *BoardConfig) {
	props, ok := issueDef["properties"].(map[string]any)
	if !ok {
		props = make(map[string]any)
		issueDef["properties"] = props
	}

	typeNames := make([]string, 0, len(board.Types))
	for _, t := range board.Types {
		typeNames = append(typeNames, t.Name)
	}

	if props["type"] == nil {
		props["type"] = map[string]any{"type": "string"}
	}
	props["type"].(map[string]any)["enum"] = typeNames

	if props["status"] == nil {
		props["status"] = map[string]any{"type": "string"}
	}
	if len(board.Transitions) > 0 {
		props["status"].(map[string]any)["enum"] = board.Transitions
	}

	issueDef["required"] = []string{"summary", "type"}
}

// HierarchySchema generates the JSON Schema for LLM extract output.
func HierarchySchema(board *BoardConfig) map[string]any {
	schema, err := jsonschema.For[[]*ExportRoot](&jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to generate Hierarchy schema: %w", err))
	}

	b, _ := json.Marshal(schema)
	var result map[string]any
	if err := json.Unmarshal(b, &result); err != nil {
		panic(fmt.Errorf("failed to unmarshal Hierarchy schema: %w", err))
	}

	if defs, ok := result["$defs"].(map[string]any); ok {
		// Apply Enums cleanly to all three levels of the hierarchy
		for _, defName := range []string{"ExportRoot", "ExportChild", "ExportSubtask"} {
			if def, ok := defs[defName].(map[string]any); ok {
				injectEnums(def, board)
			}
		}
	}

	return result
}

// WriteFrontmatterSchema writes the schema to disk and returns the path.
func WriteFrontmatterSchema(cacheDir, boardSlug string, schema map[string]any) (string, error) {
	path := filepath.Join(cacheDir, fmt.Sprintf("frontmatter.%s.schema.json", boardSlug))
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling schema: %w", err)
	}
	// Upgraded to 0600 for safer local file permissions
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing schema: %w", err)
	}
	return path, nil
}

// BuildFrontmatterDoc constructs the Markdown+YAML frontmatter string for the editor.
func BuildFrontmatterDoc(schemaPath string, metadata map[string]string, bodyText string) string {
	var lines []string
	lines = append(lines, "---")
	lines = append(lines, fmt.Sprintf("# yaml-language-server: $schema=file://%s", schemaPath))

	// Core fields in a fixed order for UX consistency.
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

	// Custom fields.
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

	// Summary always last so it's adjacent to the body.
	// Always include summary (even empty) so cursor positioning works.
	if v := metadata["summary"]; v != "" {
		lines = append(lines, fmt.Sprintf("summary: \"%s\"", v))
	} else {
		lines = append(lines, "summary: \"\"")
	}

	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, bodyText)

	return strings.Join(lines, "\n")
}

// ParseFrontmatter splits a frontmatter+body string and parses the YAML portion.
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

	// Flatten to string map for consistency.
	result := make(map[string]string)
	for k, v := range parsed {
		result[k] = fmt.Sprintf("%v", v)
	}

	return result, body, nil
}
