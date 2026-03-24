package config

import (
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
		t.Errorf("BuildJQL() = %q; want %q", jql, `project = "FOO" AND cf[15000] = "uuid-123"`)
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
		t.Errorf("BuildJQL() = %q; want substring %q", jql, `(project = "FOO")`)
	}
	if !strings.Contains(jql, `(status IN ("To Do", "In Progress"))`) {
		t.Errorf("BuildJQL() = %q; want substring %q", jql, `(status IN ("To Do", "In Progress"))`)
	}
	if !strings.Contains(jql, "ORDER BY created DESC") {
		t.Errorf("BuildJQL() = %q; want substring \"ORDER BY created DESC\"", jql)
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
		t.Errorf("BuildJQL() error = %v; want substring \"nonexistent\"", err)
	}
}

func TestBuildJQL_EmptyBase(t *testing.T) {
	board := &BoardConfig{Slug: "test", JQL: ""}
	_, err := BuildJQL(board, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for empty JQL")
	}
}

func TestCombineJQL(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		filter string
		want   string
	}{
		{"with ORDER BY", "project = FOO ORDER BY key ASC", "status = Open", "(project = FOO) AND (status = Open) ORDER BY key ASC"},
		{"without ORDER BY", "project = FOO", "status = Open", "(project = FOO) AND (status = Open)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := combineJQL(tt.base, tt.filter); got != tt.want {
				t.Errorf("combineJQL(%q, %q) = %q; want %q", tt.base, tt.filter, got, tt.want)
			}
		})
	}
}
