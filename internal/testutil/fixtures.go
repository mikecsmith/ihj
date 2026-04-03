// Package testutil provides shared test fixtures, mock implementations,
// and helpers used across package test suites.
package testutil

import (
	"io"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// TestWorkspace returns a canonical workspace for testing.
// Includes types, statuses, and weights sufficient for both
// commands and TUI tests.
func TestWorkspace() *core.Workspace {
	return &core.Workspace{
		Slug:     "eng",
		Name:     "Engineering",
		Provider: "test",
		BaseURL:  "https://test.example.com",
		Filters:  map[string]string{"default": "status != Done"},
		Statuses: []core.StatusConfig{
			{Name: "Backlog", Order: 10, Color: "default"},
			{Name: "To Do", Order: 20, Color: "cyan"},
			{Name: "In Progress", Order: 30, Color: "blue"},
			{Name: "In Review", Order: 40, Color: "magenta"},
			{Name: "Done", Order: 50, Color: "green"},
		},
		Types: []core.TypeConfig{
			{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
			{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true,
				Template: "## Acceptance Criteria\n\n-\n"},
			{ID: 11, Name: "Task", Order: 30, Color: "default"},
			{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
			{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
		},
		StatusOrderMap: map[string]core.StatusOrderEntry{
			"backlog": {Weight: 10, Color: "default"}, "to do": {Weight: 20, Color: "cyan"},
			"in progress": {Weight: 30, Color: "blue"}, "in review": {Weight: 40, Color: "magenta"},
			"done": {Weight: 50, Color: "green"},
		},
		TypeOrderMap: map[string]core.TypeOrderEntry{
			"epic":     {Order: 20, Color: "magenta", HasChildren: true},
			"story":    {Order: 30, Color: "blue", HasChildren: true},
			"task":     {Order: 30, Color: "default"},
			"spike":    {Order: 30, Color: "yellow"},
			"sub-task": {Order: 40, Color: "white"},
		},
	}
}

// TestItems returns a standard set of work items for testing.
func TestItems() []*core.WorkItem {
	return []*core.WorkItem{
		{
			ID: "TEST-1", Summary: "Epic One", Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority": "High", "assignee": "Alice", "reporter": "Bob",
				"created": "1 Jan 2025", "updated": "15 Jan 2025",
			},
		},
		{
			ID: "TEST-2", Summary: "Story One", Type: "Story", Status: "In Progress",
			Fields: map[string]any{
				"priority": "Medium", "assignee": "Charlie", "reporter": "Alice",
				"created": "2 Jan 2025", "updated": "16 Jan 2025",
			},
		},
	}
}

// RichTestItems returns a larger set of work items with hierarchy,
// descriptions, display fields, and varied types/statuses. All values
// are fixed strings (no time.Now) for deterministic output.
// Returns both the flat slice and a linked registry.
func RichTestItems() ([]*core.WorkItem, map[string]*core.WorkItem) {
	md := func(text string) *document.Node {
		doc, _ := document.ParseMarkdownString(text)
		return doc
	}

	items := []*core.WorkItem{
		{
			ID: "ENG-100", Summary: "User Authentication Overhaul",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "sarah@example.com",
				"reporter":   "mike@example.com",
				"created":    "15 Jan 2025",
				"updated":    "28 Jan 2025",
				"labels":     []string{"security", "q1-priority"},
				"components": []string{"Auth"},
			},
			DisplayFields: map[string]any{
				"assignee": "Sarah Chen",
				"reporter": "Mike Smith",
			},
			Description: md("## Overview\n\nReplace legacy session-based auth with **OAuth 2.0 + PKCE**.\n\n## Goals\n\n- Eliminate session token storage issues\n- Support SSO via SAML/OIDC\n- Reduce login friction by 40%"),
			Comments: []core.Comment{
				{Author: "Mike Smith", Created: "01 Jan 2025, 10:30",
					Body: md("Kicked off the epic. Sarah is leading this — let's aim to have the PKCE flow in staging by end of sprint 4.")},
				{Author: "Alex Rivera", Created: "10 Jan 2025, 09:15",
					Body: md("Started on the PKCE flow. A few notes from my initial investigation:\n\n1. The existing `AuthService` interface is too tightly coupled to session tokens — I'll need to introduce an `OAuthProvider` abstraction\n2. Refresh token rotation needs careful thought around race conditions with concurrent requests\n3. The IdP metadata endpoint returns XML which we'll need to parse for SAML support later\n\nI've spiked a branch (`feat/oauth-pkce-spike`) with the basic authorization URL generation working. Will open a draft PR tomorrow for early feedback.")},
				{Author: "Sarah Chen", Created: "15 Jan 2025, 14:00",
					Body: md("Quick update: PKCE implementation is **on track**. The admin panel (ENG-102) might slip to next sprint.\n\n> The IdP error handling is a pre-existing issue we should fix in parallel.")},
			},
		},
		{
			ID: "ENG-101", Summary: "Implement OAuth 2.0 PKCE login flow",
			Type: "Story", Status: "In Review", ParentID: "ENG-100",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "alex@example.com",
				"reporter": "sarah@example.com",
				"created":  "18 Jan 2025",
				"updated":  "27 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
				"reporter": "Sarah Chen",
			},
			Description: md("Implement the full OAuth 2.0 Authorization Code flow with PKCE.\n\n## Acceptance Criteria\n\n- User redirected to IdP on Sign In\n- Valid access token issued on callback\n- PKCE verifier validated"),
		},
		{
			ID: "ENG-102", Summary: "Add SSO configuration admin panel",
			Type: "Story", Status: "To Do", ParentID: "ENG-100",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "jordan@example.com",
				"created":  "20 Jan 2025",
				"updated":  "25 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Jordan Lee",
			},
		},
		{
			ID: "ENG-103", Summary: "Write unit tests for token exchange",
			Type: "Sub-task", Status: "In Progress", ParentID: "ENG-101",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "alex@example.com",
				"created":  "22 Jan 2025",
				"updated":  "28 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
			},
		},
		{
			ID: "ENG-104", Summary: "Handle refresh token rotation",
			Type: "Sub-task", Status: "To Do", ParentID: "ENG-101",
			Fields: map[string]any{
				"priority": "Low",
				"assignee": "",
				"created":  "22 Jan 2025",
				"updated":  "22 Jan 2025",
			},
		},
		{
			ID: "ENG-200", Summary: "API Performance Improvements",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "mike@example.com",
				"created":  "10 Jan 2025",
				"updated":  "26 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Mike Smith",
			},
			Description: md("Improve API response times across all endpoints."),
		},
		{
			ID: "ENG-201", Summary: "Add Redis caching layer for hot paths",
			Type: "Task", Status: "Done", ParentID: "ENG-200",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "alex@example.com",
				"created":  "12 Jan 2025",
				"updated":  "24 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Alex Rivera",
			},
		},
		{
			ID: "ENG-300", Summary: "Login fails silently on expired IdP cert",
			Type: "Bug", Status: "To Do",
			Fields: map[string]any{
				"priority": "Highest",
				"assignee": "sarah@example.com",
				"created":  "25 Jan 2025",
				"updated":  "28 Jan 2025",
			},
			DisplayFields: map[string]any{
				"assignee": "Sarah Chen",
			},
			Description: md("When the IdP certificate expires, the login page shows a blank screen.\n\n**Steps to reproduce:**\n1. Set IdP cert to expired\n2. Navigate to /login\n3. Click Sign In\n\n**Expected:** Error message shown\n**Actual:** Blank screen"),
		},
	}

	registry := make(map[string]*core.WorkItem, len(items))
	for _, item := range items {
		registry[item.ID] = item
	}
	core.LinkChildren(registry)

	return items, registry
}

