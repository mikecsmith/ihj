package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/demo"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/storage"
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
	paths := storage.DefaultPaths()
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	btUI := tui.NewBubbleTeaUI()

	// initSession loads config, creates a provider, and attaches
	// the Session to the cobra context. Called by PersistentPreRunE.
	initSession := func(ctx context.Context, mode sessionMode) (context.Context, error) {
		var cfg *storage.AppConfig

		switch mode {
		case modeDemo:
			cfg = &storage.AppConfig{}
			demo.SetupConfig(cfg)
		case modeBootstrap:
			var err error
			cfg, err = storage.LoadConfigOrEmpty(paths.ConfigFile)
			if err != nil {
				return ctx, fmt.Errorf("config: %w", err)
			}
		default:
			var err error
			cfg, err = storage.LoadConfig(paths.ConfigFile)
			if err != nil {
				if os.IsNotExist(err) {
					return ctx, fmt.Errorf("config not found at %s — run 'ihj jira bootstrap <PROJECT>' first", paths.ConfigFile)
				}
				return ctx, fmt.Errorf("config: %w", err)
			}

			for _, ws := range cfg.Workspaces {
				switch ws.Provider {
				case "jira":
					jiraCfg, err := jira.HydrateWorkspace(ws)
					if err != nil {
						return ctx, fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
					}
					ws.BaseURL = jiraCfg.Server
				}
			}
		}

		btUI.EditorCmd = cfg.EditorCommand()

		provider, client, err := newProvider(cfg, paths.CacheDir)
		if err != nil {
			return ctx, err
		}

		s := &commands.Session{
			Config:   cfg,
			Provider: provider,
			UI:       btUI,
			CacheDir: paths.CacheDir,
			Out:      stdout,
			Err:      stderr,
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

// newProvider creates a core.Provider and optionally a jira.API client for the
// default workspace. The client is only needed for bootstrap.
func newProvider(cfg *storage.AppConfig, cacheDir string) (core.Provider, jira.API, error) {
	ws, err := cfg.ResolveWorkspace("")
	if err != nil {
		// No default workspace configured — not an error for bootstrap.
		return nil, nil, nil
	}

	switch ws.Provider {
	case "jira":
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

	case "demo":
		items := demo.Issues()
		provider := demo.NewProvider(items, 150*time.Millisecond)
		return provider, nil, nil

	default:
		return nil, nil, fmt.Errorf("unsupported provider %q for workspace %q", ws.Provider, ws.Slug)
	}
}
