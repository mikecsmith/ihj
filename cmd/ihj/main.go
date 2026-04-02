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
	"time"

	tea "charm.land/bubbletea/v2"

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

	tLauncher := &tuiLauncher{ui: tuiUI}
	err := run(os.Stdout, os.Stderr, configDir, configFile, cacheDir, cliUI, tLauncher, func(caps uiCaps) {
		cliUI.EditorCmd = caps.EditorCmd
		tuiUI.EditorCmd = caps.EditorCmd
		tLauncher.vimMode = caps.VimMode
		tLauncher.shortcuts = caps.Shortcuts
		tLauncher.detailPct = caps.DetailPct
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
func run(stdout, stderr io.Writer, configDir, configFile, cacheDir string, cliUI commands.UI, launcher commands.UILauncher, onConfig func(uiCaps)) error {
	if err := ensureDirs(configDir, cacheDir); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	// Build credential store: keychain (if available) → env vars → file.
	creds := newCredentialStore(configDir)

	// initSession loads config, creates a Runtime + factory, and attaches
	// them to the cobra context. Called by PersistentPreRunE.
	initSession := func(ctx context.Context, mode sessionMode) (context.Context, error) {
		var cfg configResult

		switch mode {
		case modeDemo:
			ws := demo.Workspace()
			cfg.DefaultWorkspace = ws.Slug
			cfg.Workspaces = map[string]*core.Workspace{ws.Slug: ws}

		case modeBootstrap:
			var err error
			cfg, err = loadConfigOrEmpty(configFile)
			if err != nil {
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range cfg.Workspaces {
				if err := hydrateWorkspace(ws); err != nil {
					return ctx, err
				}
			}

		case modeAuth:
			var err error
			cfg, err = loadConfig(configFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", configFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}
			// Auth mode: skip hydration and session creation.

		default:
			var err error
			cfg, err = loadConfig(configFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", configFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range cfg.Workspaces {
				if err := hydrateWorkspace(ws); err != nil {
					return ctx, err
				}
			}
		}

		if onConfig != nil {
			onConfig(uiCaps{
				EditorCmd: editorCommand(cfg.Editor),
				VimMode:   cfg.VimMode,
				Shortcuts: cfg.Shortcuts,
				DetailPct: cfg.DetailPct,
			})
		}

		rt := &commands.Runtime{
			Theme:            cfg.Theme,
			DefaultWorkspace: cfg.DefaultWorkspace,
			Workspaces:       cfg.Workspaces,
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
		ctx = contextWithServers(ctx, cfg.Servers)

		// Pre-create session for default workspace to detect auth errors early.
		// Skip for auth mode — we don't need provider connections.
		if mode != modeAuth && cfg.DefaultWorkspace != "" {
			if _, ok := cfg.Workspaces[cfg.DefaultWorkspace]; ok {
				wsSess, err := factory(cfg.DefaultWorkspace)
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
	ui        *tui.BubbleTeaUI
	vimMode   bool
	shortcuts map[string]string
	detailPct int
}

func (l *tuiLauncher) LaunchUI(data *commands.LaunchUIData) error {
	// Swap runtime.UI to the TUI implementation for the duration of the TUI
	// session. The TUI's BubbleTeaUI bridges commands.UI calls to the Bubble
	// Tea event loop via channels.
	origUI := data.Runtime.UI
	data.Runtime.UI = l.ui
	defer func() { data.Runtime.UI = origUI }()

	model := tui.NewAppModel(data.Ctx, data.Runtime, data.Session, data.Factory, data.Workspace, data.Filter, data.Items, data.FetchedAt, l.ui, l.vimMode, l.shortcuts, l.detailPct)
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
