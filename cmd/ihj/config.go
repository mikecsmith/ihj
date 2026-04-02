package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// YAML deserialization types.
type rawConfig struct {
	Theme            string                  `yaml:"theme"`
	Editor           string                  `yaml:"editor"`
	VimMode          bool                    `yaml:"vim_mode"`
	DefaultWorkspace string                  `yaml:"default_workspace"`
	CacheTTL         string                  `yaml:"cache_ttl"`
	Guidance         string                  `yaml:"guidance"`
	Shortcuts        map[string]string       `yaml:"shortcuts"`
	Layout           rawLayout               `yaml:"layout"`
	Servers          map[string]rawServer    `yaml:"servers"`
	Workspaces       map[string]rawWorkspace `yaml:"workspaces"`
}

type rawLayout struct {
	DetailHeight int   `yaml:"detail_height"`
	ShowHelpBar  *bool `yaml:"show_help_bar"` // Pointer to distinguish unset from false.
}

// configResult bundles the parsed configuration values returned by loadConfig.
type configResult struct {
	Theme            string
	Editor           string
	VimMode          bool
	DefaultWorkspace string
	Shortcuts        map[string]string
	DetailPct        int  // Detail pane height percentage (0 = default 55).
	ShowHelpBar      bool // Show the help bar (default true).
	Servers          map[string]rawServer
	Workspaces       map[string]*core.Workspace
}

type rawServer struct {
	Provider string `yaml:"provider"` // e.g., "jira", "github"
	URL      string `yaml:"url"`
}

