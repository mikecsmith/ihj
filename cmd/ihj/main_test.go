package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

// stubLauncher records whether LaunchUI was called and captures the data.
type stubLauncher struct {
	called bool
	data   *commands.LaunchUIData
}

func (l *stubLauncher) LaunchUI(data *commands.LaunchUIData) error {
	l.called = true
	l.data = data
	return nil
}

// testRun calls run() with injected test dependencies and no config file.
func testRun(t *testing.T, args []string, ui commands.UI, launcher commands.UILauncher) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	tmp := t.TempDir()

	origArgs := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = origArgs })

	err := run(
		&stdout, &stderr,
		filepath.Join(tmp, "config"),
		filepath.Join(tmp, "config", "config.yaml"),
		filepath.Join(tmp, "cache"),
		ui, launcher, nil,
	)
	return &stdout, &stderr, err
}

// testRunWithConfig writes a config file then calls run().
func testRunWithConfig(t *testing.T, args []string, configYAML string, ui commands.UI, launcher commands.UILauncher) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	configFile := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	origArgs := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = origArgs })

	err := run(
		&stdout, &stderr,
		configDir, configFile,
		filepath.Join(tmp, "cache"),
		ui, launcher, nil,
	)
	return &stdout, &stderr, err
}

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		envEditor  string
		want       string
	}{
		{"configured takes precedence", "code", "nvim", "code"},
		{"falls back to EDITOR", "", "nvim", "nvim"},
		{"falls back to vim", "", "", "vim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", tt.envEditor)
			got := editorCommand(tt.configured)
			if got != tt.want {
				t.Errorf("editorCommand(%q) = %q, want %q", tt.configured, got, tt.want)
			}
		})
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")

	if err := ensureDirs(nested); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	// Idempotent.
	if err := ensureDirs(nested); err != nil {
		t.Fatalf("ensureDirs (idempotent): %v", err)
	}
}

