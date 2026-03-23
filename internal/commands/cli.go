package commands

import (
	"context"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ihj",
		Short: "The Instant High-speed Jira CLI",
	}

	// TUI is the default command when no subcommand is given.
	root.AddCommand(&cobra.Command{
		Use: "tui [board] [filter]", Short: "Launch interactive TUI (default)",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			board, filter := optArgs(args)
			return RunTUI(getApp(cmd), board, filter)
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "export [board] [filter]", Short: "Export issue hierarchy as JSON",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			board, filter := optArgs(args)
			return Export(getApp(cmd), board, filter)
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "bootstrap <project_key>", Short: "Scaffold a board config from Jira",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Bootstrap(getApp(cmd), strings.ToUpper(args[0]))
		},
	})

	createCmd := &cobra.Command{
		Use: "create", Short: "Create a new Jira issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Upsert(getApp(cmd), UpsertOpts{
				Board: flagVal(cmd, "board"), IsEdit: false, Overrides: collectOverrides(cmd),
			})
		},
	}
	addUpsertFlags(createCmd)
	root.AddCommand(createCmd)

	editCmd := &cobra.Command{
		Use: "edit <issue_key>", Short: "Edit an existing issue",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Upsert(getApp(cmd), UpsertOpts{
				Board: flagVal(cmd, "board"), IssueKey: strings.ToUpper(args[0]),
				IsEdit: true, Overrides: collectOverrides(cmd),
			})
		},
	}
	addUpsertFlags(editCmd)
	root.AddCommand(editCmd)

	root.AddCommand(&cobra.Command{
		Use: "comment <issue_key>", Short: "Add a comment",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Comment(getApp(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "assign <issue_key>", Short: "Assign to yourself",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Assign(getApp(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "transition <issue_key>", Short: "Change status",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Transition(getApp(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "open <issue_key>", Short: "Open in browser",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			return openURL(app.Config.Server + "/browse/" + strings.ToUpper(args[0]))
		},
	})

	branchCmd := &cobra.Command{
		Use: "branch <issue_key>", Short: "Copy git branch name to clipboard",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Branch(getApp(cmd), strings.ToUpper(args[0]), flagVal(cmd, "board"))
		},
	}
	branchCmd.Flags().StringP("board", "b", "", "Restrict cache lookup to board")
	root.AddCommand(branchCmd)

	root.AddCommand(&cobra.Command{
		Use: "extract <issue_key>", Short: "Extract issue context for LLM prompt",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Extract(getApp(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "demo", Short: "Launch TUI with synthetic data (no Jira needed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDemo(getApp(cmd))
		},
	})

	return root
}

// --- Flag helpers ---

func addUpsertFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("board", "b", "", "Board slug")
	cmd.Flags().StringP("summary", "s", "", "Summary")
	cmd.Flags().StringP("type", "t", "", "Issue type")
	cmd.Flags().StringP("priority", "p", "", "Priority")
	cmd.Flags().StringP("status", "S", "", "Status")
	cmd.Flags().StringP("team", "T", "", "Team")
	cmd.Flags().StringP("parent", "P", "", "Parent key")
}

func collectOverrides(cmd *cobra.Command) map[string]string {
	m := make(map[string]string)
	for _, k := range []string{"summary", "type", "priority", "status", "team", "parent"} {
		if v := flagVal(cmd, k); v != "" {
			m[k] = v
		}
	}
	return m
}

func flagVal(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func optArgs(args []string) (string, string) {
	a, b := "", ""
	if len(args) > 0 {
		a = args[0]
	}
	if len(args) > 1 {
		b = args[1]
	}
	return a, b
}

// --- Context-based App injection ---

type ctxKey string

const appCtxKey ctxKey = "ihj_app"

// ContextWithApp returns a new context with the App attached.
func ContextWithApp(ctx context.Context, app *App) context.Context {
	return context.WithValue(ctx, appCtxKey, app)
}

// getApp extracts the App from the Cobra command context cleanly.
// Thanks to Cobra 1.10.x and ExecuteContext, this context automatically
// propagates down to all subcommands. No more Parent() looping!
func getApp(cmd *cobra.Command) *App {
	app, _ := cmd.Context().Value(appCtxKey).(*App)
	return app
}

// --- Browser helper ---

func openURL(url string) error {
	candidates := []string{"open", "xdg-open"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return exec.Command(path, url).Start()
		}
	}
	return nil
}
