package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// testRunWithConfigAndCaps writes a config file, calls run(), and captures applied UI caps.
func testRunWithConfigAndCaps(t *testing.T, args []string, configYAML string, ui commands.UI, launcher commands.UILauncher) (*bytes.Buffer, *bytes.Buffer, uiCaps, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var gotCaps uiCaps
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
		ui, launcher, func(caps uiCaps) { gotCaps = caps },
	)
	return &stdout, &stderr, gotCaps, err
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

func TestRun_UICapabilities(t *testing.T) {
	var gotCaps uiCaps
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
		&testutil.MockUI{}, &stubLauncher{}, func(caps uiCaps) { gotCaps = caps })
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Demo mode doesn't load config, so editor falls back to $EDITOR/vim.
	// But the callback should still be invoked.
	if gotCaps.EditorCmd == "" {
		t.Error("onConfig callback was not invoked (EditorCmd empty)")
	}
	if gotCaps.VimMode {
		t.Error("VimMode should be false for demo mode")
	}
}

func TestRun_VimModeFromConfig(t *testing.T) {
	cfg := `
vim_mode: true
editor: nvim
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
    statuses: [{name: Open, order: 10, color: default}]
`
	launcher := &stubLauncher{}
	_, _, gotCaps, err := testRunWithConfigAndCaps(t, []string{"ihj", "tui", "-w", "myws"}, cfg, &testutil.MockUI{}, launcher)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !gotCaps.VimMode {
		t.Error("VimMode should be true when vim_mode: true in config")
	}
	if gotCaps.EditorCmd != "nvim" {
		t.Errorf("EditorCmd = %q, want 'nvim'", gotCaps.EditorCmd)
	}
}
