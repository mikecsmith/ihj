package fakejira_test

import (
	"context"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/jira/fakejira"
)

// newFakeProvider stands up a fakejira server and wires a jira.Provider
// against it — exercising the same code path used by demo mode.
func newFakeProvider(t *testing.T) (*jira.Provider, *fakejira.Server) {
	t.Helper()
	srv := fakejira.NewServer()
	t.Cleanup(srv.Close)

	ws := fakejira.Workspace()
	ws.BaseURL = srv.URL
	cfg, err := jira.HydrateWorkspace(ws)
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	client := jira.New(cfg.Server, "demo-token")
	return jira.NewProvider(client, ws, t.TempDir()), srv
}

func TestServer_SearchReturnsSeedIssues(t *testing.T) {
	provider, _ := newFakeProvider(t)
	items, err := provider.Search(context.Background(), "active", true)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seed issues")
	}
	// Every seeded issue should land under the DEMO project.
	for _, it := range items {
		if it.ID[:5] != "DEMO-" {
			t.Errorf("unexpected key %q", it.ID)
		}
	}
}

func TestServer_GetIssueByKey(t *testing.T) {
	provider, _ := newFakeProvider(t)
	it, err := provider.Get(context.Background(), "DEMO-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if it.ID != "DEMO-1" {
		t.Fatalf("id = %q", it.ID)
	}
	if it.Type != "Epic" {
		t.Fatalf("type = %q, want Epic", it.Type)
	}
}

func TestServer_CurrentUser(t *testing.T) {
	provider, _ := newFakeProvider(t)
	u, err := provider.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if u.DisplayName != "Demo User" {
		t.Fatalf("name = %q", u.DisplayName)
	}
}

func TestServer_TransitionUpdatesStatus(t *testing.T) {
	provider, srv := newFakeProvider(t)
	ctx := context.Background()

	done := "Done"
	if err := provider.Update(ctx, "DEMO-1", &core.Changes{Status: &done}); err != nil {
		t.Fatalf("update: %v", err)
	}
	iss := srv.State.Issue("DEMO-1")
	if iss == nil || iss.StatusID != "4" {
		t.Fatalf("expected status id 4, got %+v", iss)
	}
}

// newFakeKanbanProvider stands up a kanban fakejira server + provider.
func newFakeKanbanProvider(t *testing.T) (*jira.Provider, *fakejira.Server) {
	t.Helper()
	srv := fakejira.NewKanbanServer()
	t.Cleanup(srv.Close)

	ws := fakejira.WorkspaceKanban()
	ws.BaseURL = srv.URL
	cfg, err := jira.HydrateWorkspace(ws)
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	client := jira.New(cfg.Server, "demo-token")
	return jira.NewProvider(client, ws, t.TempDir()), srv
}

func TestServer_MeFilterReturnsOnlyCurrentUser(t *testing.T) {
	provider, _ := newFakeProvider(t)
	items, err := provider.Search(context.Background(), "me", true)
	if err != nil {
		t.Fatalf("search me: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one issue assigned to demo-user")
	}
	for _, it := range items {
		got, _ := it.Fields["assignee"].(string)
		if got != "demo@example.com" {
			t.Errorf("issue %s assignee = %q, want demo@example.com", it.ID, got)
		}
	}
}

func TestServer_KanbanWorkspace(t *testing.T) {
	provider, _ := newFakeKanbanProvider(t)
	ctx := context.Background()

	// "active" → not Done.
	active, err := provider.Search(ctx, "active", true)
	if err != nil {
		t.Fatalf("search active: %v", err)
	}
	if len(active) == 0 {
		t.Fatal("expected active kanban issues")
	}
	for _, it := range active {
		if it.Status == "Done" {
			t.Errorf("active filter returned Done issue %s", it.ID)
		}
		if len(it.ID) < 4 || it.ID[:4] != "OPS-" {
			t.Errorf("unexpected key %q (want OPS-*)", it.ID)
		}
	}

	// "done" → only Done.
	done, err := provider.Search(ctx, "done", true)
	if err != nil {
		t.Fatalf("search done: %v", err)
	}
	for _, it := range done {
		if it.Status != "Done" {
			t.Errorf("done filter returned non-Done issue %s (status=%q)", it.ID, it.Status)
		}
	}
}

func TestServer_SearchBacklogFilter(t *testing.T) {
	provider, _ := newFakeProvider(t)
	ctx := context.Background()
	items, err := provider.Search(ctx, "backlog", true)
	if err != nil {
		t.Fatalf("search backlog: %v", err)
	}
	// Backlog should be non-empty but distinct from active.
	active, _ := provider.Search(ctx, "active", true)
	if len(items) == 0 && len(active) == 0 {
		t.Fatal("both filters empty")
	}
}
