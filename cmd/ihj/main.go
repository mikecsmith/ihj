package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/tui"
)

func main() {
	isDemo := len(os.Args) > 1 && os.Args[1] == "demo"

	paths := config.DefaultPaths()
	if err := paths.EnsureDirs(); err != nil {
		fatal("Setup: %v", err)
	}

	btUI := tui.NewBubbleTeaUI()

	var cfg *config.Config
	var c *client.Client

	if isDemo {
		// Demo mode: no token or config required.
		cfg = &config.Config{
			Server: "https://demo.atlassian.net",
		}
	} else {
		token := os.Getenv("JIRA_BASIC_TOKEN")
		if token == "" {
			fatal("JIRA_BASIC_TOKEN environment variable not set.\nSet it to base64(email:api_token) for Jira Cloud.")
		}

		isBootstrap := len(os.Args) > 1 && os.Args[1] == "bootstrap"

		var err error
		if isBootstrap {
			cfg, err = config.LoadOrEmpty(paths.ConfigFile)
		} else {
			cfg, err = config.Load(paths.ConfigFile)
		}
		if err != nil {
			if os.IsNotExist(err) && !isBootstrap {
				fatal("Config not found at %s. Run 'ihj bootstrap <PROJECT>' first.", paths.ConfigFile)
			}
			fatal("Config: %v", err)
		}

		c = client.New(
			strings.TrimRight(cfg.Server, "/"),
			token,
			client.WithContext(context.Background()),
		)

		btUI.EditorCmd = cfg.EditorCommand()
	}

	// Ensure EditorCmd is always set (demo mode skips config loading).
	if btUI.EditorCmd == "" {
		btUI.EditorCmd = cfg.EditorCommand()
	}

	app := &commands.App{
		Config:   cfg,
		Client:   c,
		UI:       btUI,
		CacheDir: paths.CacheDir,
		Out:      os.Stdout,
		Err:      os.Stderr,
		LaunchTUI: func(data *commands.LaunchTUIData) error {
			model := tui.NewAppModel(data.App, data.Board, data.Filter, data.Views, data.FetchedAt)
			p := tea.NewProgram(model)
			btUI.SetProgram(p)
			_, err := p.Run()
			return err
		},
	}

	root := commands.NewRootCmd()

	// Default to TUI when no subcommand is given.
	if len(os.Args) < 2 || !isSubcommand(os.Args[1]) {
		// Insert "tui" as the subcommand.
		os.Args = append([]string{os.Args[0], "tui"}, os.Args[1:]...)
	}

	// Inject the App into the context and execute the root command.
	// Cobra will automatically propagate this context to whichever subcommand runs.
	ctx := commands.ContextWithApp(context.Background(), app)
	if err := root.ExecuteContext(ctx); err != nil {
		if commands.IsCancelled(err) {
			os.Exit(0)
		}
		os.Exit(1)
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
