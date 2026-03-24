package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBoard(t *testing.T) {
	cfg := &Config{
		DefaultBoard: "main",
		Boards: map[string]*BoardConfig{
			"main":  {Name: "Main Board"},
			"other": {Name: "Other Board"},
		},
	}

	b, err := cfg.ResolveBoard("other")
	if err != nil {
		t.Fatalf("ResolveBoard(\"other\") error = %v; want nil", err)
	}
	if b.Name != "Other Board" {
		t.Errorf("ResolveBoard(\"other\").Name = %q; want \"Other Board\"", b.Name)
	}

	b, err = cfg.ResolveBoard("")
	if err != nil {
		t.Fatalf("ResolveBoard(\"\") error = %v; want nil", err)
	}
	if b.Name != "Main Board" {
		t.Errorf("ResolveBoard(\"\").Name = %q; want \"Main Board\"", b.Name)
	}

	_, err = cfg.ResolveBoard("missing")
	if err == nil {
		t.Error("ResolveBoard(\"missing\") error = nil; want non-nil")
	}
}

func TestResolveFilter(t *testing.T) {
	cfg := &Config{DefaultFilter: "active"}
	if got := cfg.ResolveFilter("me"); got != "me" {
		t.Errorf("ResolveFilter(\"me\") = %q; want \"me\"", got)
	}
	if got := cfg.ResolveFilter(""); got != "active" {
		t.Errorf("ResolveFilter(\"\") = %q; want \"active\"", got)
	}

	cfg2 := &Config{}
	if got := cfg2.ResolveFilter(""); got != "active" {
		t.Errorf("ResolveFilter(\"\") = %q; want \"active\" (fallback)", got)
	}
}

func TestEditorCommand(t *testing.T) {
	cfg := &Config{Editor: "nvim"}
	if got := cfg.EditorCommand(); got != "nvim" {
		t.Errorf("EditorCommand() = %q; want \"nvim\"", got)
	}

	cfg2 := &Config{}
	t.Setenv("EDITOR", "nano")
	if got := cfg2.EditorCommand(); got != "nano" {
		t.Errorf("EditorCommand() = %q; want \"nano\" ($EDITOR fallback)", got)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
server: "https://jira.example.com"
default_board: "eng"
custom_fields:
  team: 15000
  epic_name: 10009
boards:
  eng:
    id: 1
    name: "Engineering"
    project_key: "ENG"
    jql: 'project = "{project_key}"'
    filters:
      active: 'status != Done'
    transitions:
      - "To Do"
      - "Done"
    types:
      - id: 10
        name: "Story"
        order: 30
        color: "blue"
        has_children: true
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server != "https://jira.example.com" {
		t.Errorf("Config.Server = %q; want \"https://jira.example.com\"", cfg.Server)
	}
	if cfg.FormattedCustomFields["team"] != "cf[15000]" {
		t.Errorf("FormattedCustomFields[\"team\"] = %q; want \"cf[15000]\"", cfg.FormattedCustomFields["team"])
	}
	if cfg.FormattedCustomFields["team_id"] != "customfield_15000" {
		t.Errorf("FormattedCustomFields[\"team_id\"] = %q; want \"customfield_15000\"", cfg.FormattedCustomFields["team_id"])
	}

	board := cfg.Boards["eng"]
	if board.Slug != "eng" {
		t.Errorf("Board.Slug = %q; want \"eng\"", board.Slug)
	}
	if _, ok := board.TypeOrderMap["10"]; !ok {
		t.Error("Board.TypeOrderMap[\"10\"] not found; want entry for id 10")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load(\"/nonexistent/config.yaml\") error = nil; want non-nil")
	}
}

func TestLoadOrEmpty_MissingFile(t *testing.T) {
	cfg, err := LoadOrEmpty("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Boards == nil {
		t.Error("LoadOrEmpty().Boards = nil; want initialized map")
	}
}

func TestValidate_MissingTypes(t *testing.T) {
	cfg := &Config{
		CustomFields: map[string]int{"team": 15000},
		Boards: map[string]*BoardConfig{
			"test": {JQL: "project = FOO", Types: nil},
		},
	}
	if err := cfg.validate(); err == nil {
		t.Error("validate() error = nil; want non-nil for missing types")
	}
}
