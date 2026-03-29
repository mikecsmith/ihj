package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/jira"
)

type sessionInitFunc func(ctx context.Context, mode sessionMode) (context.Context, error)

func newRootCmd(initSession sessionInitFunc) *cobra.Command {
	// normalInit is a PersistentPreRunE that loads config and creates the runtime.
	normalInit := func(cmd *cobra.Command, args []string) error {
		ctx, err := initSession(cmd.Context(), modeNormal)
		if err != nil {
			return err
		}
		cmd.SetContext(ctx)
		return nil
	}

	root := &cobra.Command{
		Use:   "ihj",
		Short: "The Instant High-speed Jira CLI",
		// Default to TUI when no subcommand is given.
		PersistentPreRunE: normalInit,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunUI(getRuntime(cmd), getFactory(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	root.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.Flags().StringP("filter", "f", "", "Filter name")

	tuiCmd := &cobra.Command{
		Use: "tui", Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunUI(getRuntime(cmd), getFactory(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	tuiCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	tuiCmd.Flags().StringP("filter", "f", "", "Filter name")
	root.AddCommand(tuiCmd)

	exportCmd := &cobra.Command{
		Use: "export", Short: "Export a manifest of items as YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := resolveSession(cmd)
			if err != nil {
				return err
			}
			full, _ := cmd.Flags().GetBool("full")
			return commands.Export(ws, flagVal(cmd, "filter"), full)
		},
	}
	exportCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	exportCmd.Flags().StringP("filter", "f", "", "Filter name")
	exportCmd.Flags().Bool("full", false, "Include extended and read-only fields")
	root.AddCommand(exportCmd)

	root.AddCommand(&cobra.Command{
		Use:   "apply <file>",
		Short: "Apply an exported manifest from a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Apply(getRuntime(cmd), getFactory(cmd), args[0])
		},
	})

	jiraCmd := &cobra.Command{
		Use:   "jira",
		Short: "Jira-specific commands",
	}
	jiraCmd.AddCommand(&cobra.Command{
		Use: "bootstrap <project_key>", Short: "Scaffold a board config from Jira",
		Args: cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := initSession(cmd.Context(), modeBootstrap)
			if err != nil {
				return err
			}
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			client := getJiraClient(cmd)
			serverURL := ""
			if client == nil {
				// First-time bootstrap: no workspace configured yet.
				var err error
				serverURL, err = rt.UI.PromptText("Jira Server URL (e.g., https://company.atlassian.net)")
				if err != nil || serverURL == "" {
					return fmt.Errorf("server URL is required for bootstrap")
				}
				serverURL = strings.TrimRight(serverURL, "/")
				token := os.Getenv("JIRA_BASIC_TOKEN")
				if token == "" {
					return fmt.Errorf("JIRA_BASIC_TOKEN environment variable not set")
				}
				client = jira.New(serverURL, token)
			}
			return jira.Bootstrap(client, rt.UI, rt.Out, strings.ToUpper(args[0]), serverURL, len(rt.Workspaces))
		},
	})
	jiraCmd.AddCommand(&cobra.Command{
		Use: "demo", Short: "Launch TUI with synthetic Jira data (no credentials needed)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := initSession(cmd.Context(), modeDemo)
			if err != nil {
				return err
			}
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			factory := getFactory(cmd)
			if rt.Launcher == nil {
				return fmt.Errorf("UI not available (Launcher not configured)")
			}
			wsSess, err := factory("")
			if err != nil {
				return fmt.Errorf("demo workspace not configured: %w", err)
			}
			items, err := wsSess.Provider.Search(context.TODO(), "active", false)
			if err != nil {
				return fmt.Errorf("loading demo data: %w", err)
			}
			return rt.Launcher.LaunchUI(&commands.LaunchUIData{
				Runtime:   rt,
				Session:   wsSess,
				Factory:   factory,
				Workspace: wsSess.Workspace,
				Filter:    "active",
				Items:     items,
			})
		},
	})
	root.AddCommand(jiraCmd)

	createCmd := &cobra.Command{
		Use: "create", Short: "Create a new item",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := resolveSession(cmd)
			if err != nil {
				return err
			}
			return commands.Create(ws, collectOverrides(cmd))
		},
	}
	addMutationFlags(createCmd)
	root.AddCommand(createCmd)

	editCmd := &cobra.Command{
		Use: "edit <id>", Short: "Edit an existing item",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := resolveSession(cmd)
			if err != nil {
				return err
			}
			return commands.Edit(ws, strings.ToUpper(args[0]), collectOverrides(cmd))
		},
	}
	addMutationFlags(editCmd)
	root.AddCommand(editCmd)

	root.AddCommand(&cobra.Command{
		Use: "comment <id>", Short: "Add a comment",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getDefaultSession(cmd)
			return commands.Comment(ws, strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "assign <id>", Short: "Assign to yourself",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getDefaultSession(cmd)
			return commands.Assign(ws, strings.ToUpper(args[0]))
		},
	})

	transitionCmd := &cobra.Command{
		Use: "transition <id>", Short: "Change status",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := resolveSession(cmd)
			if err != nil {
				return err
			}
			return commands.Transition(ws, strings.ToUpper(args[0]))
		},
	}
	transitionCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(transitionCmd)

	openCmd := &cobra.Command{
		Use: "open <id>", Short: "Open in browser",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			ws, err := rt.ResolveWorkspace(flagVal(cmd, "workspace"))
			if err != nil {
				return err
			}
			url := ws.BrowseURL(strings.ToUpper(args[0]))
			if url == "" {
				return fmt.Errorf("no browse URL configured for workspace %q", ws.Slug)
			}
			return commands.OpenInBrowser(url)
		},
	}
	openCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(openCmd)

	root.AddCommand(&cobra.Command{
		Use: "branch <id>", Short: "Copy git branch name to clipboard",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getDefaultSession(cmd)
			return commands.Branch(ws, strings.ToUpper(args[0]))
		},
	})

	extractCmd := &cobra.Command{
		Use: "extract [id]", Short: "Extract issue context for LLM prompt",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := resolveSession(cmd)
			if err != nil {
				return err
			}
			var issueKey string
			if len(args) > 0 {
				issueKey = strings.ToUpper(args[0])
			}
			copyFlag, _ := cmd.Flags().GetBool("copy")
			return commands.Extract(ws, issueKey, commands.ExtractOptions{
				Scope:  flagVal(cmd, "scope"),
				Prompt: flagVal(cmd, "prompt"),
				Copy:   copyFlag,
			})
		},
	}
	extractCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	extractCmd.Flags().StringP("scope", "s", "", "Scope: selected, children, parent, family, workspace")
	extractCmd.Flags().StringP("prompt", "p", "", "LLM prompt text (skip editor)")
	extractCmd.Flags().BoolP("copy", "c", false, "Copy to clipboard instead of stdout")
	root.AddCommand(extractCmd)

	return root
}

