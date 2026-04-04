package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

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

	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got.Theme != "dark" {
		t.Errorf("theme = %q, want 'dark'", got.Theme)
	}
	if got.Editor != "nvim" {
		t.Errorf("editor = %q, want 'nvim'", got.Editor)
	}
	if got.DefaultWorkspace != "myproject" {
		t.Errorf("default_workspace = %q, want 'myproject'", got.DefaultWorkspace)
	}

	ws, ok := got.Workspaces["myproject"]
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

	// VimMode defaults to false when not configured.
	if got.VimMode {
		t.Error("VimMode should default to false")
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
			cfg, err := loadConfig(path)
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			if cfg.Workspaces["fast"].CacheTTL != tt.wantFast {
				t.Errorf("fast.CacheTTL = %v, want %v", cfg.Workspaces["fast"].CacheTTL, tt.wantFast)
			}
			if cfg.Workspaces["slow"].CacheTTL != tt.wantSlow {
				t.Errorf("slow.CacheTTL = %v, want %v", cfg.Workspaces["slow"].CacheTTL, tt.wantSlow)
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
			_, err := loadConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_Guidance_PriorityChain(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantAlpha string
		wantBeta  string
	}{
		{
			name: "workspace overrides global",
			yaml: `
guidance: |
  Global guidance text
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  alpha:
    server: s
    name: Alpha
    guidance: |
      Custom alpha guidance
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  beta:
    server: s
    name: Beta
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantAlpha: "Custom alpha guidance\n",
			wantBeta:  "Global guidance text\n",
		},
		{
			name: "global applies to all workspaces",
			yaml: `
guidance: "Be concise"
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  alpha:
    server: s
    name: Alpha
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  beta:
    server: s
    name: Beta
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantAlpha: "Be concise",
			wantBeta:  "Be concise",
		},
		{
			name: "no guidance configured uses empty",
			yaml: `
servers:
  s:
    provider: demo
    url: https://x.com
workspaces:
  alpha:
    server: s
    name: Alpha
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
  beta:
    server: s
    name: Beta
    types: [{id: 1, name: T, order: 1}]
    statuses: [{name: Open, order: 10, color: default}]
`,
			wantAlpha: "",
			wantBeta:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig(path)
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			if cfg.Workspaces["alpha"].Guidance != tt.wantAlpha {
				t.Errorf("alpha.Guidance = %q, want %q", cfg.Workspaces["alpha"].Guidance, tt.wantAlpha)
			}
			if cfg.Workspaces["beta"].Guidance != tt.wantBeta {
				t.Errorf("beta.Guidance = %q, want %q", cfg.Workspaces["beta"].Guidance, tt.wantBeta)
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

	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	ws := got.Workspaces["eng"]
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
			_, err := loadConfig(path)
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
	_, err := loadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigOrEmpty_MissingFile(t *testing.T) {
	cfg, err := loadConfigOrEmpty("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != "" || cfg.Editor != "" || cfg.DefaultWorkspace != "" {
		t.Error("expected empty strings for missing config")
	}
	if cfg.Workspaces == nil {
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

	got, err := loadConfigOrEmpty(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got.Workspaces["test"]; !ok {
		t.Error("expected 'test' workspace")
	}
}

func TestLoadConfig_TypeExtraFields(t *testing.T) {
	yamlCfg := `
servers:
  s:
    provider: jira
    url: https://x.com
workspaces:
  w:
    server: s
    name: W
    types:
      - id: 1
        name: Task
        order: 1
        fields:
          story_points: 10016
          environment: 10022
      - id: 2
        name: Epic
        order: 2
    statuses: [{name: Open, order: 10, color: default}]
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yamlCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	ws := got.Workspaces["w"]

	// Task type has extra fields.
	task := ws.Types[0]
	if task.Name != "Task" {
		t.Fatalf("Types[0].Name = %q, want Task", task.Name)
	}
	if len(task.ExtraFields) != 2 {
		t.Fatalf("Task.ExtraFields len = %d, want 2", len(task.ExtraFields))
	}
	if task.ExtraFields["story_points"] != 10016 {
		t.Errorf("story_points = %d, want 10016", task.ExtraFields["story_points"])
	}
	if task.ExtraFields["environment"] != 10022 {
		t.Errorf("environment = %d, want 10022", task.ExtraFields["environment"])
	}

	// Epic type has no extra fields.
	epic := ws.Types[1]
	if epic.Name != "Epic" {
		t.Fatalf("Types[1].Name = %q, want Epic", epic.Name)
	}
	if epic.ExtraFields != nil {
		t.Errorf("Epic.ExtraFields = %v, want nil", epic.ExtraFields)
	}
}

func TestLoadConfig_VimModeEnabled(t *testing.T) {
	yaml := `
vim_mode: true
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
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !got.VimMode {
		t.Error("VimMode should be true")
	}
}