func TestLoadConfig_ValidDemoWorkspace(t *testing.T) {
	cfg := `
theme: dark
editor: nvim
default_workspace: myproject
servers:
  demo-server:
    provider: demo
    url: https://demo.example.com
workspaces:
  myproject:
    server: demo-server
    name: My Project
    types:
      - id: 1
        name: Story
        order: 1
        color: "#00ff00"
      - id: 2
        name: Task
        order: 2
        color: "#0000ff"
        has_children: true
    statuses:
      - name: To Do
        order: 10
        color: cyan
      - name: In Progress
        order: 20
        color: blue
      - name: Done
        order: 30
        color: green
    filters:
      active: "status != Done"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	theme, editor, defaultWs, _, workspaces, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if theme != "dark" {
		t.Errorf("theme = %q, want 'dark'", theme)
	}
	if editor != "nvim" {
		t.Errorf("editor = %q, want 'nvim'", editor)
	}
	if defaultWs != "myproject" {
		t.Errorf("default_workspace = %q, want 'myproject'", defaultWs)
	}

	ws, ok := workspaces["myproject"]
	if !ok {
		t.Fatal("workspace 'myproject' not found")
	}
	if ws.Name != "My Project" {
		t.Errorf("ws.Name = %q", ws.Name)
	}
	if ws.Provider != "demo" {
		t.Errorf("ws.Provider = %q, want 'demo'", ws.Provider)
	}
	if ws.ServerAlias != "demo-server" {
		t.Errorf("ws.ServerAlias = %q, want 'demo-server'", ws.ServerAlias)
	}
	if ws.BaseURL != "https://demo.example.com" {
		t.Errorf("ws.BaseURL = %q", ws.BaseURL)
	}
	if len(ws.Types) != 2 {
		t.Fatalf("len(Types) = %d, want 2", len(ws.Types))
	}
	if ws.Types[0].Name != "Story" {
		t.Errorf("Types[0].Name = %q", ws.Types[0].Name)
	}
	if !ws.Types[1].HasChildren {
		t.Error("Types[1].HasChildren should be true")
	}
	if len(ws.Statuses) != 3 {
		t.Errorf("len(Statuses) = %d, want 3", len(ws.Statuses))
	}

	// StatusOrderMap populated.
	if entry, ok := ws.StatusOrderMap["to do"]; !ok || entry.Weight != 10 {
		t.Errorf("StatusOrderMap['to do'] = %+v", ws.StatusOrderMap["to do"])
	}
	if entry, ok := ws.StatusOrderMap["done"]; !ok || entry.Weight != 30 {
		t.Errorf("StatusOrderMap['done'] = %+v", ws.StatusOrderMap["done"])
	}

	// TypeOrderMap populated.
	if entry, ok := ws.TypeOrderMap["story"]; !ok || entry.Order != 1 {
		t.Errorf("TypeOrderMap['story'] = %+v", ws.TypeOrderMap["story"])
	}

	// CacheTTL defaults to DefaultCacheTTL when not configured.
	if ws.CacheTTL != core.DefaultCacheTTL {
		t.Errorf("CacheTTL = %v, want %v", ws.CacheTTL, core.DefaultCacheTTL)
	}

	// Filters preserved.
	if ws.Filters["active"] != "status != Done" {
		t.Errorf("Filters['active'] = %q", ws.Filters["active"])
	}
}

func TestLoadConfig_CacheTTL_PriorityChain(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantFast time.Duration
		wantSlow time.Duration
	}{
		{
			name: "workspace overrides global",
			yaml: `
cache_ttl: 10m
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  fast:
    server: s
    name: Fast
    cache_ttl: 2m
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  slow:
    server: s
    name: Slow
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantFast: 2 * time.Minute,
			wantSlow: 10 * time.Minute,
		},
		{
			name: "global overrides default",
			yaml: `
cache_ttl: 5m
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  fast:
    server: s
    name: Fast
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  slow:
    server: s
    name: Slow
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantFast: 5 * time.Minute,
			wantSlow: 5 * time.Minute,
		},
		{
			name: "no config uses default",
			yaml: `
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  fast:
    server: s
    name: Fast
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  slow:
    server: s
    name: Slow
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantFast: core.DefaultCacheTTL,
			wantSlow: core.DefaultCacheTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, _, _, workspaces, err := loadConfig(path)
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			if workspaces["fast"].CacheTTL != tt.wantFast {
				t.Errorf("fast.CacheTTL = %v, want %v", workspaces["fast"].CacheTTL, tt.wantFast)
			}
			if workspaces["slow"].CacheTTL != tt.wantSlow {
				t.Errorf("slow.CacheTTL = %v, want %v", workspaces["slow"].CacheTTL, tt.wantSlow)
			}
		})
	}
}

