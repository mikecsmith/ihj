// Command ihj is a provider-agnostic work-tracking CLI and TUI.
//
// It connects to issue trackers (currently Jira) and presents their
// data through a keyboard-driven terminal interface. See the internal
// packages for the domain model (core), business logic (commands),
// terminal UI (tui), and provider implementations (jira, demo).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/goccy/go-yaml"

	"github.com/mikecsmith/ihj/internal/auth"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/demo"
	"github.com/mikecsmith/ihj/internal/headless"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/tui"
)

func main() {
	configDir, configFile, cacheDir := defaultPaths()

	cliUI := headless.NewHeadlessUI()
	tuiUI := tui.NewBubbleTeaUI()

	err := run(os.Stdout, os.Stderr, configDir, configFile, cacheDir, cliUI, &tuiLauncher{ui: tuiUI}, func(cmd string) {
		cliUI.EditorCmd = cmd
		tuiUI.EditorCmd = cmd
	})
	if err != nil {
		if commands.IsCancelled(err) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run wires up the application. All external dependencies are injected by main(),
// making the function testable with stubs for the UI, launcher, and config paths.
func run(stdout, stderr io.Writer, configDir, configFile, cacheDir string, cliUI commands.UI, launcher commands.UILauncher, setEditorCmd func(string)) error {
	if err := ensureDirs(configDir, cacheDir); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	// Build credential store: keychain (if available) → env vars → file.
	creds := newCredentialStore(configDir)

	// initSession loads config, creates a Runtime + factory, and attaches
	// them to the cobra context. Called by PersistentPreRunE.
	initSession := func(ctx context.Context, mode sessionMode) (context.Context, error) {
		var (
			theme            string
			editor           string
			defaultWorkspace string
			servers          map[string]rawServer
			workspaces       map[string]*core.Workspace
		)

		switch mode {
		case modeDemo:
			ws := demo.Workspace()
			defaultWorkspace = ws.Slug
			workspaces = map[string]*core.Workspace{ws.Slug: ws}

		case modeBootstrap:
			var err error
			theme, editor, defaultWorkspace, servers, workspaces, err = loadConfigOrEmpty(configFile)
			if err != nil {
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range workspaces {
				if err := hydrateWorkspace(ws); err != nil {
					return ctx, err
				}
			}

		case modeAuth:
			var err error
			theme, editor, defaultWorkspace, servers, workspaces, err = loadConfig(configFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", configFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}
			// Auth mode: skip hydration and session creation.

		default:
			var err error
			theme, editor, defaultWorkspace, servers, workspaces, err = loadConfig(configFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", configFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range workspaces {
				if err := hydrateWorkspace(ws); err != nil {
					return ctx, err
				}
			}
		}

		editorCmd := editorCommand(editor)
		if setEditorCmd != nil {
			setEditorCmd(editorCmd)
		}

		rt := &commands.Runtime{
			Theme:            theme,
			DefaultWorkspace: defaultWorkspace,
			Workspaces:       workspaces,
			UI:               cliUI,
			CacheDir:         cacheDir,
			Out:              stdout,
			Err:              stderr,
			Launcher:         launcher,
		}

		factory := func(slug string) (*commands.WorkspaceSession, error) {
			ws, err := rt.ResolveWorkspace(slug)
			if err != nil {
				return nil, err
			}
			provider, client, err := newProviderForWorkspace(ws, cacheDir, creds)
			if err != nil {
				return nil, err
			}
			if client != nil {
				ctx = contextWithJiraClient(ctx, client)
			}
			return &commands.WorkspaceSession{
				Runtime:   rt,
				Workspace: ws,
				Provider:  provider,
			}, nil
		}

		ctx = contextWithRuntime(ctx, rt)
		ctx = contextWithFactory(ctx, factory)
		ctx = contextWithCredStore(ctx, creds)
		ctx = contextWithServers(ctx, servers)

		// Pre-create session for default workspace to detect auth errors early.
		// Skip for auth mode — we don't need provider connections.
		if mode != modeAuth && defaultWorkspace != "" {
			if _, ok := workspaces[defaultWorkspace]; ok {
				wsSess, err := factory(defaultWorkspace)
				if err != nil {
					return ctx, err
				}
				ctx = contextWithDefaultSession(ctx, wsSess)
			}
		}

		return ctx, nil
	}

	root := newRootCmd(initSession)
	return root.ExecuteContext(context.Background())
}

// tuiLauncher implements commands.UILauncher using Bubble Tea.
type tuiLauncher struct {
	ui *tui.BubbleTeaUI
}

func (l *tuiLauncher) LaunchUI(data *commands.LaunchUIData) error {
	// Swap runtime.UI to the TUI implementation for the duration of the TUI
	// session. The TUI's BubbleTeaUI bridges commands.UI calls to the Bubble
	// Tea event loop via channels.
	origUI := data.Runtime.UI
	data.Runtime.UI = l.ui
	defer func() { data.Runtime.UI = origUI }()

	model := tui.NewAppModel(data.Ctx, data.Runtime, data.Session, data.Factory, data.Workspace, data.Filter, data.Items, data.FetchedAt, l.ui)
	p := tea.NewProgram(model)
	l.ui.SetProgram(p)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(tui.AppModel); ok && m.Err() != nil {
		return m.Err()
	}
	return nil
}

type sessionMode int

const (
	modeNormal    sessionMode = iota
	modeDemo                  // skip config loading, use synthetic data
	modeBootstrap             // allow missing/empty config
	modeAuth                  // load config but skip provider/session creation
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

// YAML deserialization types.
type rawConfig struct {
	Theme            string                  `yaml:"theme"`
	Editor           string                  `yaml:"editor"`
	DefaultWorkspace string                  `yaml:"default_workspace"`
	CacheTTL         string                  `yaml:"cache_ttl"`
	Servers          map[string]rawServer    `yaml:"servers"`
	Workspaces       map[string]rawWorkspace `yaml:"workspaces"`
}

type rawServer struct {
	Provider string `yaml:"provider"` // e.g., "jira", "github"
	URL      string `yaml:"url"`
}

type rawWorkspace struct {
	Server   string            `yaml:"server"` // Server alias (references servers map)
	Name     string            `yaml:"name"`
	CacheTTL string            `yaml:"cache_ttl"`
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

// loadConfig reads and parses the YAML config file. ProviderConfig on each
// workspace is set to map[string]any — the composition root hydrates typed
// provider configs via provider-specific functions (e.g., jira.HydrateWorkspace).
func loadConfig(path string) (theme, editor, defaultWorkspace string, servers map[string]rawServer, workspaces map[string]*core.Workspace, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("reading config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", "", "", nil, nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(raw.Workspaces) == 0 {
		return "", "", "", nil, nil, fmt.Errorf("missing 'workspaces' in config")
	}

	if len(raw.Servers) == 0 {
		return "", "", "", nil, nil, fmt.Errorf("missing 'servers' in config — define your servers under the top-level 'servers:' key")
	}

	// Validate server definitions.
	for alias, srv := range raw.Servers {
		if srv.Provider == "" {
			return "", "", "", nil, nil, fmt.Errorf("server '%s' is missing 'provider' field", alias)
		}
		if srv.URL == "" {
			return "", "", "", nil, nil, fmt.Errorf("server '%s' is missing 'url' field", alias)
		}
	}

	// Second pass: parse each workspace block as map[string]any
	// to extract provider-specific fields.
	var fullConfig map[string]any
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return "", "", "", nil, nil, fmt.Errorf("re-parsing config: %w", err)
	}

	workspacesRaw, _ := fullConfig["workspaces"].(map[string]any)

	universalKeys := map[string]bool{
		"server": true, "name": true, "types": true, "statuses": true, "filters": true,
		"cache_ttl": true,
	}

	// Parse global cache TTL (falls back to core.DefaultCacheTTL).
	globalCacheTTL := core.DefaultCacheTTL
	if raw.CacheTTL != "" {
		d, err := time.ParseDuration(raw.CacheTTL)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("invalid global cache_ttl %q: %w", raw.CacheTTL, err)
		}
		globalCacheTTL = d
	}

	workspaces = make(map[string]*core.Workspace, len(raw.Workspaces))

	for slug, rws := range raw.Workspaces {
		if rws.Server == "" {
			return "", "", "", nil, nil, fmt.Errorf("workspace '%s' is missing 'server' field", slug)
		}

		srv, ok := raw.Servers[rws.Server]
		if !ok {
			return "", "", "", nil, nil, fmt.Errorf("workspace '%s' references unknown server '%s'", slug, rws.Server)
		}

		if len(rws.Types) == 0 {
			return "", "", "", nil, nil, fmt.Errorf("workspace '%s' is missing 'types' array", slug)
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
				return "", "", "", nil, nil, fmt.Errorf("workspace '%s': invalid cache_ttl %q: %w", slug, rws.CacheTTL, err)
			}
			cacheTTL = d
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
			Types:          types,
			Statuses:       statuses,
			Filters:        rws.Filters,
			StatusOrderMap: statusOrderMap,
			TypeOrderMap:   typeOrderMap,
			ProviderConfig: providerCfg,
		}
	}

	return raw.Theme, raw.Editor, raw.DefaultWorkspace, raw.Servers, workspaces, nil
}

// loadConfigOrEmpty attempts to load the config, returning empty values
// if the file doesn't exist. Used during bootstrap.
func loadConfigOrEmpty(path string) (theme, editor, defaultWorkspace string, servers map[string]rawServer, workspaces map[string]*core.Workspace, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", "", "", nil, make(map[string]*core.Workspace), nil
	}
	return loadConfig(path)
}

// newProviderForWorkspace creates a core.Provider and optionally a jira.API client
// for a specific workspace. Tokens are resolved via the credential store.
func newProviderForWorkspace(ws *core.Workspace, cacheDir string, creds auth.CredentialStore) (core.Provider, jira.API, error) {
	switch ws.Provider {
	case core.ProviderJira:
		token, err := creds.Get(ws.ServerAlias)
		if errors.Is(err, auth.ErrNotFound) {
			return nil, nil, fmt.Errorf(
				"no token found for server %q (%s).\nRun 'ihj auth login %s' to store your credentials",
				ws.ServerAlias, ws.BaseURL, ws.ServerAlias,
			)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reading token for server %q: %w", ws.ServerAlias, err)
		}
		jiraCfg, ok := ws.ProviderConfig.(*jira.Config)
		if !ok || jiraCfg == nil {
			return nil, nil, fmt.Errorf("workspace %q has no Jira configuration — run 'ihj jira bootstrap' first", ws.Slug)
		}
		client := jira.New(jiraCfg.Server, token)
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

// newCredentialStore builds a ChainStore with available backends.
// Keychain is preferred when available, with env vars and file as fallbacks.
func newCredentialStore(configDir string) auth.CredentialStore {
	var stores []auth.CredentialStore

	if auth.KeychainAvailable() {
		stores = append(stores, &auth.KeychainStore{})
	}
	stores = append(stores, &auth.EnvStore{})
	stores = append(stores, auth.NewFileStore(configDir))

	return auth.NewChainStore(stores...)
}

// hydrateWorkspace applies provider-specific hydration to a workspace.
func hydrateWorkspace(ws *core.Workspace) error {
	switch ws.Provider {
	case core.ProviderJira:
		if _, err := jira.HydrateWorkspace(ws); err != nil {
			return fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
		}
	}
	return nil
}
