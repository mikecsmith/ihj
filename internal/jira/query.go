package jira

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mikecsmith/ihj/internal/config"
)

// BuildJQL constructs the final JQL query by interpolating the board's
// base JQL template with custom field references and board metadata,
// then AND-ing the active filter clause.
func BuildJQL(board *config.BoardConfig, filterName string, formattedCF map[string]string) (string, error) {
	baseJQL := strings.TrimSpace(board.JQL)
	if baseJQL == "" {
		return "", fmt.Errorf("board '%s' has no base JQL", board.Slug)
	}

	// Build the replacement map: custom fields + board metadata.
	vars := make(map[string]string)
	for k, v := range formattedCF {
		vars[k] = v
	}
	vars["project_key"] = board.ProjectKey
	vars["team_uuid"] = board.TeamUUID
	vars["id"] = fmt.Sprintf("%d", board.ID)
	vars["name"] = board.Name
	vars["slug"] = board.Slug

	// Interpolate base JQL.
	expandedBase, err := interpolateJQL(baseJQL, vars)
	if err != nil {
		return "", fmt.Errorf("expanding base JQL for '%s': %w", board.Slug, err)
	}

	// Get the filter clause.
	filterJQL := ""
	if filterName != "" {
		if f, ok := board.Filters[filterName]; ok {
			filterJQL = strings.TrimSpace(f)
		}
	}

	if filterJQL == "" {
		return expandedBase, nil
	}

	// Interpolate the filter clause too.
	expandedFilter, err := interpolateJQL(filterJQL, vars)
	if err != nil {
		return "", fmt.Errorf("expanding filter '%s' for '%s': %w", filterName, board.Slug, err)
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
