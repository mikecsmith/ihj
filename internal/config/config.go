// Package config manages the application configuration
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// Config is the top-level configuration structure parsed from
// ~/.config/ihj/config.yaml. Replaces the raw dict approach from Python.
type Config struct {
	Server        string                  `yaml:"server"`
	DefaultBoard  string                  `yaml:"default_board"`
	DefaultFilter string                  `yaml:"default_filter"`
	Editor        string                  `yaml:"editor"`
	CustomFields  map[string]int          `yaml:"custom_fields"`
	Boards        map[string]*BoardConfig `yaml:"boards"`

	// Computed at parse time, not stored in YAML.
	FormattedCustomFields map[string]string `yaml:"-"`
}

// BoardConfig describes a single Jira board's configuration.
type BoardConfig struct {
	ID          int               `yaml:"id"`
	Name        string            `yaml:"name"`
	ProjectKey  string            `yaml:"project_key"`
	TeamUUID    string            `yaml:"team_uuid,omitempty"`
	JQL         string            `yaml:"jql"`
	Filters     map[string]string `yaml:"filters"`
	Transitions []string          `yaml:"transitions"`
	Types       []IssueTypeConfig `yaml:"types"`

	// Computed at parse time.
	Slug         string                    `yaml:"-"`
	TypeOrderMap map[string]TypeOrderEntry `yaml:"-"`
}

// IssueTypeConfig describes an issue type within a board.
type IssueTypeConfig struct {
	ID          int    `yaml:"id"`
	Name        string `yaml:"name"`
	Order       int    `yaml:"order"`
	Color       string `yaml:"color"`
	HasChildren bool   `yaml:"has_children"`
	Template    string `yaml:"template,omitempty"`
}

// TypeOrderEntry is the computed rendering metadata for an issue type.
type TypeOrderEntry struct {
	Order       int
	Color       string
	HasChildren bool
}

// Paths returns the standard filesystem paths for config and cache.
type Paths struct {
	ConfigDir  string
	ConfigFile string
	CacheDir   string
}

// DefaultPaths returns XDG-compliant paths for ihj.
func DefaultPaths() Paths {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "ihj")
	cacheDir := filepath.Join(home, ".local", "state", "ihj")
	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.yaml"),
		CacheDir:   cacheDir,
	}
}

// EnsureDirs creates the config and cache directories if they don't exist.
func (p Paths) EnsureDirs() error {
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	return nil
}

// Load reads and parses the YAML config file. Returns the raw Config
// with computed fields populated. Replaces load_raw_config_file +
// validate_config + parse_config from Python.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.computeDerivedFields()
	return &cfg, nil
}

// LoadOrEmpty attempts to load the config, returning an empty Config
// if the file doesn't exist. Used during bootstrap.
func LoadOrEmpty(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{
			Boards:       make(map[string]*BoardConfig),
			CustomFields: make(map[string]int),
		}, nil
	}
	return Load(path)
}

// validate checks structural correctness of the config. Replaces
// config/processor.py validate_config.
func (c *Config) validate() error {
	if c.CustomFields == nil {
		return fmt.Errorf("missing 'custom_fields' in config")
	}
	if len(c.Boards) == 0 {
		return fmt.Errorf("missing 'boards' in config")
	}

	availableKeys := make(map[string]bool)
	for k := range c.CustomFields {
		availableKeys[k] = true
	}
	boardMetaKeys := map[string]bool{
		"id": true, "name": true, "project_key": true,
		"team_uuid": true, "slug": true,
	}

	varPattern := regexp.MustCompile(`\{(\w+)\}`)

	for slug, board := range c.Boards {
		if len(board.Types) == 0 {
			return fmt.Errorf("board '%s' is missing 'types' array", slug)
		}
		if strings.TrimSpace(board.JQL) == "" {
			return fmt.Errorf("board '%s' is missing base 'jql' string", slug)
		}

		// Check all JQL templates for undefined variables.
		templates := []string{board.JQL}
		for _, f := range board.Filters {
			templates = append(templates, f)
		}

		for _, tmpl := range templates {
			if strings.TrimSpace(tmpl) == "" {
				continue
			}
			matches := varPattern.FindAllStringSubmatch(tmpl, -1)
			for _, m := range matches {
				varName := m[1]
				if !availableKeys[varName] && !boardMetaKeys[varName] {
					return fmt.Errorf(
						"JQL error in board '%s': '{%s}' is not defined in custom_fields or board metadata",
						slug, varName,
					)
				}
			}
		}
	}

	return nil
}

// computeDerivedFields populates the computed fields that the Python
// version built in parse_config.
func (c *Config) computeDerivedFields() {
	// Build formatted custom field map (e.g. "team" → "cf[15000]").
	c.FormattedCustomFields = make(map[string]string)
	for key, val := range c.CustomFields {
		c.FormattedCustomFields[key] = fmt.Sprintf("cf[%d]", val)
		c.FormattedCustomFields[key+"_id"] = fmt.Sprintf("customfield_%d", val)
	}

	// Build type order maps per board.
	for slug, board := range c.Boards {
		board.Slug = slug
		board.TypeOrderMap = make(map[string]TypeOrderEntry)
		for _, t := range board.Types {
			board.TypeOrderMap[fmt.Sprintf("%d", t.ID)] = TypeOrderEntry{
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
			}
		}
	}
}

// ResolveBoard returns the board config for the given slug, falling back
// to DefaultBoard. Returns an error if neither is found.
func (c *Config) ResolveBoard(slug string) (*BoardConfig, error) {
	if slug == "" {
		slug = c.DefaultBoard
	}
	if slug == "" {
		return nil, fmt.Errorf("no board specified and 'default_board' not set in config")
	}
	board, ok := c.Boards[slug]
	if !ok {
		return nil, fmt.Errorf("board '%s' not found in config", slug)
	}
	return board, nil
}

// ResolveFilter returns the effective filter name, falling back to
// DefaultFilter then "active".
func (c *Config) ResolveFilter(name string) string {
	if name != "" {
		return name
	}
	if c.DefaultFilter != "" {
		return c.DefaultFilter
	}
	return "active"
}

// EditorCommand returns the configured editor, falling back to $EDITOR then vim.
func (c *Config) EditorCommand() string {
	if c.Editor != "" {
		return c.Editor
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "vim"
}
