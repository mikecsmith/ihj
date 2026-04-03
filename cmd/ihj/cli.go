package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mikecsmith/ihj/internal/auth"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/jira"
)

type sessionInitFunc func(ctx context.Context, mode sessionMode) (context.Context, error)

func versionString() string {
	v := version
	if commit != "none" {
		v += " (" + commit + ")"
	}
	if date != "unknown" {
		v += " built " + date
	}
	return v
}

func newRootCmd(initSession sessionInitFunc, version string) *cobra.Command {
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
		Use:     "ihj",
		Short:   "The Instant High-speed Jira CLI",
		Version: version,
		// Default to TUI when no subcommand is given.
		PersistentPreRunE: normalInit,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunUI(cmd.Context(), getRuntime(cmd), getFactory(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
		},
	}
	root.Flags().StringP("workspace", "w", "", "Workspace slug")
	root.Flags().StringP("filter", "f", "", "Filter name")

	tuiCmd := &cobra.Command{
		Use: "tui", Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunUI(cmd.Context(), getRuntime(cmd), getFactory(cmd), flagVal(cmd, "workspace"), flagVal(cmd, "filter"))
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
			return commands.Export(cmd.Context(), ws, flagVal(cmd, "filter"), full)
		},
	}
	exportCmd.Flags().StringP("workspace", "w", "", "Workspace slug")
	exportCmd.Flags().StringP("filter", "f", "", "Filter name")
	exportCmd.Flags().Bool("full", false, "Include extended and read-only fields")
	root.AddCommand(exportCmd)

	applyCmd := &cobra.Command{
		Use:   "apply <file>",
		Short: "Apply an exported manifest from a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Apply(cmd.Context(), getRuntime(cmd), getFactory(cmd), args[0], flagVal(cmd, "workspace"))
		},
	}
	applyCmd.Flags().StringP("workspace", "w", "", "Workspace slug (overrides manifest metadata)")
	root.AddCommand(applyCmd)

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
			creds := getCredStore(cmd)

			serverURL, serverAlias, token, err := resolveBootstrapServer(rt, creds)
			if err != nil {
				return err
			}

			client := jira.New(serverURL, token)
			return jira.Bootstrap(cmd.Context(), client, rt.UI, rt.Out, strings.ToUpper(args[0]), serverURL, serverAlias, len(rt.Workspaces))
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
			items, err := wsSess.Provider.Search(cmd.Context(), "active", false)
			if err != nil {
				return fmt.Errorf("loading demo data: %w", err)
			}
			return rt.Launcher.LaunchUI(&commands.LaunchUIData{
				Ctx:       cmd.Context(),
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
			return commands.Create(cmd.Context(), ws, collectOverrides(cmd))
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
			return commands.Edit(cmd.Context(), ws, strings.ToUpper(args[0]), collectOverrides(cmd))
		},
	}
	addMutationFlags(editCmd)
	root.AddCommand(editCmd)

	root.AddCommand(&cobra.Command{
		Use: "comment <id>", Short: "Add a comment",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getDefaultSession(cmd)
			return commands.Comment(cmd.Context(), ws, strings.ToUpper(args[0]))
		},
	})

	root.AddCommand(&cobra.Command{
		Use: "assign <id>", Short: "Assign to yourself",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getDefaultSession(cmd)
			return commands.Assign(cmd.Context(), ws, strings.ToUpper(args[0]))
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
			return commands.Transition(cmd.Context(), ws, strings.ToUpper(args[0]))
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
			return commands.Branch(cmd.Context(), ws, strings.ToUpper(args[0]))
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
			return commands.Extract(cmd.Context(), ws, issueKey, commands.ExtractOptions{
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

	// ── Auth commands ───────────────────────────────────────────
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage server authentication",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := initSession(cmd.Context(), modeAuth)
			if err != nil {
				return err
			}
			cmd.SetContext(ctx)
			return nil
		},
	}

	authCmd.AddCommand(&cobra.Command{
		Use:   "login <server-alias>",
		Short: "Store an access token for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			creds := getCredStore(cmd)
			servers := getServers(cmd)
			alias := args[0]

			srv, ok := servers[alias]
			if !ok {
				return fmt.Errorf("server %q not found in config — add it under 'servers:' first", alias)
			}

			token, err := promptJiraCredentials(rt.UI)
			if err != nil {
				return err
			}

			if err := creds.Set(alias, token); err != nil {
				return fmt.Errorf("storing token: %w", err)
			}

			fmt.Fprintf(rt.Out, "Token stored for server %q (%s)\n", alias, srv.URL)
			return nil
		},
	})

	authCmd.AddCommand(&cobra.Command{
		Use:   "logout <server-alias>",
		Short: "Remove the stored token for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			creds := getCredStore(cmd)
			alias := args[0]

			if err := creds.Delete(alias); err != nil {
				return fmt.Errorf("removing token: %w", err)
			}
			fmt.Fprintf(rt.Out, "Token removed for server %q\n", alias)
			return nil
		},
	})

	authCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show token status for all configured servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := getRuntime(cmd)
			creds := getCredStore(cmd)
			servers := getServers(cmd)

			if len(servers) == 0 {
				fmt.Fprintln(rt.Out, "No servers configured.")
				return nil
			}

			for alias, srv := range servers {
				_, err := creds.Get(alias)
				status := "[no token]"
				if err == nil {
					status = "[token stored]"
				}
				fmt.Fprintf(rt.Out, "  %-24s %-40s %s\n", alias, srv.URL, status)
			}
			return nil
		},
	})
	root.AddCommand(authCmd)

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

