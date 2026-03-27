package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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
	if err := run(os.Args, os.Stdout, os.Stderr); err != nil {
		if commands.IsCancelled(err) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run is the real entry point. It returns an error instead of calling
// os.Exit, making it testable without process termination.
func run(args []string, stdout, stderr io.Writer) error {
	isDemo := len(args) > 1 && args[1] == "demo"

	paths := storage.DefaultPaths()
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	btUI := tui.NewBubbleTeaUI()

	var cfg *storage.AppConfig

	if isDemo {
		cfg = &storage.AppConfig{}
		demo.SetupConfig(cfg)
	} else {
		isBootstrap := len(args) > 1 && args[1] == "bootstrap"

		var err error
		if isBootstrap {
			cfg, err = storage.LoadConfigOrEmpty(paths.ConfigFile)
		} else {
			cfg, err = storage.LoadConfig(paths.ConfigFile)
		}
		if err != nil {
			if os.IsNotExist(err) && !isBootstrap {
				return fmt.Errorf("config not found at %s — run 'ihj bootstrap <PROJECT>' first", paths.ConfigFile)
			}
			return fmt.Errorf("config: %w", err)
		}

		// Hydrate provider-specific configs for each workspace.
		for _, ws := range cfg.Workspaces {
			switch ws.Provider {
			case "jira":
				jiraCfg, err := jira.HydrateWorkspace(ws)
				if err != nil {
					return fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
				}
				ws.BaseURL = jiraCfg.Server
			default:
				// Future providers hydrate here.
			}
		}

		btUI.EditorCmd = cfg.EditorCommand()
	}

	if btUI.EditorCmd == "" {
		btUI.EditorCmd = cfg.EditorCommand()
	}

	// Create a provider for the default workspace.
	provider, client, err := newProvider(cfg, paths.CacheDir)
	if err != nil {
		return err
	}

	app := &commands.App{
		Config:   cfg,
		Client:   client,
		Provider: provider,
		UI:       btUI,
		CacheDir: paths.CacheDir,
		Out:      stdout,
		Err:      stderr,
		LaunchTUI: func(data *commands.LaunchTUIData) error {
			model := tui.NewAppModel(data.App, data.Workspace, data.Filter, data.Items, data.FetchedAt)
			p := tea.NewProgram(model)
			btUI.SetProgram(p)
			_, err := p.Run()
			return err
		},
	}

	root := newRootCmd()

	// Default to TUI when no subcommand is given.
	if len(args) < 2 || !isSubcommand(args[1]) {
		args = append([]string{args[0], "tui"}, args[1:]...)
	}

	// Cobra reads os.Args directly, so we must update it.
	os.Args = args

	ctx := contextWithApp(context.Background(), app)
	return root.ExecuteContext(ctx)
}

// newProvider creates a core.Provider (and transitional jira.Client) for the
// default workspace. It dispatches on the workspace's Provider field so that
// adding a new backend is a single case in this switch.
func newProvider(cfg *storage.AppConfig, cacheDir string) (core.Provider, *jira.Client, error) {
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

// isSubcommand checks if the arg is a known subcommand rather than a flag or board slug.
func isSubcommand(arg string) bool {
	known := map[string]bool{
		"tui": true, "export": true, "apply": true, "bootstrap": true,
		"create": true, "edit": true, "comment": true,
		"assign": true, "transition": true, "open": true,
		"branch": true, "extract": true, "demo": true,
		"help": true, "completion": true,
	}
	return known[arg] || strings.HasPrefix(arg, "-")
}
