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
	// normalInit is a PersistentPreRunE that loads config and creates the session.
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
			return commands.RunUI(getSession(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	root.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.Flags().StringP("filter", "f", "", "Filter name")

	tuiCmd := &cobra.Command{
		Use: "tui", Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunUI(getSession(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	tuiCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	tuiCmd.Flags().StringP("filter", "f", "", "Filter name")
	root.AddCommand(tuiCmd)

	exportCmd := &cobra.Command{
		Use: "export", Short: "Export a manifest of items as YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Export(getSession(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	exportCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	exportCmd.Flags().StringP("filter", "f", "", "Filter name")
	root.AddCommand(exportCmd)

	root.AddCommand(&cobra.Command{
		Use:   "apply <file>",
		Short: "Apply an exported manifest from a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Apply(getSession(cmd), args[0])
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
			s := getSession(cmd)
			client := getJiraClient(cmd)
			serverURL := ""
			if client == nil {
				// First-time bootstrap: no workspace configured yet.
				var err error
				serverURL, err = s.UI.PromptText("Jira Server URL (e.g., https://company.atlassian.net)")
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
			return jira.Bootstrap(client, s.UI, s.Out, strings.ToUpper(args[0]), serverURL, len(s.Workspaces))
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
			s := getSession(cmd)
			if s.Launcher == nil {
				return fmt.Errorf("UI not available (Launcher not configured)")
			}
			ws, err := s.ResolveWorkspace("")
			if err != nil {
				return fmt.Errorf("demo workspace not configured: %w", err)
			}
			items, err := s.Provider.Search(context.TODO(), "active", false)
			if err != nil {
				return fmt.Errorf("loading demo data: %w", err)
			}
			return s.Launcher.LaunchUI(&commands.LaunchUIData{
				Session:   s,
				Workspace: ws,
				Filter:    "active",
				Items:     items,
			})
		},
	})
	root.AddCommand(jiraCmd)

	createCmd := &cobra.Command{
		Use: "create", Short: "Create a new item",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Create(getSession(cmd), flagVal(cmd, "workspace"), collectOverrides(cmd))
		},
	}
	addMutationFlags(createCmd)
	root.AddCommand(createCmd)

	editCmd := &cobra.Command{
		Use: "edit <id>", Short: "Edit an existing item",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Edit(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]), collectOverrides(cmd))
		},
	}
	addMutationFlags(editCmd)
	root.AddCommand(editCmd)

	root.AddCommand(&cobra.Command{
		Use: "comment <id>", Short: "Add a comment",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Comment(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "assign <id>", Short: "Assign to yourself",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Assign(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	transitionCmd := &cobra.Command{
		Use: "transition <id>", Short: "Change status",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Transition(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]))
		},
	}
	transitionCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(transitionCmd)

	openCmd := &cobra.Command{
		Use: "open <id>", Short: "Open in browser",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := getSession(cmd)
			ws, err := s.ResolveWorkspace(flagVal(cmd, "workspace"))
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
			return commands.Branch(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	extractCmd := &cobra.Command{
		Use: "extract <id>", Short: "Extract issue context for LLM prompt",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Extract(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]))
		},
	}
	extractCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(extractCmd)

	return root
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

const sessionCtxKey ctxKey = "ihj_session"

func contextWithSession(ctx context.Context, s *commands.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

func getSession(cmd *cobra.Command) *commands.Session {
	s, _ := cmd.Context().Value(sessionCtxKey).(*commands.Session)
	return s
}

const jiraClientCtxKey ctxKey = "ihj_jira_client"

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
