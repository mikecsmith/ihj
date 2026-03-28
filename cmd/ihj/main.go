package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/goccy/go-yaml"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/demo"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/tui"
)

func main() {
	if err := run(os.Stdout, os.Stderr); err != nil {
		if commands.IsCancelled(err) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(stdout, stderr io.Writer) error {
	configDir, configFile, cacheDir := defaultPaths()
	if err := ensureDirs(configDir, cacheDir); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	btUI := tui.NewBubbleTeaUI()

	// initSession loads config, creates a provider, and attaches
	// the Session to the cobra context. Called by PersistentPreRunE.
	initSession := func(ctx context.Context, mode sessionMode) (context.Context, error) {
		var (
			theme            string
			editor           string
			defaultWorkspace string
			workspaces       map[string]*core.Workspace
		)

		switch mode {
		case modeDemo:
			ws := demo.Workspace()
			defaultWorkspace = ws.Slug
			workspaces = map[string]*core.Workspace{ws.Slug: ws}

		case modeBootstrap:
			var err error
			theme, editor, defaultWorkspace, workspaces, err = loadConfigOrEmpty(configFile)
			if err != nil {
				return ctx, fmt.Errorf("config: %w", err)
			}

		default:
			var err error
			theme, editor, defaultWorkspace, workspaces, err = loadConfig(configFile)
			if err != nil {
				if os.IsNotExist(err) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", configFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range workspaces {
				switch ws.Provider {
				case core.ProviderJira:
					jiraCfg, err := jira.HydrateWorkspace(ws)
					if err != nil {
						return ctx, fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
					}
					ws.BaseURL = jiraCfg.Server
				}
			}
		}

		btUI.EditorCmd = editorCommand(editor)

		provider, client, err := newProvider(defaultWorkspace, workspaces, cacheDir)
		if err != nil {
			return ctx, err
		}

		s := &commands.Session{
			Theme:            theme,
			DefaultWorkspace: defaultWorkspace,
			Workspaces:       workspaces,
			Provider:         provider,
			UI:               btUI,
			CacheDir:         cacheDir,
			Out:              stdout,
			Err:              stderr,
			LaunchTUI: func(data *commands.LaunchTUIData) error {
				model := tui.NewAppModel(data.Session, data.Workspace, data.Filter, data.Items, data.FetchedAt)
				p := tea.NewProgram(model)
				btUI.SetProgram(p)
				_, err := p.Run()
				return err
			},
		}

		ctx = contextWithSession(ctx, s)
		if client != nil {
			ctx = contextWithJiraClient(ctx, client)
		}
		return ctx, nil
	}

	root := newRootCmd(initSession)
	return root.ExecuteContext(context.Background())
}

type sessionMode int

const (
	modeNormal    sessionMode = iota
	modeDemo                  // skip config loading, use synthetic data
	modeBootstrap             // allow missing/empty config
)

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

// --- Filesystem paths ---

// defaultPaths returns XDG-compliant paths for ihj config and cache.
func defaultPaths() (configDir, configFile, cacheDir string) {
	home, _ := os.UserHomeDir()
	configDir = filepath.Join(home, ".config", "ihj")
	configFile = filepath.Join(configDir, "config.yaml")
	cacheDir = filepath.Join(home, ".local", "state", "ihj")
	return
}

func ensureDirs(dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}

// --- Config loading ---

// YAML deserialization types.
type rawConfig struct {
	Theme            string                  `yaml:"theme"`
	Editor           string                  `yaml:"editor"`
	DefaultWorkspace string                  `yaml:"default_workspace"`
	Workspaces       map[string]rawWorkspace `yaml:"workspaces"`
}

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

// loadConfig reads and parses the YAML config file. ProviderConfig on each
// workspace is set to map[string]any — the composition root hydrates typed
// provider configs via provider-specific functions (e.g., jira.HydrateWorkspace).
func loadConfig(path string) (theme, editor, defaultWorkspace string, workspaces map[string]*core.Workspace, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("reading config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", "", "", nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(raw.Workspaces) == 0 {
		return "", "", "", nil, fmt.Errorf("missing 'workspaces' in config")
	}

	// Second pass: parse each workspace block as map[string]any
	// to extract provider-specific fields.
	var fullConfig map[string]any
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return "", "", "", nil, fmt.Errorf("re-parsing config: %w", err)
	}

	workspacesRaw, _ := fullConfig["workspaces"].(map[string]any)

	universalKeys := map[string]bool{
		"provider": true, "name": true, "types": true, "statuses": true, "filters": true,
	}

	workspaces = make(map[string]*core.Workspace, len(raw.Workspaces))

	for slug, rws := range raw.Workspaces {
		if rws.Provider == "" {
			return "", "", "", nil, fmt.Errorf("workspace '%s' is missing 'provider' field", slug)
		}

		if len(rws.Types) == 0 {
			return "", "", "", nil, fmt.Errorf("workspace '%s' is missing 'types' array", slug)
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
			Provider:       rws.Provider,
			Types:          types,
			Statuses:       rws.Statuses,
			Filters:        rws.Filters,
			StatusWeights:  statusWeights,
			TypeOrderMap:   typeOrderMap,
			ProviderConfig: providerCfg,
		}
	}

	return raw.Theme, raw.Editor, raw.DefaultWorkspace, workspaces, nil
}

// loadConfigOrEmpty attempts to load the config, returning empty values
// if the file doesn't exist. Used during bootstrap.
func loadConfigOrEmpty(path string) (theme, editor, defaultWorkspace string, workspaces map[string]*core.Workspace, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", "", "", make(map[string]*core.Workspace), nil
	}
	return loadConfig(path)
}

// --- Provider creation ---

// newProvider creates a core.Provider and optionally a jira.API client for the
// default workspace. The client is only needed for bootstrap.
func newProvider(defaultWorkspace string, workspaces map[string]*core.Workspace, cacheDir string) (core.Provider, jira.API, error) {
	if defaultWorkspace == "" {
		// No default workspace configured — not an error for bootstrap.
		return nil, nil, nil
	}
	ws, ok := workspaces[defaultWorkspace]
	if !ok {
		return nil, nil, nil
	}

	switch ws.Provider {
	case core.ProviderJira:
		token := os.Getenv("JIRA_BASIC_TOKEN")
		if token == "" {
			return nil, nil, fmt.Errorf("JIRA_BASIC_TOKEN environment variable not set.\nSet it to base64(email:api_token) for Jira Cloud")
		}
		jiraCfg, _ := ws.ProviderConfig.(*jira.Config)
		client := jira.New(
			jiraCfg.Server,
			token,
			jira.WithContext(context.Background()),
		)
		provider := jira.NewProvider(client, ws, cacheDir)
		return provider, client, nil

	case core.ProviderDemo:
		items := demo.Issues()
		provider := demo.NewProvider(items, 150*time.Millisecond)
		return provider, nil, nil

	default:
		return nil, nil, fmt.Errorf("unsupported provider %q for workspace %q", ws.Provider, ws.Slug)
	}
}