// resolveBootstrapServer determines the Jira server URL, alias, and token
// for the bootstrap command. If existing servers are configured, the user
// can pick one; otherwise they are prompted for a new URL and token.
func resolveBootstrapServer(rt *commands.Runtime, creds auth.CredentialStore) (serverURL, alias, token string, err error) {
	type serverInfo struct {
		alias string
		url   string
	}

	// Collect unique servers from existing workspaces.
	seen := make(map[string]bool)
	var existing []serverInfo
	for _, ws := range rt.Workspaces {
		if ws.ServerAlias != "" && !seen[ws.ServerAlias] && ws.Provider == core.ProviderJira {
			seen[ws.ServerAlias] = true
			existing = append(existing, serverInfo{alias: ws.ServerAlias, url: ws.BaseURL})
		}
	}

	if len(existing) > 0 {
		// Offer existing servers plus an "add new" option.
		options := make([]string, 0, len(existing)+1)
		for _, s := range existing {
			options = append(options, fmt.Sprintf("%s (%s)", s.alias, s.url))
		}
		options = append(options, "Add new server")

		choice, selErr := rt.UI.Select("Which Jira server?", options)
		if selErr != nil {
			return "", "", "", selErr
		}
		if choice < 0 {
			return "", "", "", fmt.Errorf("bootstrap cancelled")
		}

		if choice < len(existing) {
			// Use selected existing server.
			picked := existing[choice]
			serverURL = picked.url
			alias = picked.alias

			// Check if we already have a token stored.
			token, err = creds.Get(alias)
			if err == nil {
				return serverURL, alias, token, nil
			}
			// No stored token — prompt for one.
			token, err = promptJiraCredentials(rt.UI)
			if err != nil {
				return "", "", "", err
			}
			if storeErr := creds.Set(alias, token); storeErr != nil {
				return "", "", "", fmt.Errorf("storing token: %w", storeErr)
			}
			return serverURL, alias, token, nil
		}
		// Fall through to "add new server" below.
	}

	// No existing servers or user chose "add new".
	serverURL, err = rt.UI.PromptText("Jira Server URL (e.g., https://company.atlassian.net)")
	if err != nil {
		return "", "", "", err
	}
	if serverURL == "" {
		return "", "", "", fmt.Errorf("server URL is required")
	}
	serverURL = strings.TrimRight(serverURL, "/")

	alias = jira.ServerAliasFromURL(serverURL)

	token, err = promptJiraCredentials(rt.UI)
	if err != nil {
		return "", "", "", err
	}

	if storeErr := creds.Set(alias, token); storeErr != nil {
		return "", "", "", fmt.Errorf("storing token: %w", storeErr)
	}

	return serverURL, alias, token, nil
}

// promptJiraCredentials asks for an email and API token, then returns the
// base64-encoded Basic auth credential. This keeps the encoding detail out
// of the user's hands.
func promptJiraCredentials(ui commands.UI) (string, error) {
	email, err := ui.PromptText("Jira email address")
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	apiToken, err := ui.PromptSecret("Jira API token")
	if err != nil {
		return "", err
	}
	if apiToken == "" {
		return "", fmt.Errorf("API token is required")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
	return encoded, nil
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
	credStoreCtxKey      ctxKey = "ihj_cred_store"
	serversCtxKey        ctxKey = "ihj_servers"
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

func contextWithCredStore(ctx context.Context, creds auth.CredentialStore) context.Context {
	return context.WithValue(ctx, credStoreCtxKey, creds)
}

func getCredStore(cmd *cobra.Command) auth.CredentialStore {
	c, _ := cmd.Context().Value(credStoreCtxKey).(auth.CredentialStore)
	return c
}

func contextWithServers(ctx context.Context, servers map[string]rawServer) context.Context {
	return context.WithValue(ctx, serversCtxKey, servers)
}

func getServers(cmd *cobra.Command) map[string]rawServer {
	s, _ := cmd.Context().Value(serversCtxKey).(map[string]rawServer)
	return s
}
