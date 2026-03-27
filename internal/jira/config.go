package jira

import (
	"fmt"

	"github.com/mikecsmith/ihj/internal/core"
)

// Config holds Jira-specific workspace configuration.
// Populated by ParseConfig from the raw provider config map.
type Config struct {
	Server       string
	BoardID      int
	ProjectKey   string
	TeamUUID     string
	JQL          string
	CustomFields map[string]int

	// FormattedCustomFields is computed from CustomFields.
	// Maps "team" → "cf[15000]" and "team_id" → "customfield_15000".
	FormattedCustomFields map[string]string
}

// ParseConfig extracts a typed Config from a workspace's raw ProviderConfig.
// Called by the composition root after storage.LoadConfig.
func ParseConfig(ws *core.Workspace) (*Config, error) {
	raw, ok := ws.ProviderConfig.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workspace '%s': expected map[string]any provider config, got %T", ws.Slug, ws.ProviderConfig)
	}

	cfg := &Config{
		Server:     stringVal(raw, "server"),
		ProjectKey: stringVal(raw, "project_key"),
		TeamUUID:   stringVal(raw, "team_uuid"),
		JQL:        stringVal(raw, "jql"),
		BoardID:    intVal(raw, "board_id"),
	}

	// Parse custom_fields map.
	if cfRaw, ok := raw["custom_fields"].(map[string]any); ok {
		cfg.CustomFields = make(map[string]int, len(cfRaw))
		for k, v := range cfRaw {
			switch n := v.(type) {
			case int:
				cfg.CustomFields[k] = n
			case int64:
				cfg.CustomFields[k] = int(n)
			case uint64:
				cfg.CustomFields[k] = int(n)
			case float64:
				cfg.CustomFields[k] = int(n)
			}
		}
	}

	// Compute formatted custom fields.
	cfg.FormattedCustomFields = make(map[string]string)
	for key, val := range cfg.CustomFields {
		cfg.FormattedCustomFields[key] = fmt.Sprintf("cf[%d]", val)
		cfg.FormattedCustomFields[key+"_id"] = fmt.Sprintf("customfield_%d", val)
	}

	return cfg, nil
}

// HydrateWorkspace parses the raw provider config and sets the typed
// Config back on the workspace. Convenience for the composition root.
func HydrateWorkspace(ws *core.Workspace) (*Config, error) {
	cfg, err := ParseConfig(ws)
	if err != nil {
		return nil, err
	}
	ws.ProviderConfig = cfg
	return cfg, nil
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