func TestLoadConfig_CacheTTL_InvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "invalid global",
			yaml: `
cache_ttl: banana
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  w:
    server: s
    name: W
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantErr: "invalid global cache_ttl",
		},
		{
			name: "invalid workspace",
			yaml: `
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  w:
    server: s
    name: W
    cache_ttl: not-a-duration
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantErr: "invalid cache_ttl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, _, _, _, err := loadConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_ProviderSpecificFields(t *testing.T) {
	cfg := `
servers:
  company-jira:
    provider: jira
    url: https://company.atlassian.net
workspaces:
  eng:
    server: company-jira
    name: Engineering
    project: ENG
    board_id: 42
    types:
      - id: 10001
        name: Story
        order: 1
        color: green
    statuses:
      - name: Open
        order: 10
        color: default
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, _, workspaces, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	ws := workspaces["eng"]
	if ws.Provider != "jira" {
		t.Errorf("ws.Provider = %q, want 'jira'", ws.Provider)
	}
	if ws.ServerAlias != "company-jira" {
		t.Errorf("ws.ServerAlias = %q, want 'company-jira'", ws.ServerAlias)
	}
	if ws.BaseURL != "https://company.atlassian.net" {
		t.Errorf("ws.BaseURL = %q", ws.BaseURL)
	}

	provCfg, ok := ws.ProviderConfig.(map[string]any)
	if !ok {
		t.Fatalf("ProviderConfig type = %T, want map[string]any", ws.ProviderConfig)
	}

	if provCfg["project"] != "ENG" {
		t.Errorf("project = %v", provCfg["project"])
	}
	// Universal keys must not leak into provider config.
	for _, k := range []string{"server", "name", "types", "statuses", "filters"} {
		if _, exists := provCfg[k]; exists {
			t.Errorf("universal key %q leaked into ProviderConfig", k)
		}
	}
}

func TestLoadConfig_Errors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing workspaces",
			yaml:    "theme: dark\nservers:\n  s:\n    provider: demo\n    url: https://x.com\n",
			wantErr: "missing 'workspaces'",
		},
		{
			name:    "missing servers",
			yaml:    "workspaces:\n  x:\n    server: s\n    name: X\n    types:\n      - {id: 1, name: T, order: 1}\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "missing 'servers'",
		},
		{
			name:    "missing server on workspace",
			yaml:    "servers:\n  s:\n    provider: demo\n    url: https://x.com\nworkspaces:\n  x:\n    name: X\n    types:\n      - {id: 1, name: T, order: 1}\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "missing 'server'",
		},
		{
			name:    "unknown server alias",
			yaml:    "servers:\n  s:\n    provider: demo\n    url: https://x.com\nworkspaces:\n  x:\n    server: unknown\n    name: X\n    types:\n      - {id: 1, name: T, order: 1}\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "unknown server",
		},
		{
			name:    "missing types",
			yaml:    "servers:\n  s:\n    provider: demo\n    url: https://x.com\nworkspaces:\n  x:\n    server: s\n    name: X\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "missing 'types'",
		},
		{
			name:    "server missing provider",
			yaml:    "servers:\n  s:\n    url: https://x.com\nworkspaces:\n  x:\n    server: s\n    name: X\n    types:\n      - {id: 1, name: T, order: 1}\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "missing 'provider'",
		},
		{
			name:    "server missing url",
			yaml:    "servers:\n  s:\n    provider: demo\nworkspaces:\n  x:\n    server: s\n    name: X\n    types:\n      - {id: 1, name: T, order: 1}\n    statuses: [{name: Open, order: 10, color: default}]\n",
			wantErr: "missing 'url'",
		},
		{
			name:    "invalid yaml",
			yaml:    "workspaces:\n  - this is a list not a map",
			wantErr: "parsing config YAML",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, _, _, _, err := loadConfig(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, _, _, _, _, err := loadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigOrEmpty_MissingFile(t *testing.T) {
	theme, editor, defaultWs, _, workspaces, err := loadConfigOrEmpty("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if theme != "" || editor != "" || defaultWs != "" {
		t.Error("expected empty strings for missing config")
	}
	if workspaces == nil {
		t.Error("expected non-nil (empty) workspaces map")
	}
}

func TestLoadConfigOrEmpty_ExistingFile(t *testing.T) {
	cfg := `
servers:
  demo-srv:
    provider: demo
    url: https://demo.example.com
workspaces:
  test:
    server: demo-srv
    name: Test
    types:
      - {id: 1, name: Task, order: 1, color: blue}
    statuses: [{name: Open, order: 10, color: default}]
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, _, workspaces, err := loadConfigOrEmpty(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := workspaces["test"]; !ok {
		t.Error("expected 'test' workspace")
	}
}

func TestNewProviderForWorkspace_Demo(t *testing.T) {
	ws := &core.Workspace{
		Slug:     "demo",
		Provider: core.ProviderDemo,
	}
	creds := testutil.NewMockCredentialStore()
	provider, client, err := newProviderForWorkspace(ws, t.TempDir(), creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Error("expected non-nil provider")
	}
	if client != nil {
		t.Error("expected nil client for demo provider")
	}
}

func TestNewProviderForWorkspace_UnsupportedProvider(t *testing.T) {
	ws := &core.Workspace{
		Slug:     "test",
		Provider: "unsupported",
	}
	creds := testutil.NewMockCredentialStore()
	_, _, err := newProviderForWorkspace(ws, t.TempDir(), creds)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("error = %q", err)
	}
}

func TestNewProviderForWorkspace_JiraMissingToken(t *testing.T) {
	ws := &core.Workspace{
		Slug:        "eng",
		Provider:    core.ProviderJira,
		ServerAlias: "test-jira",
		BaseURL:     "https://test.atlassian.net",
	}
	creds := testutil.NewMockCredentialStore() // no token stored
	_, _, err := newProviderForWorkspace(ws, t.TempDir(), creds)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "no token found") {
		t.Errorf("error = %q", err)
	}
}

