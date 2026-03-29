package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/document"
)

// Frontmatter is the schema name used for caching.
const Frontmatter = "frontmatter"

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

// frontmatterFieldOrder defines the display order for known frontmatter fields.
// Summary is always emitted last (closest to the body) for editor ergonomics.
var frontmatterFieldOrder = []string{"key", "type", "priority", "status", "parent"}

// BuildFrontmatterDoc assembles a YAML-frontmatter document for the editor.
// Field ordering is deterministic: known fields appear in a fixed order,
// followed by any extra fields, with summary always last. Quoting is
// delegated to yaml.Marshal so special characters are handled correctly.
func BuildFrontmatterDoc(schemaPath string, metadata map[string]string, bodyText string) string {
	var s yaml.MapSlice

	// Known fields in display order.
	for _, k := range frontmatterFieldOrder {
		if v := metadata[k]; v != "" {
			s = append(s, yaml.MapItem{Key: k, Value: v})
		}
	}

	// Extra fields (excluding summary and ordered fields).
	for k, v := range metadata {
		if k == "summary" || slices.Contains(frontmatterFieldOrder, k) || v == "" {
			continue
		}
		s = append(s, yaml.MapItem{Key: k, Value: coerceFrontmatterValue(v)})
	}

	// Summary always last — closest to the markdown body for easy editing.
	if v := metadata["summary"]; v != "" {
		s = append(s, yaml.MapItem{Key: "summary", Value: v})
	} else {
		s = append(s, yaml.MapItem{Key: "summary", Value: nil})
	}

	yamlBytes, _ := yaml.Marshal(s)

	// Clean up null values for a friendlier editor experience.
	// e.g. `summary: null` becomes `summary: ` — YAML parses both as empty.
	// The trailing space keeps the cursor positioned naturally after the colon.
	yamlStr := strings.ReplaceAll(string(yamlBytes), ": null", ": ")

	var lines []string
	lines = append(lines, "---")
	lines = append(lines, fmt.Sprintf("# yaml-language-server: $schema=file://%s", schemaPath))
	lines = append(lines, strings.TrimSpace(yamlStr))
	lines = append(lines, "---", "", bodyText)
	return strings.Join(lines, "\n")
}

// coerceFrontmatterValue converts string values to typed values where
// appropriate so that yaml.Marshal produces clean output (e.g. true
// instead of "true").
func coerceFrontmatterValue(v string) any {
	lower := strings.ToLower(v)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}
	return v
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
		if v == nil {
			result[k] = ""
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return result, body, nil
}

// WorkItemToMetadata converts a WorkItem to the frontmatter metadata map
// used by the editor.
func WorkItemToMetadata(item *WorkItem) map[string]string {
	m := map[string]string{
		"key":     item.ID,
		"type":    item.Type,
		"status":  item.Status,
		"summary": item.Summary,
	}
	if item.ParentID != "" {
		m["parent"] = item.ParentID
	}
	if v := item.StringField("priority"); v != "" {
		m["priority"] = v
	}
	return m
}

// FrontmatterToWorkItem builds a WorkItem from parsed frontmatter and
// a description AST. Used by the create flow.
func FrontmatterToWorkItem(fm map[string]string, description *document.Node) *WorkItem {
	item := &WorkItem{
		Summary: fm["summary"],
		Type:    fm["type"],
		Status:  fm["status"],
	}
	if fm["parent"] != "" {
		item.ParentID = fm["parent"]
	}
	if description != nil {
		item.Description = description
	}
	fields := make(map[string]any)
	if fm["priority"] != "" {
		fields["priority"] = fm["priority"]
	}
	if strings.EqualFold(fm["sprint"], "true") {
		fields["sprint"] = true
	}
	if len(fields) > 0 {
		item.Fields = fields
	}
	return item
}

// FrontmatterToChanges builds a Changes struct from edited frontmatter,
// comparing against the original work item to detect modifications.
func FrontmatterToChanges(fm map[string]string, description *document.Node, origItem *WorkItem) *Changes {
	changes := &Changes{}
	hasChange := false

	if fm["summary"] != origItem.Summary {
		changes.Summary = new(fm["summary"])
		hasChange = true
	}
	if !strings.EqualFold(fm["type"], origItem.Type) {
		changes.Type = new(fm["type"])
		hasChange = true
	}
	if !strings.EqualFold(fm["status"], origItem.Status) {
		changes.Status = new(fm["status"])
		hasChange = true
	}
	if fm["parent"] != origItem.ParentID {
		changes.ParentID = new(fm["parent"])
		hasChange = true
	}
	if description != nil {
		newMD := strings.TrimSpace(document.RenderMarkdown(description))
		origMD := origItem.DescriptionMarkdown()
		if newMD != origMD {
			changes.Description = description
			hasChange = true
		}
	}

	fields := make(map[string]any)
	if fm["priority"] != "" && fm["priority"] != origItem.StringField("priority") {
		fields["priority"] = fm["priority"]
	}
	if strings.EqualFold(fm["sprint"], "true") {
		fields["sprint"] = true
	}
	if len(fields) > 0 {
		changes.Fields = fields
		hasChange = true
	}

	if !hasChange {
		return nil
	}
	return changes
}
