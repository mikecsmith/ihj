package testutil

import (
	"context"
	"fmt"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Verify MockProvider implements core.Provider at compile time.
var _ core.Provider = (*MockProvider)(nil)

// MockProvider implements core.Provider for tests across packages.
type MockProvider struct {
	SearchReturn  []*core.WorkItem
	SearchErr     error
	GetReturn     *core.WorkItem
	Registry      map[string]*core.WorkItem // keyed lookups for Get
	CreateErr     error
	CreateCounter int    // auto-increments
	CreatePrefix  string // e.g. "ENG" — Create returns "ENG-1", "ENG-2", ...
	UpdateErr     error
	CommentErr    error
	AssignErr     error
	UserReturn    *core.User
	UserErr       error
	Caps          core.Capabilities
	Renderer      core.ContentRenderer

	// Call records.
	CommentCalls []MockCommentCall
	AssignCalls  []string
	UpdateCalls  []MockUpdateCall
}

type MockCommentCall struct {
	ID   string
	Body string
}

type MockUpdateCall struct {
	ID      string
	Changes *core.Changes
}

func (m *MockProvider) Search(_ context.Context, filter string, _ bool) ([]*core.WorkItem, error) {
	return m.SearchReturn, m.SearchErr
}

func (m *MockProvider) Get(_ context.Context, id string) (*core.WorkItem, error) {
	if m.Registry != nil {
		if item, ok := m.Registry[id]; ok {
			return item, nil
		}
		return nil, fmt.Errorf("issue %s not found", id)
	}
	return m.GetReturn, nil
}

func (m *MockProvider) Create(_ context.Context, item *core.WorkItem) (string, error) {
	if m.CreateErr != nil {
		return "", m.CreateErr
	}
	m.CreateCounter++
	prefix := m.CreatePrefix
	if prefix == "" {
		prefix = "MOCK"
	}
	return fmt.Sprintf("%s-%d", prefix, m.CreateCounter), nil
}

func (m *MockProvider) Update(_ context.Context, id string, changes *core.Changes) error {
	m.UpdateCalls = append(m.UpdateCalls, MockUpdateCall{ID: id, Changes: changes})
	return m.UpdateErr
}

func (m *MockProvider) Comment(_ context.Context, id string, body string) error {
	m.CommentCalls = append(m.CommentCalls, MockCommentCall{ID: id, Body: body})
	return m.CommentErr
}

func (m *MockProvider) Assign(_ context.Context, id string) error {
	m.AssignCalls = append(m.AssignCalls, id)
	return m.AssignErr
}

func (m *MockProvider) CurrentUser(_ context.Context) (*core.User, error) {
	return m.UserReturn, m.UserErr
}

func (m *MockProvider) Capabilities() core.Capabilities { return m.Caps }

func (m *MockProvider) FieldDefinitions() core.FieldDefs {
	return core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum,
			Enum: []string{"High", "Medium", "Low"},
			Role: core.RoleUrgency, Primary: true},
		{Key: "assignee", Label: "Assignee", Type: core.FieldString,
			Role: core.RoleOwnership, Primary: true},
		{Key: "labels", Label: "Labels", Type: core.FieldStringArray,
			Role: core.RoleCategorisation, Primary: true},
		{Key: "components", Label: "Components", Type: core.FieldStringArray,
			Role: core.RoleCategorisation, Optional: true},
		{Key: "reporter", Label: "Reporter", Type: core.FieldEmail,
			Role: core.RoleOwnership},
		{Key: "created", Label: "Created", Type: core.FieldString,
			Role: core.RoleTemporal, Primary: true, Derived: true, Immutable: true},
		{Key: "updated", Label: "Updated", Type: core.FieldString,
			Role: core.RoleTemporal, Derived: true},
	}
}

func (m *MockProvider) ContentRenderer() core.ContentRenderer {
	if m.Renderer != nil {
		return m.Renderer
	}
	return &MockContentRenderer{}
}

// MockContentRenderer is a no-op content renderer for tests.
type MockContentRenderer struct{}

func (r *MockContentRenderer) ParseContent(raw any) (*document.Node, error) {
	return document.NewDoc(), nil
}

func (r *MockContentRenderer) RenderContent(node *document.Node) (any, error) {
	return map[string]any{"type": "doc"}, nil
}