type rawWorkspace struct {
	Server   string            `yaml:"server"` // Server alias (references servers map)
	Name     string            `yaml:"name"`
	CacheTTL string            `yaml:"cache_ttl"`
	Guidance string            `yaml:"guidance"`
	Types    []rawTypeConfig   `yaml:"types"`
	Statuses []rawStatusConfig `yaml:"statuses"`
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

type rawStatusConfig struct {
	Name  string `yaml:"name"`
	Order int    `yaml:"order"`
	Color string `yaml:"color"`
}

// uiCaps holds UI-implementation settings resolved from config.
// The composition root populates this after parsing config, then the caller
// applies the values to the concrete UI implementations it owns.
type uiCaps struct {
	EditorCmd   string
	VimMode     bool
	Shortcuts   map[string]string // Action name → key string overrides (default mode only).
	DetailPct   int               // Detail pane height percentage (20-80, default 55).
	ShowHelpBar bool              // Show the help bar (default true).
}

// editorCommand returns the configured editor, falling back to $EDITOR then vim.
func editorCommand(configured string) string {
	if configured != "" {
		return configured
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "vim"
}

// loadConfig reads and parses the YAML config file. ProviderConfig on each
// workspace is set to map[string]any — the composition root hydrates typed
// provider configs via provider-specific functions (e.g., jira.HydrateWorkspace).
func loadConfig(path string) (configResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configResult{}, fmt.Errorf("reading config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return configResult{}, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(raw.Workspaces) == 0 {
		return configResult{}, fmt.Errorf("missing 'workspaces' in config")
	}

	if len(raw.Servers) == 0 {
		return configResult{}, fmt.Errorf("missing 'servers' in config — define your servers under the top-level 'servers:' key")
	}

	// Validate server definitions.
	for alias, srv := range raw.Servers {
		if srv.Provider == "" {
			return configResult{}, fmt.Errorf("server '%s' is missing 'provider' field", alias)
		}
		if srv.URL == "" {
			return configResult{}, fmt.Errorf("server '%s' is missing 'url' field", alias)
		}
	}

	// Second pass: parse each workspace block as map[string]any
	// to extract provider-specific fields.
	var fullConfig map[string]any
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return configResult{}, fmt.Errorf("re-parsing config: %w", err)
	}

	workspacesRaw, _ := fullConfig["workspaces"].(map[string]any)

	universalKeys := map[string]bool{
		"server": true, "name": true, "types": true, "statuses": true, "filters": true,
		"cache_ttl": true, "guidance": true,
	}

	// Parse global cache TTL (falls back to core.DefaultCacheTTL).
	globalCacheTTL := core.DefaultCacheTTL
	if raw.CacheTTL != "" {
		d, err := time.ParseDuration(raw.CacheTTL)
		if err != nil {
			return configResult{}, fmt.Errorf("invalid global cache_ttl %q: %w", raw.CacheTTL, err)
		}
		globalCacheTTL = d
	}

	workspaces := make(map[string]*core.Workspace, len(raw.Workspaces))

	for slug, rws := range raw.Workspaces {
		if rws.Server == "" {
			return configResult{}, fmt.Errorf("workspace '%s' is missing 'server' field", slug)
		}

		srv, ok := raw.Servers[rws.Server]
		if !ok {
			return configResult{}, fmt.Errorf("workspace '%s' references unknown server '%s'", slug, rws.Server)
		}

		if len(rws.Types) == 0 {
			return configResult{}, fmt.Errorf("workspace '%s' is missing 'types' array", slug)
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
			typeOrderMap[strings.ToLower(t.Name)] = core.TypeOrderEntry{
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
			}
		}

		statuses := make([]core.StatusConfig, len(rws.Statuses))
		statusOrderMap := make(map[string]core.StatusOrderEntry, len(rws.Statuses))
		for i, s := range rws.Statuses {
			statuses[i] = core.StatusConfig{Name: s.Name, Order: s.Order, Color: s.Color}
			statusOrderMap[strings.ToLower(s.Name)] = core.StatusOrderEntry{
				Weight: s.Order,
				Color:  s.Color,
			}
		}

		// Resolve cache TTL: workspace > global > default.
		cacheTTL := globalCacheTTL
		if rws.CacheTTL != "" {
			d, err := time.ParseDuration(rws.CacheTTL)
			if err != nil {
				return configResult{}, fmt.Errorf("workspace '%s': invalid cache_ttl %q: %w", slug, rws.CacheTTL, err)
			}
			cacheTTL = d
		}

		// Resolve guidance: workspace > global > empty (extract uses defaults).
		guidance := raw.Guidance
		if rws.Guidance != "" {
			guidance = rws.Guidance
		}

		providerCfg := make(map[string]any)
		if wsMap, ok := workspacesRaw[slug].(map[string]any); ok {
			for k, v := range wsMap {
				if !universalKeys[k] {
					providerCfg[k] = v
				}
			}
		}

		workspaces[slug] = &core.Workspace{
			Slug:           slug,
			Name:           rws.Name,
			Provider:       srv.Provider,
			ServerAlias:    rws.Server,
			BaseURL:        srv.URL,
			CacheTTL:       cacheTTL,
			Guidance:       guidance,
			Types:          types,
			Statuses:       statuses,
			Filters:        rws.Filters,
			StatusOrderMap: statusOrderMap,
			TypeOrderMap:   typeOrderMap,
			ProviderConfig: providerCfg,
		}
	}

	// Validate shortcuts against the base keymap (default mode only).
	if len(raw.Shortcuts) > 0 && !raw.VimMode {
		km := terminal.DefaultKeyMap()
		if err := km.ApplyShortcuts(raw.Shortcuts); err != nil {
			return configResult{}, fmt.Errorf("config: %w", err)
		}
	}

	// Validate layout.
	detailPct := raw.Layout.DetailHeight
	if detailPct != 0 {
		if detailPct < 20 || detailPct > 80 {
			return configResult{}, fmt.Errorf("config: layout.detail_height must be between 20 and 80, got %d", detailPct)
		}
	}
	showHelpBar := true
	if raw.Layout.ShowHelpBar != nil {
		showHelpBar = *raw.Layout.ShowHelpBar
	}

	return configResult{
		Theme:            raw.Theme,
		Editor:           raw.Editor,
		VimMode:          raw.VimMode,
		DefaultWorkspace: raw.DefaultWorkspace,
		Shortcuts:        raw.Shortcuts,
		DetailPct:        detailPct,
		ShowHelpBar:      showHelpBar,
		Servers:          raw.Servers,
		Workspaces:       workspaces,
	}, nil
}

// loadConfigOrEmpty attempts to load the config, returning empty values
// if the file doesn't exist. Used during bootstrap.
func loadConfigOrEmpty(path string) (configResult, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return configResult{Workspaces: make(map[string]*core.Workspace)}, nil
	}
	return loadConfig(path)
}
