// Package storage handles all filesystem operations for ihj:
// configuration loading, schema writing, cache management, and state persistence.
package storage

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
)

// AppConfig holds the global application configuration.
// Workspace-specific settings live inside each core.Workspace.
type AppConfig struct {
	Editor           string
	DefaultWorkspace string
	Workspaces       map[string]*core.Workspace
}

// ResolveWorkspace returns the workspace for the given slug, falling back
// to DefaultWorkspace. Returns an error if neither is found.
func (c *AppConfig) ResolveWorkspace(slug string) (*core.Workspace, error) {
	if slug == "" {
		slug = c.DefaultWorkspace
	}
	if slug == "" {
		return nil, fmt.Errorf("no workspace specified and 'default_workspace' not set in config")
	}
	ws, ok := c.Workspaces[slug]
	if !ok {
		return nil, fmt.Errorf("workspace '%s' not found in config", slug)
	}
	return ws, nil
}

// ResolveFilter returns the effective filter name, falling back to "active".
func (c *AppConfig) ResolveFilter(name string) string {
	if name != "" {
		return name
	}
	return "active"
}

// EditorCommand returns the configured editor, falling back to $EDITOR then vim.
func (c *AppConfig) EditorCommand() string {
	if c.Editor != "" {
		return c.Editor
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "vim"
}

// --- YAML deserialization types (unexported) ---

// rawConfig is the top-level shape of config.yaml.
type rawConfig struct {
	Editor           string                  `yaml:"editor"`
	DefaultWorkspace string                  `yaml:"default_workspace"`
	Workspaces       map[string]rawWorkspace `yaml:"workspaces"`
}

// rawWorkspace is a single workspace entry before typed provider config extraction.
type rawWorkspace struct {
	Provider string            `yaml:"provider"`
	Name     string            `yaml:"name"`
	Types    []rawTypeConfig   `yaml:"types"`
	Statuses []string          `yaml:"statuses"`
	Filters  map[string]string `yaml:"filters"`
}

type rawTypeConfig struct {
	ID          int    `yaml:"id"`
	Name        string `yaml:"name"`
	Order       int    `yaml:"order"`
	Color       string `yaml:"color"`
	HasChildren bool   `yaml:"has_children"`
	Template    string `yaml:"template,omitempty"`
}

// --- Loading ---

// LoadConfig reads and parses the YAML config file, returning an AppConfig
// with workspaces populated. ProviderConfig on each workspace is set to
// map[string]any — the composition root hydrates typed provider configs.
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// First pass: parse into raw struct for universal fields.
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(raw.Workspaces) == 0 {
		return nil, fmt.Errorf("missing 'workspaces' in config")
	}

	// Second pass: parse each workspace block as map[string]any
	// to extract provider-specific fields.
	var fullConfig map[string]any
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return nil, fmt.Errorf("re-parsing config: %w", err)
	}

	workspacesRaw, _ := fullConfig["workspaces"].(map[string]any)

	cfg := &AppConfig{
		Editor:           raw.Editor,
		DefaultWorkspace: raw.DefaultWorkspace,
		Workspaces:       make(map[string]*core.Workspace, len(raw.Workspaces)),
	}

	// Universal fields that live on core.Workspace, not in provider config.
	universalKeys := map[string]bool{
		"provider": true, "name": true, "types": true, "statuses": true, "filters": true,
	}

	for slug, rws := range raw.Workspaces {
		if rws.Provider == "" {
			return nil, fmt.Errorf("workspace '%s' is missing 'provider' field", slug)
		}

		types := make([]core.TypeConfig, len(rws.Types))
		for i, t := range rws.Types {
			types[i] = core.TypeConfig{
				ID:          t.ID,
				Name:        t.Name,
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
				Template:    t.Template,
			}
		}

		typeOrderMap := make(map[string]core.TypeOrderEntry, len(types))
		for _, t := range types {
			typeOrderMap[t.Name] = core.TypeOrderEntry{
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
			}
		}

		statusWeights := make(map[string]int, len(rws.Statuses))
		for i, s := range rws.Statuses {
			statusWeights[strings.ToLower(s)] = i
		}

		// Extract provider-specific fields as map[string]any.
		providerCfg := make(map[string]any)
		if wsMap, ok := workspacesRaw[slug].(map[string]any); ok {
			for k, v := range wsMap {
				if !universalKeys[k] {
					providerCfg[k] = v
				}
			}
		}

		ws := &core.Workspace{
			Slug:           slug,
			Name:           rws.Name,
			Provider:       rws.Provider,
			Types:          types,
			Statuses:       rws.Statuses,
			Filters:        rws.Filters,
			StatusWeights:  statusWeights,
			TypeOrderMap:   typeOrderMap,
			ProviderConfig: providerCfg,
		}

		if err := validateWorkspace(slug, ws, providerCfg); err != nil {
			return nil, err
		}

		cfg.Workspaces[slug] = ws
	}

	return cfg, nil
}

// LoadConfigOrEmpty attempts to load the config, returning an empty AppConfig
// if the file doesn't exist. Used during bootstrap.
func LoadConfigOrEmpty(path string) (*AppConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &AppConfig{
			Workspaces: make(map[string]*core.Workspace),
		}, nil
	}
	return LoadConfig(path)
}

// validateWorkspace checks structural correctness of a workspace.
func validateWorkspace(slug string, ws *core.Workspace, providerCfg map[string]any) error {
	if len(ws.Types) == 0 {
		return fmt.Errorf("workspace '%s' is missing 'types' array", slug)
	}

	// Validate JQL templates for Jira workspaces.
	if ws.Provider == "jira" {
		jql, _ := providerCfg["jql"].(string)
		if strings.TrimSpace(jql) == "" {
			return fmt.Errorf("workspace '%s' (jira) is missing 'jql' field", slug)
		}

		customFields, _ := providerCfg["custom_fields"].(map[string]any)
		filters, _ := providerCfg["filters"].(map[string]any)

		if err := validateJQLTemplates(slug, jql, filters, customFields); err != nil {
			return err
		}
	}

	return nil
}

// validateJQLTemplates checks that all {var} references in JQL templates
// resolve to known fields.
func validateJQLTemplates(slug, baseJQL string, filters, customFields map[string]any) error {
	varPattern := regexp.MustCompile(`\{(\w+)\}`)

	available := make(map[string]bool)
	for k := range customFields {
		available[k] = true
	}
	// Workspace metadata available as JQL variables.
	metaKeys := map[string]bool{
		"id": true, "name": true, "project_key": true,
		"team_uuid": true, "slug": true,
	}

	templates := []string{baseJQL}
	for _, v := range filters {
		if s, ok := v.(string); ok {
			templates = append(templates, s)
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
					"JQL error in workspace '%s': '{%s}' is not defined in custom_fields or workspace metadata",
					slug, varName,
				)
			}
		}
	}

	return nil
}