func TestNewProviderForWorkspace_JiraNilConfig(t *testing.T) {
	ws := &core.Workspace{
		Slug:           "eng",
		Provider:       core.ProviderJira,
		ServerAlias:    "test-jira",
		BaseURL:        "https://test.atlassian.net",
		ProviderConfig: map[string]any{"server": "https://example.com"},
	}
	creds := testutil.NewMockCredentialStore()
	creds.Tokens["test-jira"] = "dGVzdDp0ZXN0" // token exists but config not hydrated
	_, _, err := newProviderForWorkspace(ws, t.TempDir(), creds)
	if err == nil {
		t.Fatal("expected error for non-*jira.Config ProviderConfig")
	}
	if !strings.Contains(err.Error(), "no Jira configuration") {
		t.Errorf("error = %q", err)
	}
}

func TestRun_DemoMode(t *testing.T) {
	ui := &testutil.MockUI{}
	launcher := &stubLauncher{}

	_, _, err := testRun(t, []string{"ihj", "jira", "demo"}, ui, launcher)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !launcher.called {
		t.Error("expected launcher.LaunchUI to be called")
	}
	if launcher.data == nil {
		t.Fatal("expected non-nil launch data")
	}
	if launcher.data.Filter != "active" {
		t.Errorf("filter = %q, want 'active'", launcher.data.Filter)
	}
	if launcher.data.Workspace.Provider != core.ProviderDemo {
		t.Errorf("provider = %q, want 'demo'", launcher.data.Workspace.Provider)
	}
	if len(launcher.data.Items) == 0 {
		t.Error("expected demo items")
	}
}

func TestRun_MissingConfig(t *testing.T) {
	_, _, err := testRun(t, []string{"ihj", "export"}, &testutil.MockUI{}, &stubLauncher{})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Errorf("error = %q, want substring 'config not found'", err)
	}
}

func TestRun_BootstrapMissingConfig(t *testing.T) {
	_, _, err := testRun(t, []string{"ihj", "jira", "bootstrap", "ENG"}, &testutil.MockUI{}, &stubLauncher{})
	if err == nil {
		t.Fatal("expected error (mock UI returns empty server URL)")
	}
	// Should fail on "server URL is required", not on config loading.
	if strings.Contains(err.Error(), "config:") {
		t.Errorf("should not fail on config loading, got: %v", err)
	}
}

func TestRun_DemoWorkspaceConfig(t *testing.T) {
	cfg := `
default_workspace: myws
servers:
  demo-srv:
    provider: demo
    url: https://demo.example.com
workspaces:
  myws:
    server: demo-srv
    name: My Workspace
    types:
      - {id: 1, name: Story, order: 1, color: green}
    statuses: [{name: Open, order: 10, color: default}, {name: Done, order: 20, color: green}]
    filters:
      active: "status = Open"
`
	ui := &testutil.MockUI{}
	launcher := &stubLauncher{}

	_, _, err := testRunWithConfig(t, []string{"ihj", "tui", "-w", "myws"}, cfg, ui, launcher)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !launcher.called {
		t.Error("expected launcher to be called")
	}
	if launcher.data.Workspace.Slug != "myws" {
		t.Errorf("workspace slug = %q, want 'myws'", launcher.data.Workspace.Slug)
	}
}

func TestRun_EditorCmdCallback(t *testing.T) {
	var gotEditorCmd string
	var stdout, stderr bytes.Buffer
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	configFile := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origArgs := os.Args
	os.Args = []string{"ihj", "jira", "demo"}
	t.Cleanup(func() { os.Args = origArgs })

	err := run(&stdout, &stderr, configDir, configFile, filepath.Join(tmp, "cache"),
		&testutil.MockUI{}, &stubLauncher{}, func(cmd string) { gotEditorCmd = cmd })
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Demo mode doesn't load config, so editor falls back to $EDITOR/vim.
	// But the callback should still be invoked.
	if gotEditorCmd == "" {
		t.Error("setEditorCmd callback was not invoked")
	}
}
