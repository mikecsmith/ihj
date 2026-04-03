// Package demo provides an in-memory Provider for demo/testing purposes.
// It implements core.Provider backed entirely by synthetic WorkItems,
// requiring no external backend connection.
package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Provider implements core.Provider backed by in-memory WorkItems.
type Provider struct {
	mu       sync.RWMutex
	registry map[string]*core.WorkItem
	items    []*core.WorkItem
	latency  time.Duration
}

var _ core.Provider = (*Provider)(nil)

// NewProvider creates a demo provider from the given items.
// Latency simulates API delay for realistic TUI behavior.
func NewProvider(items []*core.WorkItem, latency time.Duration) *Provider {
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	return &Provider{
		registry: registry,
		items:    items,
		latency:  latency,
	}
}

func (p *Provider) sleep() {
	if p.latency > 0 {
		time.Sleep(p.latency)
	}
}

func (p *Provider) Search(_ context.Context, filter string, _ bool) ([]*core.WorkItem, error) {
	p.sleep()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if filter == "my" {
		var mine []*core.WorkItem
		for _, item := range p.items {
			if item.StringField("assignee") == "Mike Smith" {
				mine = append(mine, item)
			}
		}
		return mine, nil
	}
	return p.items, nil
}

func (p *Provider) Get(_ context.Context, id string) (*core.WorkItem, error) {
	p.sleep()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if item, ok := p.registry[id]; ok {
		return item, nil
	}
	return nil, fmt.Errorf("issue %s not found", id)
}

func (p *Provider) Create(_ context.Context, item *core.WorkItem) (string, error) {
	p.sleep()
	p.mu.Lock()
	defer p.mu.Unlock()
	key := fmt.Sprintf("DEMO-%d", 100+len(p.registry))
	item.ID = key
	p.registry[key] = item
	p.items = append(p.items, item)
	return key, nil
}

func (p *Provider) Update(_ context.Context, id string, changes *core.Changes) error {
	p.sleep()
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.registry[id]
	if !ok {
		return fmt.Errorf("issue %s not found", id)
	}
	if changes.Summary != nil {
		item.Summary = *changes.Summary
	}
	if changes.Type != nil {
		item.Type = *changes.Type
	}
	if changes.Status != nil {
		item.Status = *changes.Status
	}
	if changes.ParentID != nil {
		item.ParentID = *changes.ParentID
	}
	if changes.Description != nil {
		item.Description = changes.Description
	}
	return nil
}

func (p *Provider) Comment(_ context.Context, id string, _ string) error {
	p.sleep()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.registry[id]; !ok {
		return fmt.Errorf("issue %s not found", id)
	}
	// Don't append here — the TUI appends locally after a successful Comment call.
	// Appending in both places would duplicate the comment.
	return nil
}

func (p *Provider) Assign(_ context.Context, id string) error {
	p.sleep()
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.registry[id]
	if !ok {
		return fmt.Errorf("issue %s not found", id)
	}
	if item.Fields == nil {
		item.Fields = make(map[string]any)
	}
	item.Fields["assignee"] = "Mike Smith"
	return nil
}

func (p *Provider) CurrentUser(_ context.Context) (*core.User, error) {
	return &core.User{
		ID:          "demo-user-1",
		DisplayName: "Mike Smith",
		Email:       "mike.smith@example.com",
	}, nil
}

func (p *Provider) Capabilities() core.Capabilities {
	return core.Capabilities{
		HasHierarchy:   true,
		HasTransitions: true,
		HasTypes:       true,
	}
}

func (p *Provider) ContentRenderer() core.ContentRenderer {
	return &markdownRenderer{}
}

func (p *Provider) FieldDefinitions() core.FieldDefs {
	return core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum,
			Enum: []string{"High", "Medium", "Low"},
			Role: core.RoleUrgency, Primary: true},
		{Key: "assignee", Label: "Assignee", Icon: core.IconUser, Type: core.FieldString,
			Role: core.RoleOwnership, Primary: true},
		{Key: "labels", Label: "Labels", Icon: core.IconTag, Type: core.FieldStringArray,
			Role: core.RoleCategorisation, Primary: true},
	}
}

// markdownRenderer is a pass-through — demo data is already in AST form.
type markdownRenderer struct{}

func (r *markdownRenderer) ParseContent(raw any) (*document.Node, error) {
	if s, ok := raw.(string); ok {
		return document.ParseMarkdownString(s)
	}
	return nil, fmt.Errorf("unsupported content type: %T", raw)
}

func (r *markdownRenderer) RenderContent(node *document.Node) (any, error) {
	return document.RenderMarkdown(node), nil
}
