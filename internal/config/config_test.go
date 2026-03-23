package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildJQL_BaseOnly(t *testing.T) {
	board := &BoardConfig{
		Slug:       "test",
		ProjectKey: "FOO",
		TeamUUID:   "uuid-123",
		JQL:        `project = "{project_key}" AND {team} = "{team_uuid}"`,
		Filters:    map[string]string{},
	}
	cf := map[string]string{"team": "cf[15000]", "team_id": "customfield_15000"}

	jql, err := BuildJQL(board, "", cf)
	if err != nil {
		t.Fatal(err)
	}
	if jql != `project = "FOO" AND cf[15000] = "uuid-123"` {
		t.Errorf("jql = %q", jql)
	}
}

func TestBuildJQL_WithFilter(t *testing.T) {
	board := &BoardConfig{
		Slug:       "test",
		ProjectKey: "FOO",
		JQL:        `project = "{project_key}" ORDER BY created DESC`,
		Filters: map[string]string{
			"active": `status IN ("To Do", "In Progress")`,
		},
	}
	cf := map[string]string{}

	jql, err := BuildJQL(board, "active", cf)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(jql, `(project = "FOO")`) {
		t.Errorf("missing base query in: %s", jql)
	}
	if !strings.Contains(jql, `(status IN ("To Do", "In Progress"))`) {
		t.Errorf("missing filter in: %s", jql)
	}
	if !strings.Contains(jql, "ORDER BY created DESC") {
		t.Errorf("missing ORDER BY in: %s", jql)
	}
}

func TestBuildJQL_UndefinedVariable(t *testing.T) {
	board := &BoardConfig{
		Slug: "test",
		JQL:  `project = "{nonexistent}"`,
	}
	_, err := BuildJQL(board, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %v, expected mention of 'nonexistent'", err)
	}
}

func TestBuildJQL_EmptyBase(t *testing.T) {
	board := &BoardConfig{Slug: "test", JQL: ""}
	_, err := BuildJQL(board, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for empty JQL")
	}
}

func TestCombineJQL_WithOrderBy(t *testing.T) {
	result := combineJQL("project = FOO ORDER BY key ASC", "status = Open")
	if result != "(project = FOO) AND (status = Open) ORDER BY key ASC" {
		t.Errorf("got: %s", result)
	}
}

func TestCombineJQL_WithoutOrderBy(t *testing.T) {
	result := combineJQL("project = FOO", "status = Open")
	if result != "(project = FOO) AND (status = Open)" {
		t.Errorf("got: %s", result)
	}
}

func TestResolveBoard(t *testing.T) {
	cfg := &Config{
		DefaultBoard: "main",
		Boards: map[string]*BoardConfig{
			"main":  {Name: "Main Board"},
			"other": {Name: "Other Board"},
		},
	}

	b, err := cfg.ResolveBoard("other")
	if err != nil || b.Name != "Other Board" {
		t.Errorf("explicit slug: got %v, err=%v", b, err)
	}

	b, err = cfg.ResolveBoard("")
	if err != nil || b.Name != "Main Board" {
		t.Errorf("default: got %v, err=%v", b, err)
	}

	_, err = cfg.ResolveBoard("missing")
	if err == nil {
		t.Error("expected error for missing board")
	}
}

func TestResolveFilter(t *testing.T) {
	cfg := &Config{DefaultFilter: "active"}
	if cfg.ResolveFilter("me") != "me" {
		t.Error("explicit filter not returned")
	}
	if cfg.ResolveFilter("") != "active" {
		t.Error("default filter not returned")
	}

	cfg2 := &Config{}
	if cfg2.ResolveFilter("") != "active" {
		t.Error("fallback 'active' not returned")
	}
}

func TestEditorCommand(t *testing.T) {
	cfg := &Config{Editor: "nvim"}
	if cfg.EditorCommand() != "nvim" {
		t.Error("expected nvim")
	}

	cfg2 := &Config{}
	t.Setenv("EDITOR", "nano")
	if cfg2.EditorCommand() != "nano" {
		t.Error("expected $EDITOR fallback")
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
		t.Errorf("server = %q", cfg.Server)
	}
	if cfg.FormattedCustomFields["team"] != "cf[15000]" {
		t.Errorf("formatted team = %q", cfg.FormattedCustomFields["team"])
	}
	if cfg.FormattedCustomFields["team_id"] != "customfield_15000" {
		t.Errorf("formatted team_id = %q", cfg.FormattedCustomFields["team_id"])
	}

	board := cfg.Boards["eng"]
	if board.Slug != "eng" {
		t.Errorf("slug = %q", board.Slug)
	}
	if _, ok := board.TypeOrderMap["10"]; !ok {
		t.Error("missing type order map entry for id 10")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadOrEmpty_MissingFile(t *testing.T) {
	cfg, err := LoadOrEmpty("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Boards == nil {
		t.Error("expected initialized boards map")
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
		t.Error("expected error for missing types")
	}
}
