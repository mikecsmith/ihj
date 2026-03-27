package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
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
	var client *jira.Client

	if isDemo {
		cfg = &storage.AppConfig{
			Workspaces: make(map[string]*core.Workspace),
		}
	} else {
		token := os.Getenv("JIRA_BASIC_TOKEN")
		if token == "" {
			return fmt.Errorf("JIRA_BASIC_TOKEN environment variable not set.\nSet it to base64(email:api_token) for Jira Cloud")
		}

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

		// Hydrate provider configs and find the server URL.
		var server string
		for _, ws := range cfg.Workspaces {
			if ws.Provider == "jira" {
				jiraCfg, err := jira.HydrateWorkspace(ws)
				if err != nil {
					return fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
				}
				if server == "" {
					server = jiraCfg.Server
				}
				ws.BaseURL = jiraCfg.Server
			}
		}

		if server != "" {
			client = jira.New(
				server,
				token,
				jira.WithContext(context.Background()),
			)
		}

		btUI.EditorCmd = cfg.EditorCommand()
	}

	if btUI.EditorCmd == "" {
		btUI.EditorCmd = cfg.EditorCommand()
	}

	// Resolve the default workspace for the provider.
	var provider *jira.Provider
	if client != nil {
		if ws, err := cfg.ResolveWorkspace(""); err == nil {
			provider = jira.NewProvider(client, ws)
		}
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
