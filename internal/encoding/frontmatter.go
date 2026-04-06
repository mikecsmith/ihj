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

// BuildFrontmatterDoc assembles a YAML-frontmatter document for the editor.
// The item is encoded using the same pipeline as manifest items: prominent
// fields are top-level, others go in the "fields" bag. Description is
// omitted from the YAML and placed as the markdown body below the
// delimiters. Summary is positioned last (closest to the body) for
// editing convenience.
func BuildFrontmatterDoc(schemaPath string, item *core.WorkItem, defs []core.FieldDef, bodyText string) string {
	s := workItemToMap(item, defs, itemEncodeOpts{
		full:         false,
		summaryLast:  true,
		omitChildren: true,
		omitDesc:     true,
		topLevel:     true,
	})

	yamlBytes, _ := yaml.MarshalWithOptions(s, yaml.UseLiteralStyleIfMultiline(true))

	// Clean up null values for a friendlier editor experience.
	// e.g. `summary: null` becomes `summary: ` — YAML parses both as empty.
	yamlStr := strings.ReplaceAll(string(yamlBytes), ": null", ": ")

	var lines []string
	lines = append(lines, "---")
	lines = append(lines, fmt.Sprintf("# yaml-language-server: $schema=file://%s", schemaPath))
	lines = append(lines, strings.TrimSpace(yamlStr))
	lines = append(lines, "---", "", bodyText)
	return strings.Join(lines, "\n")
}

// ParseFrontmatter splits a YAML-frontmatter document into a WorkItem and
// the markdown body. The WorkItem is decoded using the same pipeline as
// manifest items, with Presence tracking which keys were explicitly set.
// The markdown body is parsed into the WorkItem's Description field, and
// "description" is always marked present in Presence when delimiters exist.
// Pass nil defs if only core fields (Summary, Type, Status) are needed.
func ParseFrontmatter(raw string, defs []core.FieldDef) (*core.WorkItem, string, error) {
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return &core.WorkItem{}, strings.TrimSpace(raw), nil
	}

	yamlStr := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		return nil, body, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	item := workItemFromMap(parsed, defs)

	// Description comes from the markdown body, not the YAML.
	if body != "" {
		item.Description, _ = document.ParseMarkdownString(body)
	}
	if item.Presence == nil {
		item.Presence = make(core.FieldPresence)
	}
	item.Presence[core.KeyDescription] = true

	return item, body, nil
}

// ValidateFrontmatter checks domain rules on a parsed frontmatter WorkItem.
// Returns an error message string, or "" if valid.
func ValidateFrontmatter(item *core.WorkItem) string {
	if item.Summary == "" {
		return "Summary is required."
	}
	return ""
}
