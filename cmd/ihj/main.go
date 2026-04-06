// Command ihj is a provider-agnostic work-tracking CLI and TUI.
//
// It connects to issue trackers (currently Jira) and presents their
// data through a keyboard-driven terminal interface. See the internal
// packages for the domain model (core), encoding boundary (encoding),
// business logic (commands), terminal UI (tui), and provider
// implementations (jira).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/auth"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/headless"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/jira/fakejira"
	"github.com/mikecsmith/ihj/internal/tui"
)

// Set by goreleaser via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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
		tLauncher.showHelpBar = caps.ShowHelpBar
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

		// Per-invocation credential store — each call to initSession
		// starts from the process-wide chain and may layer on extras
		// (e.g. demo mode seeds tokens for its fakejira aliases).
		creds := creds

		// Per-invocation cache dir — normally the shared XDG path, but
		// demo mode swaps in a temp dir so each fake run gets fresh
		// createmeta + work-item caches and never pollutes (or reads
		// from) real workspace caches.
		cacheDir := cacheDir

		switch mode {
		case modeDemo:
			// Stand up two in-process Jira servers — one scrum (DEMO)
			// and one kanban (OPS) — and expose each as its own
			// workspace. Everything runs through the real jira.Provider
			// and the real credential lookup path; we just prepend an
			// in-memory store holding a dummy token for the demo aliases.
			scrumSrv := fakejira.NewServer()
			kanbanSrv := fakejira.NewKanbanServer()
			scrumWS := fakejira.Workspace()
			scrumWS.BaseURL = scrumSrv.URL
			kanbanWS := fakejira.WorkspaceKanban()
			kanbanWS.BaseURL = kanbanSrv.URL
			for _, ws := range []*core.Workspace{scrumWS, kanbanWS} {
				if err := hydrateWorkspace(ws); err != nil {
					return ctx, err
				}
			}
			_, _ = scrumSrv, kanbanSrv // retained by accept goroutines for the process lifetime
			cfg.DefaultWorkspace = scrumWS.Slug
			cfg.Workspaces = map[string]*core.Workspace{
				scrumWS.Slug:  scrumWS,
				kanbanWS.Slug: kanbanWS,
			}
			demoTokens := &memCredStore{tokens: map[string]string{
				scrumWS.ServerAlias:  "demo-token",
				kanbanWS.ServerAlias: "demo-token",
			}}
			creds = auth.NewChainStore(demoTokens, creds)

			// Demo mode owns a dedicated cache path that is wiped at the
			// start of every run. This keeps createmeta / work-item
			// caches fresh against the ephemeral fakejira servers and
			// guarantees real workspace caches are never touched.
			demoCache := filepath.Join(os.TempDir(), "ihj-demo-cache")
			if err := os.RemoveAll(demoCache); err != nil {
				return ctx, fmt.Errorf("demo cache: %w", err)
			}
			if err := os.MkdirAll(demoCache, 0o755); err != nil {
				return ctx, fmt.Errorf("demo cache: %w", err)
			}
			cacheDir = demoCache

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
				EditorCmd:   editorCommand(cfg.Editor),
				VimMode:     cfg.VimMode,
				Shortcuts:   cfg.Shortcuts,
				DetailPct:   cfg.DetailPct,
				ShowHelpBar: cfg.ShowHelpBar,
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
			provider, err := newProviderForWorkspace(ws, cacheDir, creds)
			if err != nil {
				return nil, err
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

	root := newRootCmd(initSession, versionString())
	return root.ExecuteContext(context.Background())
}

// tuiLauncher implements commands.UILauncher using Bubble Tea.
type tuiLauncher struct {
	ui          *tui.BubbleTeaUI
	vimMode     bool
	shortcuts   map[string]string
	detailPct   int
	showHelpBar bool
}

func (l *tuiLauncher) LaunchUI(data *commands.LaunchUIData) error {
	// Swap runtime.UI to the TUI implementation for the duration of the TUI
	// session. The TUI's BubbleTeaUI bridges commands.UI calls to the Bubble
	// Tea event loop via channels.
	origUI := data.Runtime.UI
	data.Runtime.UI = l.ui
	defer func() { data.Runtime.UI = origUI }()

	model := tui.NewAppModel(data.Ctx, data.Runtime, data.Session, data.Factory, data.Workspace, data.Filter, data.Items, data.FetchedAt, l.ui, l.vimMode, l.shortcuts, l.detailPct, l.showHelpBar)
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

// newProviderForWorkspace creates a core.Provider for a specific workspace.
// Tokens are resolved via the credential store.
func newProviderForWorkspace(ws *core.Workspace, cacheDir string, creds auth.CredentialStore) (core.Provider, error) {
	switch ws.Provider {
	case core.ProviderJira:
		token, err := creds.Get(ws.ServerAlias)
		if errors.Is(err, auth.ErrNotFound) {
			return nil, fmt.Errorf(
				"no token found for server %q (%s).\nRun 'ihj auth login %s' to store your credentials",
				ws.ServerAlias, ws.BaseURL, ws.ServerAlias,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("reading token for server %q: %w", ws.ServerAlias, err)
		}
		jiraCfg, ok := ws.ProviderConfig.(*jira.Config)
		if !ok || jiraCfg == nil {
			return nil, fmt.Errorf("workspace %q has no Jira configuration — run 'ihj jira bootstrap' first", ws.Slug)
		}
		client := jira.New(jiraCfg.Server, token)
		provider := jira.NewProvider(client, ws, cacheDir)
		return provider, nil

	default:
		return nil, fmt.Errorf("unsupported provider %q for workspace %q", ws.Provider, ws.Slug)
	}
}

// memCredStore is an in-memory CredentialStore used only by demo mode
// to seed dummy tokens for its fakejira server aliases. It never touches
// the keychain, environment, or disk.
type memCredStore struct{ tokens map[string]string }

func (m *memCredStore) Get(alias string) (string, error) {
	if t, ok := m.tokens[alias]; ok {
		return t, nil
	}
	return "", auth.ErrNotFound
}
func (m *memCredStore) Set(alias, token string) error { m.tokens[alias] = token; return nil }
func (m *memCredStore) Delete(alias string) error     { delete(m.tokens, alias); return nil }
func (m *memCredStore) List() ([]string, error) {
	out := make([]string, 0, len(m.tokens))
	for k := range m.tokens {
		out = append(out, k)
	}
	return out, nil
}

// newCredentialStore builds a ChainStore with available backends prefering the keychain first.
func newCredentialStore(configDir string) auth.CredentialStore {
	return auth.NewChainStore(
		&auth.KeychainStore{},
		&auth.EnvStore{},
		auth.NewFileStore(configDir),
	)
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
