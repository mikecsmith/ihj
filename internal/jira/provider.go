// Package jira implements the Atlassian Jira provider.
//
// It acts as an adapter between the Jira REST API and the universal
// domain model defined in the core package. Its primary responsibilities
// are translating Jira-specific concepts (ADF descriptions, JQL, custom
// fields, sprint management, and workflow transitions) into backend-agnostic
// core.WorkItem structures, and managing per-workspace caching.
//
// API types are derived from the Atlassian OpenAPI spec at:
//
//	https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Provider implements core.Provider for Jira backends.
// It wraps the low-level API client and translates between
// Jira-specific types and the universal core.WorkItem model.
type Provider struct {
	client   API
	ws       *core.Workspace
	cfg      *Config
	cacheDir string

	// cachedUser avoids repeated FetchMyself calls within a session.
	cachedUser *user
}

// Compile-time check that *Provider implements core.Provider.
var _ core.Provider = (*Provider)(nil)

// NewProvider creates a Jira provider for the given workspace.
// The workspace's ProviderConfig must already be a *jira.Config
// (hydrated by the composition root).
// cacheDir may be empty to disable disk caching.
func NewProvider(client API, ws *core.Workspace, cacheDir string) *Provider {
	cfg, _ := ws.ProviderConfig.(*Config)
	return &Provider{
		client:   client,
		ws:       ws,
		cfg:      cfg,
		cacheDir: cacheDir,
	}
}

// Search returns work items matching the named filter.
// By default, a fresh disk cache is returned without hitting the API.
// Pass noCache=true to force a fresh fetch.
func (p *Provider) Search(_ context.Context, filter string, noCache bool) ([]*core.WorkItem, error) {
	// Try cache first unless caller explicitly wants fresh data.
	if !noCache && p.cacheDir != "" {
		if cached, err := loadCache(p.cacheDir, p.ws.Slug, filter); err == nil {
			return issuesToWorkItems(cached.Issues), nil
		}
	}

	jql, err := buildJQL(p.ws, p.cfg, filter)
	if err != nil {
		return nil, err
	}

	issues, err := fetchAllIssues(p.client, jql, p.cfg.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	// Save to cache for future calls.
	if p.cacheDir != "" {
		_ = saveCache(p.cacheDir, p.ws.Slug, filter, issues)
	}

	return issuesToWorkItems(issues), nil
}

// Get returns a single work item by its Jira issue key.
func (p *Provider) Get(_ context.Context, id string) (*core.WorkItem, error) {
	iss, err := p.client.FetchIssue(id)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", id, err)
	}
	return issueToWorkItem(iss), nil
}

// Create persists a new work item and returns its assigned key.
func (p *Provider) Create(_ context.Context, item *core.WorkItem) (string, error) {
	fm := workItemToFrontmatter(item)

	var adfDesc map[string]any
	if item.Description != nil {
		adfDesc = renderADFValue(item.Description)
	}

	payload := buildUpsertPayload(
		fm, adfDesc, p.ws.Types, p.cfg.CustomFields,
		p.cfg.ProjectKey, p.cfg.TeamUUID,
	)

	created, err := p.client.CreateIssue(payload)
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}

	return created.Key, nil
}

