package jira

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
)

// Config holds Jira-specific workspace configuration.
// Populated by ParseConfig from the raw provider config map.
type Config struct {
	Server     string
	BoardID    int
	BoardType  string // "scrum", "kanban", or "simple"
	ProjectKey string
	TeamUUID   string
	JQL        string

	// FormattedFields is computed from ws.FieldAliases at parse time.
	// Maps "team" → "cf[15000]" and "team_id" → "customfield_15000".
	FormattedFields map[string]string
}

// ParseConfig extracts a typed Config from a workspace's raw ProviderConfig.
// Called by the composition root after config parsing.
func ParseConfig(ws *core.Workspace) (*Config, error) {
	raw, ok := ws.ProviderConfig.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workspace '%s': expected map[string]any provider config, got %T", ws.Slug, ws.ProviderConfig)
	}

	cfg := &Config{
		Server:     ws.BaseURL,
		ProjectKey: stringVal(raw, "project_key"),
		TeamUUID:   stringVal(raw, "team_uuid"),
		JQL:        stringVal(raw, "jql"),
		BoardID:    intVal(raw, "board_id"),
		BoardType:  stringVal(raw, "board_type"),
	}

	// Compute formatted field references from workspace-level aliases.
	cfg.FormattedFields = make(map[string]string)
	for key, val := range ws.FieldAliases {
		cfg.FormattedFields[key] = fmt.Sprintf("cf[%d]", val)
		cfg.FormattedFields[key+"_id"] = fmt.Sprintf("customfield_%d", val)
	}

	return cfg, nil
}

// HydrateWorkspace parses the raw provider config, validates JQL templates,
// and sets the typed Config back on the workspace.
func HydrateWorkspace(ws *core.Workspace) (*Config, error) {
	cfg, err := ParseConfig(ws)
	if err != nil {
		return nil, err
	}
	if err := cfg.validateJQL(ws, ws.Filters); err != nil {
		return nil, err
	}
	ws.ProviderConfig = cfg
	return cfg, nil
}

// validateJQL checks that all {var} references in JQL templates resolve to
// known field aliases or workspace metadata keys.
func (c *Config) validateJQL(ws *core.Workspace, filters map[string]string) error {
	if strings.TrimSpace(c.JQL) == "" {
		return fmt.Errorf("workspace '%s' (jira) is missing 'jql' field", ws.Slug)
	}

	varPattern := regexp.MustCompile(`\{(\w+)\}`)

	available := make(map[string]bool, len(ws.FieldAliases)*2)
	for k := range ws.FieldAliases {
		available[k] = true
		available[k+"_id"] = true
	}
	metaKeys := map[string]bool{
		"id": true, "name": true, "project_key": true,
		"team_uuid": true, "slug": true,
	}

	templates := []string{c.JQL}
	for _, v := range filters {
		if v != "" {
			templates = append(templates, v)
		}
	}

	for _, tmpl := range templates {
		if strings.TrimSpace(tmpl) == "" {
			continue
		}
		matches := varPattern.FindAllStringSubmatch(tmpl, -1)
		for _, m := range matches {
			varName := m[1]
			if !available[varName] && !metaKeys[varName] {
				return fmt.Errorf(
					"JQL error in workspace '%s': '{%s}' is not defined in fields or workspace metadata",
					ws.Slug, varName,
				)
			}
		}
	}
	return nil
}

func stringVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func intVal(m map[string]any, key string) int {
	switch n := m[key].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
