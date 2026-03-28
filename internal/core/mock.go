package core

import (
	"context"
	"fmt"

	"github.com/mikecsmith/ihj/internal/document"
)

// MockProvider implements Provider for tests across packages.
var _ Provider = (*MockProvider)(nil)

type MockProvider struct {
	SearchReturn []*WorkItem
	SearchErr    error
	GetReturn    *WorkItem
	Registry     map[string]*WorkItem // keyed lookups for Get
	CreateErr    error
	CreateCounter int    // auto-increments
	CreatePrefix  string // e.g. "ENG" — Create returns "ENG-1", "ENG-2", ...
	UpdateErr    error
	CommentErr   error
	AssignErr    error
	UserReturn   *User
	UserErr      error
	Caps         Capabilities
	Renderer     ContentRenderer

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
	Changes *Changes
}

func (m *MockProvider) Search(_ context.Context, filter string, _ bool) ([]*WorkItem, error) {
	return m.SearchReturn, m.SearchErr
}

func (m *MockProvider) Get(_ context.Context, id string) (*WorkItem, error) {
	if m.Registry != nil {
		if item, ok := m.Registry[id]; ok {
			return item, nil
		}
		return nil, fmt.Errorf("issue %s not found", id)
	}
	return m.GetReturn, nil
}

func (m *MockProvider) Create(_ context.Context, item *WorkItem) (string, error) {
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

func (m *MockProvider) Update(_ context.Context, id string, changes *Changes) error {
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

func (m *MockProvider) CurrentUser(_ context.Context) (*User, error) {
	return m.UserReturn, m.UserErr
}

func (m *MockProvider) Capabilities() Capabilities { return m.Caps }

func (m *MockProvider) ContentRenderer() ContentRenderer {
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