// NewMockProvider creates a MockProvider pre-populated with TestItems
// and standard capabilities. Callers can override fields as needed.
func NewMockProvider() *MockProvider {
	items := TestItems()
	mp := &MockProvider{
		SearchReturn: items,
		Registry:     make(map[string]*core.WorkItem, len(items)),
		Caps:         core.Capabilities{HasTransitions: true, HasTypes: true, HasHierarchy: true},
		UserReturn:   &core.User{DisplayName: "Demo User", ID: "test-user"},
	}
	for _, item := range items {
		mp.Registry[item.ID] = item
	}
	return mp
}

// NewTestRuntime creates a Runtime backed by any commands.UI, the canonical
// TestWorkspace, and default settings suitable for testing.
func NewTestRuntime(ui commands.UI) *commands.Runtime {
	ws := TestWorkspace()
	return &commands.Runtime{
		DefaultWorkspace: ws.Slug,
		Workspaces:       map[string]*core.Workspace{ws.Slug: ws},
		UI:               ui,
		Out:              io.Discard,
		Err:              io.Discard,
	}
}

// NewTestSession creates a WorkspaceSession backed by any commands.UI, the
// canonical TestWorkspace, and a pre-populated MockProvider.
func NewTestSession(ui commands.UI) *commands.WorkspaceSession {
	rt := NewTestRuntime(ui)
	ws := TestWorkspace()
	return &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  NewMockProvider(),
	}
}