// Update applies changes to an existing work item.
func (p *Provider) Update(_ context.Context, id string, changes *core.Changes) error {
	fields := make(map[string]any)

	if changes.Summary != nil {
		fields["summary"] = *changes.Summary
	}

	if changes.Type != nil {
		for _, t := range p.ws.Types {
			if strings.EqualFold(t.Name, *changes.Type) {
				fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
				break
			}
		}
	}

	if changes.ParentID != nil {
		fields["parent"] = map[string]any{"key": *changes.ParentID}
	}

	if changes.Description != nil {
		fields["description"] = renderADFValue(changes.Description)
	}

	// Extract sprint before copying fields — it's not a Jira field but a
	// board-level action handled separately via the agile API.
	var doAssignSprint bool
	if changes.Fields != nil {
		if sprintVal, ok := changes.Fields["sprint"]; ok {
			if b, isBool := sprintVal.(bool); isBool && b {
				doAssignSprint = true
			}
			// Don't copy sprint into the Jira fields payload.
			filtered := make(map[string]any, len(changes.Fields)-1)
			for k, v := range changes.Fields {
				if k != "sprint" {
					filtered[k] = v
				}
			}
			maps.Copy(fields, filtered)
		} else {
			maps.Copy(fields, changes.Fields)
		}
	}

	if len(fields) > 0 {
		if err := p.client.UpdateIssue(id, map[string]any{"fields": fields}); err != nil {
			return fmt.Errorf("updating issue %s: %w", id, err)
		}
	}

	if changes.Status != nil {
		if err := performTransition(p.client, id, *changes.Status); err != nil {
			return fmt.Errorf("transitioning %s to '%s': %w", id, *changes.Status, err)
		}
	}

	if doAssignSprint {
		if _, err := assignToSprint(p.client, p.cfg.BoardID, id); err != nil {
			return fmt.Errorf("assigning %s to active sprint: %w", id, err)
		}
	}

	return nil
}

// Comment adds a comment to a Jira issue.
func (p *Provider) Comment(_ context.Context, id string, body string) error {
	ast, err := document.ParseMarkdownString(body)
	if err != nil {
		return fmt.Errorf("parsing comment: %w", err)
	}
	adfBody := renderADFValue(ast)
	return p.client.AddComment(id, adfBody)
}

// Assign assigns the issue to the current authenticated user.
func (p *Provider) Assign(_ context.Context, id string) error {
	u, err := p.resolveUser()
	if err != nil {
		return fmt.Errorf("fetching current user: %w", err)
	}
	return p.client.AssignIssue(id, u.AccountID)
}

// CurrentUser returns the authenticated Jira user.
func (p *Provider) CurrentUser(_ context.Context) (*core.User, error) {
	u, err := p.resolveUser()
	if err != nil {
		return nil, err
	}
	return &core.User{
		ID:          u.AccountID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
	}, nil
}

// resolveUser returns the cached user or fetches and caches it.
func (p *Provider) resolveUser() (*user, error) {
	if p.cachedUser != nil {
		return p.cachedUser, nil
	}
	u, err := p.client.FetchMyself()
	if err != nil {
		return nil, err
	}
	p.cachedUser = u
	return p.cachedUser, nil
}

// Capabilities returns the feature set supported by the Jira provider.
func (p *Provider) Capabilities() core.Capabilities {
	return core.Capabilities{
		HasSprints:      true,
		HasHierarchy:    true,
		HasTransitions:  true,
		HasCustomFields: true,
		HasTypes:        true,
		HasPriority:     true,
		HasComponents:   true,
	}
}

// ContentRenderer returns the Jira ADF content renderer.
func (p *Provider) ContentRenderer() core.ContentRenderer {
	return &adfRenderer{}
}

// adfRenderer implements core.ContentRenderer for Jira's ADF format.
type adfRenderer struct{}

func (r *adfRenderer) ParseContent(raw any) (*document.Node, error) {
	switch v := raw.(type) {
	case json.RawMessage:
		return parseADF(v)
	case []byte:
		return parseADF(v)
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling ADF: %w", err)
		}
		return parseADF(data)
	default:
		return nil, fmt.Errorf("unsupported ADF input type: %T", raw)
	}
}

func (r *adfRenderer) RenderContent(node *document.Node) (any, error) {
	return renderADFValue(node), nil
}

// workItemToFrontmatter converts a core.WorkItem to the frontmatter map
// expected by buildUpsertPayload.
func workItemToFrontmatter(item *core.WorkItem) map[string]string {
	fm := map[string]string{
		"summary": item.Summary,
		"type":    item.Type,
	}
	if item.Status != "" {
		fm["status"] = item.Status
	}
	if v, ok := item.Fields["priority"].(string); ok && v != "" {
		fm["priority"] = v
	}
	if item.ParentID != "" {
		fm["parent"] = item.ParentID
	}
	return fm
}