// resolveSession creates a WorkspaceSession for the workspace flag (or default).
func resolveSession(cmd *cobra.Command) (*commands.WorkspaceSession, error) {
	slug := flagVal(cmd, "workspace")
	if slug == "" {
		// If no workspace flag, use the pre-created default session.
		if ws := getDefaultSession(cmd); ws != nil {
			return ws, nil
		}
	}
	return getFactory(cmd)(slug)
}

func addMutationFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	cmd.Flags().StringP("summary", "s", "", "Summary")
	cmd.Flags().StringP("type", "t", "", "Issue type")
	cmd.Flags().StringP("priority", "p", "", "Priority")
	cmd.Flags().StringP("status", "S", "", "Status")
	cmd.Flags().StringP("parent", "P", "", "Parent key")
}

func collectOverrides(cmd *cobra.Command) map[string]string {
	m := make(map[string]string)
	for _, k := range []string{"summary", "type", "priority", "status", "parent"} {
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

type ctxKey string

const (
	runtimeCtxKey        ctxKey = "ihj_runtime"
	factoryCtxKey        ctxKey = "ihj_factory"
	defaultSessionCtxKey ctxKey = "ihj_default_session"
	jiraClientCtxKey     ctxKey = "ihj_jira_client"
)

func contextWithRuntime(ctx context.Context, rt *commands.Runtime) context.Context {
	return context.WithValue(ctx, runtimeCtxKey, rt)
}

func getRuntime(cmd *cobra.Command) *commands.Runtime {
	rt, _ := cmd.Context().Value(runtimeCtxKey).(*commands.Runtime)
	return rt
}

func contextWithFactory(ctx context.Context, f commands.WorkspaceSessionFactory) context.Context {
	return context.WithValue(ctx, factoryCtxKey, f)
}

func getFactory(cmd *cobra.Command) commands.WorkspaceSessionFactory {
	f, _ := cmd.Context().Value(factoryCtxKey).(commands.WorkspaceSessionFactory)
	return f
}

func contextWithDefaultSession(ctx context.Context, ws *commands.WorkspaceSession) context.Context {
	return context.WithValue(ctx, defaultSessionCtxKey, ws)
}

func getDefaultSession(cmd *cobra.Command) *commands.WorkspaceSession {
	ws, _ := cmd.Context().Value(defaultSessionCtxKey).(*commands.WorkspaceSession)
	return ws
}

func contextWithJiraClient(ctx context.Context, client jira.API) context.Context {
	if client == nil {
		return ctx
	}
	return context.WithValue(ctx, jiraClientCtxKey, client)
}

func getJiraClient(cmd *cobra.Command) jira.API {
	c, _ := cmd.Context().Value(jiraClientCtxKey).(jira.API)
	return c
}