// TestHarness bundles the common dependencies needed to construct a TUI
// AppModel in tests. All fields share a single Workspace instance, avoiding
// the subtle mismatch where Runtime.Workspaces and the model's WS diverge.
// Modify WS, Provider, or Runtime before passing to NewAppModel.
type TestHarness struct {
	WS       *core.Workspace
	Provider *MockProvider
	Runtime  *commands.Runtime
	Session  *commands.WorkspaceSession
	Factory  commands.WorkspaceSessionFactory
}

// NewTestHarness creates a fully wired harness with a single workspace,
// mock provider, and a factory that returns sessions using the harness's
// own fields. Callers can modify WS/Provider/Runtime before building a model.
func NewTestHarness(t testing.TB, ui commands.UI) *TestHarness {
	t.Helper()
	ws := TestWorkspace()
	provider := NewMockProvider()
	rt := &commands.Runtime{
		DefaultWorkspace: ws.Slug,
		Workspaces:       map[string]*core.Workspace{ws.Slug: ws},
		UI:               ui,
		Out:              io.Discard,
		Err:              io.Discard,
		CacheDir:         t.TempDir(),
	}
	session := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	h := &TestHarness{
		WS:       ws,
		Provider: provider,
		Runtime:  rt,
		Session:  session,
	}
	h.Factory = func(slug string) (*commands.WorkspaceSession, error) {
		return &commands.WorkspaceSession{
			Runtime:   h.Runtime,
			Workspace: h.WS,
			Provider:  h.Provider,
		}, nil
	}
	return h
}

// TestFieldDefs returns the standard test FieldDefs (same as MockProvider.FieldDefinitions).
func TestFieldDefs() core.FieldDefs {
	return (&MockProvider{}).FieldDefinitions()
}

// TestChildChain returns a three-level parent→child→grandchild chain.
// Each parent has exactly one child so hint key '0' is deterministic.
func TestChildChain() []*core.WorkItem {
	return []*core.WorkItem{
		{
			ID: "TEST-1", Summary: "Parent Epic", Type: "Epic", Status: "In Progress",
			Fields: map[string]any{"priority": "High", "assignee": "Alice", "created": "1 Jan 2025", "updated": "15 Jan 2025"},
		},
		{
			ID: "TEST-10", Summary: "Child Story", Type: "Story", Status: "To Do",
			ParentID: "TEST-1",
			Fields:   map[string]any{"priority": "Medium", "created": "2 Jan 2025", "updated": "16 Jan 2025"},
		},
		{
			ID: "TEST-20", Summary: "Grandchild Task", Type: "Task", Status: "Done",
			ParentID: "TEST-10",
			Fields:   map[string]any{"priority": "Medium", "created": "4 Jan 2025", "updated": "18 Jan 2025"},
		},
	}
}
