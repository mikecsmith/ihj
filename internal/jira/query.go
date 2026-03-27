package jira

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
)

// BuildJQL constructs the final JQL query by interpolating the workspace's
// base JQL template with custom field references and workspace metadata,
// then AND-ing the active filter clause.
func BuildJQL(ws *core.Workspace, cfg *Config, filterName string) (string, error) {
	baseJQL := strings.TrimSpace(cfg.JQL)
	if baseJQL == "" {
		return "", fmt.Errorf("workspace '%s' has no base JQL", ws.Slug)
	}

	// Build the replacement map: custom fields + workspace metadata.
	vars := make(map[string]string)
	for k, v := range cfg.FormattedCustomFields {
		vars[k] = v
	}
	vars["project_key"] = cfg.ProjectKey
	vars["team_uuid"] = cfg.TeamUUID
	vars["id"] = fmt.Sprintf("%d", cfg.BoardID)
	vars["name"] = ws.Name
	vars["slug"] = ws.Slug

	// Interpolate base JQL.
	expandedBase, err := interpolateJQL(baseJQL, vars)
	if err != nil {
		return "", fmt.Errorf("expanding base JQL for '%s': %w", ws.Slug, err)
	}

	// Get the filter clause from workspace filters.
	filterJQL := ""
	if filterName != "" {
		if f, ok := ws.Filters[filterName]; ok {
			filterJQL = strings.TrimSpace(f)
		}
	}

	if filterJQL == "" {
		return expandedBase, nil
	}

	// Interpolate the filter clause too.
	expandedFilter, err := interpolateJQL(filterJQL, vars)
	if err != nil {
		return "", fmt.Errorf("expanding filter '%s' for '%s': %w", filterName, ws.Slug, err)
	}

	// Inject filter before any ORDER BY clause.
	return combineJQL(expandedBase, expandedFilter), nil
}

// interpolateJQL replaces {varName} placeholders in a JQL template.
// Returns an error if a placeholder references an undefined variable.
func interpolateJQL(template string, vars map[string]string) (string, error) {
	pattern := regexp.MustCompile(`\{(\w+)\}`)

	var missingVars []string
	result := pattern.ReplaceAllStringFunc(template, func(match string) string {
		key := match[1 : len(match)-1] // Strip { and }
		if val, ok := vars[key]; ok {
			return val
		}
		missingVars = append(missingVars, key)
		return match // Leave placeholder in place for the error message.
	})

	if len(missingVars) > 0 {
		return "", fmt.Errorf("undefined JQL variables: %s", strings.Join(missingVars, ", "))
	}

	return result, nil
}

// combineJQL appends a filter clause to base JQL, respecting ORDER BY.
func combineJQL(base, filter string) string {
	orderByPattern := regexp.MustCompile(`(?i)\s+ORDER\s+BY\s+`)

	parts := orderByPattern.Split(base, 2)
	if len(parts) > 1 {
		queryPart := parts[0]
		// Find the original ORDER BY text to preserve casing.
		loc := orderByPattern.FindStringIndex(base)
		orderPart := base[loc[0]:]
		return fmt.Sprintf("(%s) AND (%s)%s", queryPart, filter, orderPart)
	}

	return fmt.Sprintf("(%s) AND (%s)", base, filter)
}
