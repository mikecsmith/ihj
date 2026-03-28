package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/jira/bootstrap"
)

func newRootCmd() *cobra.Command {
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
			return commands.RunTUI(getSession(cmd), board, filter)
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "export [board] [filter]", Short: "Export issue hierarchy as JSON",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			board, filter := optArgs(args)
			return commands.Export(getSession(cmd), board, filter)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "apply <file>",
		Short: "Apply an exported issue hierarchy from a file",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			s := getSession(cmd)
			client := getJiraClient(cmd)
			serverURL := ""
			if client == nil {
				// First-time bootstrap: no workspace configured yet.
				// Prompt for server URL and create a client on the fly.
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
			return bootstrap.Run(client, s.UI, s.Out, strings.ToUpper(args[0]), serverURL, len(s.Config.Workspaces))
		},
	})
	jiraCmd.AddCommand(&cobra.Command{
		Use: "demo", Short: "Launch TUI with synthetic Jira data (no credentials needed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunDemo(getSession(cmd))
		},
	})
	root.AddCommand(jiraCmd)

	createCmd := &cobra.Command{
		Use: "create", Short: "Create a new issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Create(getSession(cmd), flagVal(cmd, "workspace"), collectOverrides(cmd))
		},
	}
	addMutationFlags(createCmd)
	root.AddCommand(createCmd)

	editCmd := &cobra.Command{
		Use: "edit <issue_key>", Short: "Edit an existing issue",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Edit(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]), collectOverrides(cmd))
		},
	}
	addMutationFlags(editCmd)
	root.AddCommand(editCmd)

	root.AddCommand(&cobra.Command{
		Use: "comment <issue_key>", Short: "Add a comment",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Comment(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "assign <issue_key>", Short: "Assign to yourself",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Assign(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	transitionCmd := &cobra.Command{
		Use: "transition <issue_key>", Short: "Change status",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Transition(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]))
		},
	}
	transitionCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(transitionCmd)

	openCmd := &cobra.Command{
		Use: "open <issue_key>", Short: "Open in browser",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := getSession(cmd)
			ws, err := s.Config.ResolveWorkspace(flagVal(cmd, "workspace"))
			if err != nil {
				return err
			}
			return commands.OpenInBrowser(ws.BaseURL + "/browse/" + strings.ToUpper(args[0]))
		},
	}
	openCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(openCmd)

	root.AddCommand(&cobra.Command{
		Use: "branch <issue_key>", Short: "Copy git branch name to clipboard",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Branch(getSession(cmd), strings.ToUpper(args[0]))
		},
	})

	extractCmd := &cobra.Command{
		Use: "extract <issue_key>", Short: "Extract issue context for LLM prompt",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Extract(getSession(cmd), flagVal(cmd, "workspace"), strings.ToUpper(args[0]))
		},
	}
	extractCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.AddCommand(extractCmd)

	return root
}

// --- Flag helpers ---

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

// --- Context-based Session injection ---

type ctxKey string

const sessionCtxKey ctxKey = "ihj_session"

// contextWithSession returns a new context with the Session attached.
func contextWithSession(ctx context.Context, s *commands.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

// getSession extracts the Session from the Cobra command context.
func getSession(cmd *cobra.Command) *commands.Session {
	s, _ := cmd.Context().Value(sessionCtxKey).(*commands.Session)
	return s
}

const jiraClientCtxKey ctxKey = "ihj_jira_client"

// contextWithJiraClient attaches a jira.API client to the context.
func contextWithJiraClient(ctx context.Context, client jira.API) context.Context {
	if client == nil {
		return ctx
	}
	return context.WithValue(ctx, jiraClientCtxKey, client)
}

// getJiraClient extracts the jira client from the Cobra command context.
func getJiraClient(cmd *cobra.Command) jira.API {
	c, _ := cmd.Context().Value(jiraClientCtxKey).(jira.API)
	return c
}
