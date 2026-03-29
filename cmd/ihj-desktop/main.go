// Command ihj-desktop launches the Wails v2 desktop GUI for ihj.
//
// It reuses the same config loading and session creation as the CLI,
// but renders the UI via a WebView window instead of a terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	fe "github.com/mikecsmith/ihj/cmd/ihj-desktop/frontend"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/demo"
	"github.com/mikecsmith/ihj/internal/desktop"
	"github.com/mikecsmith/ihj/internal/jira"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configDir, configFile, cacheDir := defaultPaths()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	theme, _, defaultWorkspace, workspaces, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	for _, ws := range workspaces {
		if ws.Provider == core.ProviderJira {
			if _, err := jira.HydrateWorkspace(ws); err != nil {
				return fmt.Errorf("hydrating workspace '%s': %w", ws.Slug, err)
			}
		}
	}

	ui := desktop.NewDesktopUI()

	rt := &commands.Runtime{
		Theme:            theme,
		DefaultWorkspace: defaultWorkspace,
		Workspaces:       workspaces,
		UI:               ui,
		CacheDir:         cacheDir,
		Out:              os.Stdout,
		Err:              os.Stderr,
	}

	factory := func(slug string) (*commands.WorkspaceSession, error) {
		ws, err := rt.ResolveWorkspace(slug)
		if err != nil {
			return nil, err
		}
		provider, err := newProviderForWorkspace(ws, cacheDir)
		if err != nil {
			return nil, err
		}
		return &commands.WorkspaceSession{
			Runtime:   rt,
			Workspace: ws,
			Provider:  provider,
		}, nil
	}

	sess, err := factory(defaultWorkspace)
	if err != nil {
		return fmt.Errorf("creating default session: %w", err)
	}

	app := desktop.NewApp(sess, factory, rt, ui)

	return wails.Run(&options.App{
		Title:    "ihj — " + sess.Workspace.Name,
		Width:    1280,
		Height:   800,
		MinWidth: 800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: fe.Assets,
		},
		OnStartup:  app.Startup,
		OnDomReady: app.DomReady,
		Bind: []interface{}{
			app,
		},
	})
}

// ── Config loading (mirrors cmd/ihj/main.go) ──

func defaultPaths() (configDir, configFile, cacheDir string) {
	home, _ := os.UserHomeDir()
	configDir = filepath.Join(home, ".config", "ihj")
	configFile = filepath.Join(configDir, "config.yaml")
	cacheDir = filepath.Join(home, ".local", "state", "ihj")
	return
}

type rawConfig struct {
	Theme            string                  `yaml:"theme"`
	Editor           string                  `yaml:"editor"`
	DefaultWorkspace string                  `yaml:"default_workspace"`
	Workspaces       map[string]rawWorkspace `yaml:"workspaces"`
}

type rawWorkspace struct {
	Provider string            `yaml:"provider"`
	Name     string            `yaml:"name"`
	Types    []rawTypeConfig   `yaml:"types"`
	Statuses []string          `yaml:"statuses"`
	Filters  map[string]string `yaml:"filters"`
}

type rawTypeConfig struct {
	ID          int    `yaml:"id"`
	Name        string `yaml:"name"`
	Order       int    `yaml:"order"`
	Color       string `yaml:"color"`
	HasChildren bool   `yaml:"has_children"`
	Template    string `yaml:"template,omitempty"`
}

func loadConfig(path string) (theme, editor, defaultWorkspace string, workspaces map[string]*core.Workspace, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("reading config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", "", "", nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(raw.Workspaces) == 0 {
		return "", "", "", nil, fmt.Errorf("missing 'workspaces' in config")
	}

	var fullConfig map[string]any
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return "", "", "", nil, fmt.Errorf("re-parsing config: %w", err)
	}

	workspacesRaw, _ := fullConfig["workspaces"].(map[string]any)

	universalKeys := map[string]bool{
		"provider": true, "name": true, "types": true, "statuses": true, "filters": true,
	}

	workspaces = make(map[string]*core.Workspace, len(raw.Workspaces))

	for slug, rws := range raw.Workspaces {
		if rws.Provider == "" {
			return "", "", "", nil, fmt.Errorf("workspace '%s' is missing 'provider' field", slug)
		}

		types := make([]core.TypeConfig, len(rws.Types))
		for i, t := range rws.Types {
			types[i] = core.TypeConfig{
				ID:          t.ID,
				Name:        t.Name,
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
				Template:    t.Template,
			}
		}

		typeOrderMap := make(map[string]core.TypeOrderEntry, len(types))
		for _, t := range types {
			typeOrderMap[t.Name] = core.TypeOrderEntry{
				Order:       t.Order,
				Color:       t.Color,
				HasChildren: t.HasChildren,
			}
		}

		statusWeights := make(map[string]int, len(rws.Statuses))
		for i, s := range rws.Statuses {
			statusWeights[strings.ToLower(s)] = i
		}

		providerCfg := make(map[string]any)
		if wsMap, ok := workspacesRaw[slug].(map[string]any); ok {
			for k, v := range wsMap {
				if !universalKeys[k] {
					providerCfg[k] = v
				}
			}
		}

		workspaces[slug] = &core.Workspace{
			Slug:           slug,
			Name:           rws.Name,
			Provider:       rws.Provider,
			Types:          types,
			Statuses:       rws.Statuses,
			Filters:        rws.Filters,
			StatusWeights:  statusWeights,
			TypeOrderMap:   typeOrderMap,
			ProviderConfig: providerCfg,
		}
	}

	return raw.Theme, raw.Editor, raw.DefaultWorkspace, workspaces, nil
}

func newProviderForWorkspace(ws *core.Workspace, cacheDir string) (core.Provider, error) {
	switch ws.Provider {
	case core.ProviderJira:
		token := os.Getenv("JIRA_BASIC_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("JIRA_BASIC_TOKEN environment variable not set")
		}
		jiraCfg, _ := ws.ProviderConfig.(*jira.Config)
		client := jira.New(
			jiraCfg.Server,
			token,
			jira.WithContext(context.Background()),
		)
		return jira.NewProvider(client, ws, cacheDir), nil

	case core.ProviderDemo:
		items := demo.Issues()
		return demo.NewProvider(items, 150*time.Millisecond), nil

	default:
		return nil, fmt.Errorf("unsupported provider %q for workspace %q", ws.Provider, ws.Slug)
	}
}
