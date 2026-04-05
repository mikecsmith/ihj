package encoding

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Frontmatter is the schema name used for caching.
const Frontmatter = "frontmatter"

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

	yamlBytes, _ := yaml.MarshalWithOptions(s, yaml.UseLiteralStyleIfMultiline(true))

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
// hardcoded field names. RichText custom fields are rendered to markdown
// strings so they can be edited inline (as YAML block literals).
func WorkItemToMetadata(item *core.WorkItem, defs core.FieldDefs) map[string]string {
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
		if !def.Authored() {
			continue
		}
		if def.Type == core.FieldRichText {
			if node := item.RichTextField(def.Key); node != nil {
				if s := strings.TrimSpace(document.RenderMarkdown(node)); s != "" {
					m[def.Key] = s
				}
			}
			continue
		}
		if v := item.DisplayStringField(def.Key); v != "" {
			m[def.Key] = v
		}
	}
	return m
}

// richTextKeys returns the set of keys for FieldRichText defs.
func richTextKeys(defs core.FieldDefs) map[string]bool {
	out := make(map[string]bool)
	for _, def := range defs {
		if def.Type == core.FieldRichText {
			out[def.Key] = true
		}
	}
	return out
}

// FrontmatterToWorkItem builds a WorkItem from parsed frontmatter and
// a description AST. Used by the create flow. Non-reserved keys are routed
// into the Fields map; RichText-typed keys are parsed from markdown into
// *document.Node values.
func FrontmatterToWorkItem(fm map[string]string, description *document.Node, defs core.FieldDefs) *core.WorkItem {
	item := &core.WorkItem{
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
	richKeys := richTextKeys(defs)
	fields := make(map[string]any)
	for k, v := range fm {
		if core.IsReservedKey(k) || v == "" {
			continue
		}
		if richKeys[k] {
			if node, err := document.ParseMarkdownString(v); err == nil {
				fields[k] = node
			}
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
// RichText-typed fields are compared via rendered markdown so AST-level
// formatting differences do not produce spurious diffs.
func FrontmatterToChanges(fm map[string]string, description *document.Node, origItem *core.WorkItem, defs core.FieldDefs) *core.Changes {
	changes := &core.Changes{}
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

	richKeys := richTextKeys(defs)
	fields := make(map[string]any)
	for k, v := range fm {
		if core.IsReservedKey(k) || v == "" {
			continue
		}
		if richKeys[k] {
			origNode := origItem.RichTextField(k)
			origMD := ""
			if origNode != nil {
				origMD = strings.TrimSpace(document.RenderMarkdown(origNode))
			}
			if strings.TrimSpace(v) != origMD {
				if node, err := document.ParseMarkdownString(v); err == nil {
					fields[k] = node
				}
			}
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
