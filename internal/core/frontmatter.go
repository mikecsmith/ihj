package core

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mikecsmith/ihj/internal/document"
)

// Frontmatter is the schema name used for caching.
const Frontmatter = "frontmatter"

// FrontmatterSchema generates the JSON Schema for the editor's YAML frontmatter.
// Field defs drive provider-specific properties (e.g., sprint for scrum boards).
func FrontmatterSchema(ws *Workspace, defs []FieldDef) *jsonschema.Schema {
	typeNames := make([]any, 0, len(ws.Types))
	for _, t := range ws.Types {
		typeNames = append(typeNames, t.Name)
	}

	statusNames := make([]any, 0, len(ws.Statuses))
	for _, st := range ws.Statuses {
		statusNames = append(statusNames, st.Name)
	}

	properties := map[string]*jsonschema.Schema{
		"key":     {Type: "string", Description: "Existing issue key (e.g., ENG-123, 51). Omit if creating new."},
		"summary": {Type: "string"},
		"type":    {Type: "string", Enum: typeNames},
		"status":  {Type: "string", Enum: statusNames},
		"parent":  {Type: "string"},
	}

	// Add field-def-driven properties: top-level fields and required fields.
	for _, def := range defs {
		if !def.IncludeInSchema() || (!def.TopLevelField() && !def.Required) {
			continue
		}
		switch def.Type {
		case FieldEnum:
			enums := make([]any, len(def.Enum))
			for i, e := range def.Enum {
				enums[i] = e
			}
			properties[def.Key] = &jsonschema.Schema{Type: "string", Enum: enums}
		case FieldString:
			properties[def.Key] = &jsonschema.Schema{Type: "string"}
		case FieldStringArray:
			properties[def.Key] = &jsonschema.Schema{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			}
		case FieldBool:
			properties[def.Key] = &jsonschema.Schema{Type: "boolean"}
		case FieldAssignee, FieldEmail:
			properties[def.Key] = &jsonschema.Schema{Type: "string"}
		}
	}

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"summary", "type"},
	}
}

// frontmatterCoreOrder defines the display order for structural frontmatter
// fields. Provider-driven fields (from FieldDefs) are inserted between
// type and status. Summary is always emitted last (closest to the body).
var frontmatterCoreOrder = []string{"key", "type", "status", "parent"}

// BuildFrontmatterDoc assembles a YAML-frontmatter document for the editor.
// Field ordering is deterministic: core fields first (key, type, status,
// parent), then provider-driven fields by role, with summary always last.
// Quoting is delegated to yaml.Marshal so special characters are handled
// correctly.
func BuildFrontmatterDoc(schemaPath string, metadata map[string]string, bodyText string) string {
	var s yaml.MapSlice
	emitted := make(map[string]bool)

	// Core structural fields in fixed order.
	for _, k := range frontmatterCoreOrder {
		if v := metadata[k]; v != "" {
			s = append(s, yaml.MapItem{Key: k, Value: v})
			emitted[k] = true
		}
	}

	// Remaining fields (excluding summary, which goes last).
	for k, v := range metadata {
		if k == "summary" || emitted[k] || v == "" {
			continue
		}
		s = append(s, yaml.MapItem{Key: k, Value: coerceFrontmatterValue(v)})
		emitted[k] = true
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
// Provider-specific validation (e.g. parent requirements for sub-tasks) is
// handled by the provider API — recoverable errors surface in the edit loop.
func ValidateFrontmatter(fm map[string]string) string {
	if fm["summary"] == "" {
		return "Summary is required."
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
// used by the editor. Top-level fields are driven by FieldDefs rather than
// hardcoded field names.
func WorkItemToMetadata(item *WorkItem, defs FieldDefs) map[string]string {
	m := map[string]string{
		"key":     item.ID,
		"type":    item.Type,
		"status":  item.Status,
		"summary": item.Summary,
	}
	if item.ParentID != "" {
		m["parent"] = item.ParentID
	}
	for _, def := range defs {
		if !def.TopLevelField() || !def.IncludeInSchema() || def.Informational() {
			continue
		}
		if v := item.DisplayStringField(def.Key); v != "" {
			m[def.Key] = v
		}
	}
	return m
}

// coreKeys are frontmatter keys handled as first-class WorkItem fields,
// not routed into the Fields bag.
var coreKeys = map[string]bool{
	"key": true, "summary": true, "type": true, "status": true, "parent": true,
}

// IsCoreKey reports whether a frontmatter key is a first-class WorkItem field
// (summary, type, status, parent, key) rather than a provider-specific field.
func IsCoreKey(k string) bool {
	return coreKeys[k]
}

// FrontmatterToWorkItem builds a WorkItem from parsed frontmatter and
// a description AST. Used by the create flow. Non-core keys (anything not
// in coreKeys) are routed into the Fields map.
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
	for k, v := range fm {
		if coreKeys[k] || v == "" {
			continue
		}
		fields[k] = v
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
	for k, v := range fm {
		if coreKeys[k] || v == "" {
			continue
		}
		if v != origItem.StringField(k) {
			fields[k] = v
		}
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
