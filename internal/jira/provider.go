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
	"fmt"
	"maps"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

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

	// wellKnown is the single source of truth for fields the provider recognises.
	wellKnown wellKnownFields

	nameToID map[string]string // "fieldKey:valueName" → "valueID" for payload construction
}

// Compile-time check that *Provider implements core.Provider.
var _ core.Provider = (*Provider)(nil)

// NewProvider creates a Jira provider for the given workspace.
// The workspace's ProviderConfig must already be a *jira.Config
// (hydrated by the composition root).
// Eagerly loads createmeta (from disk cache or API) so field metadata
// is available immediately. Returns an error if createmeta cannot be loaded.
// cacheDir may be empty to disable disk caching.
func NewProvider(client API, ws *core.Workspace, cacheDir string) (*Provider, error) {
	cfg, _ := ws.ProviderConfig.(*Config)
	p := &Provider{
		client:   client,
		ws:       ws,
		cfg:      cfg,
		cacheDir: cacheDir,
	}
	p.wellKnown = p.buildWellKnownFields()

	if err := p.loadFieldMeta(); err != nil {
		return nil, fmt.Errorf("loading field metadata: %w", err)
	}

	// If the disk cache is approaching expiry, refresh in the background
	// so the next session has a warm cache.
	p.backgroundRefreshIfNeeded()

	return p, nil
}

// Search returns work items matching the named filter.
// By default, a fresh disk cache is returned without hitting the API.
// Pass noCache=true to force a fresh fetch.
func (p *Provider) Search(ctx context.Context, filter string, noCache bool) ([]*core.WorkItem, error) {
	// Try cache first unless caller explicitly wants fresh data.
	if !noCache && p.cacheDir != "" {
		if cached, err := loadCache(p.cacheDir, p.ws.Slug, filter, p.ws.CacheTTL); err == nil {
			return issuesToWorkItems(cached.Issues, p.wellKnown, p.customFieldMap()), nil
		}
	}

	jql, err := buildJQL(p.ws, p.cfg, filter)
	if err != nil {
		return nil, err
	}

	issues, err := fetchAllIssues(ctx, p.client, jql, p.cfg.FormattedFields, p.customFieldIDs())
	if err != nil {
		return nil, err
	}

	// Save to cache for future calls.
	if p.cacheDir != "" {
		_ = saveCache(p.cacheDir, p.ws.Slug, filter, issues)
	}

	return issuesToWorkItems(issues, p.wellKnown, p.customFieldMap()), nil
}

// Get returns a single work item by its Jira issue key.
func (p *Provider) Get(ctx context.Context, id string) (*core.WorkItem, error) {
	iss, err := p.client.FetchIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", id, err)
	}
	return issueToWorkItem(iss, p.wellKnown, p.customFieldMap()), nil
}

// Create persists a new work item and returns its assigned key.
func (p *Provider) Create(ctx context.Context, item *core.WorkItem) (string, error) {
	fields := map[string]any{
		"summary": item.Summary,
		"project": map[string]any{"key": p.cfg.ProjectKey},
	}

	if tc := p.ws.TypeByName(item.Type); tc != nil {
		fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", tc.ID)}
	}

	if item.ParentID != "" {
		fields["parent"] = map[string]any{"key": strings.ToUpper(item.ParentID)}
	}

	if item.Description != nil {
		fields["description"] = renderADFValue(item.Description)
	}

	tx, err := p.wellKnown.TranslateFields(p, ctx, item.Fields)
	if err != nil {
		return "", err
	}

	maps.Copy(fields, tx.fields)

	created, err := p.client.CreateIssue(ctx, map[string]any{"fields": fields})
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}

	return created.Key, nil
}

// Update applies changes to an existing work item.
func (p *Provider) Update(ctx context.Context, id string, changes *core.Changes) error {
	fields := make(map[string]any)

	if changes.Summary != nil {
		fields["summary"] = *changes.Summary
	}

	if changes.Type != nil {
		if tc := p.ws.TypeByName(*changes.Type); tc != nil {
			fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", tc.ID)}
		}
	}

	if changes.ParentID != nil {
		if *changes.ParentID == "" {
			fields["parent"] = nil // clear parent
		} else {
			fields["parent"] = map[string]any{"key": strings.ToUpper(*changes.ParentID)}
		}
	}

	if changes.Description != nil {
		fields["description"] = renderADFValue(changes.Description)
	}

	tx, err := p.wellKnown.TranslateFields(p, ctx, changes.Fields)
	if err != nil {
		return err
	}
	for k, v := range tx.fields {
		fields[k] = v
	}

	if len(fields) > 0 {
		if err := p.client.UpdateIssue(ctx, id, map[string]any{"fields": fields}); err != nil {
			return fmt.Errorf("updating issue %s: %w", id, err)
		}
	}

	if tx.assignUser != nil {
		if err := p.client.AssignIssue(ctx, id, *tx.assignUser); err != nil {
			return fmt.Errorf("assigning %s: %w", id, err)
		}
	}

	if changes.Status != nil {
		if err := performTransition(ctx, p.client, id, *changes.Status); err != nil {
			return fmt.Errorf("transitioning %s to '%s': %w", id, *changes.Status, err)
		}
	}

	if tx.sprintTarget != "" {
		if err := sprintAssign(ctx, p.client, p.cfg.BoardID, id, tx.sprintTarget); err != nil {
			return fmt.Errorf("assigning %s to %s sprint: %w", id, tx.sprintTarget, err)
		}
	}

	return nil
}

// Comment adds a comment to a Jira issue.
func (p *Provider) Comment(ctx context.Context, id string, body string) error {
	ast, err := document.ParseMarkdownString(body)
	if err != nil {
		return fmt.Errorf("parsing comment: %w", err)
	}
	adfBody := renderADFValue(ast)
	return p.client.AddComment(ctx, id, adfBody)
}

// Assign assigns the issue to the current authenticated user.
func (p *Provider) Assign(ctx context.Context, id string) error {
	u, err := p.resolveUser(ctx)
	if err != nil {
		return fmt.Errorf("fetching current user: %w", err)
	}
	return p.client.AssignIssue(ctx, id, u.AccountID)
}

// CurrentUser returns the authenticated Jira user.
func (p *Provider) CurrentUser(ctx context.Context) (*core.User, error) {
	u, err := p.resolveUser(ctx)
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
func (p *Provider) resolveUser(ctx context.Context) (*user, error) {
	if p.cachedUser != nil {
		return p.cachedUser, nil
	}
	u, err := p.client.FetchMyself(ctx)
	if err != nil {
		return nil, err
	}
	p.cachedUser = u
	return p.cachedUser, nil
}

// Capabilities returns the feature set supported by the Jira provider.
func (p *Provider) Capabilities() core.Capabilities {
	return core.Capabilities{
		HasHierarchy:   true,
		HasTransitions: true,
		HasTypes:       true,
		StatusSource:   core.StatusSourceWorkflow,
	}
}

// TransitionsFor returns the selectable workflow transition names for the
// issue along with its current status name. Jira filters transitions by
// workflow on the server, so we simply surface what the API returns.
func (p *Provider) TransitionsFor(ctx context.Context, id string) (string, []string, error) {
	item, err := p.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	transitions, err := p.client.FetchTransitions(ctx, id)
	if err != nil {
		return "", nil, fmt.Errorf("fetching transitions for %s: %w", id, err)
	}
	titleCase := cases.Title(language.English)
	opts := make([]string, 0, len(transitions))
	for _, t := range transitions {
		name := t.To.Name
		if name == "" {
			name = t.Name
		}
		// Skip transitions that land on the current status — they're no-ops.
		if strings.EqualFold(name, item.Status) {
			continue
		}
		opts = append(opts, titleCase.String(name))
	}
	return item.Status, opts, nil
}

// ContentRenderer returns the Jira ADF content renderer.
func (p *Provider) ContentRenderer() core.ContentRenderer {
	return &adfRenderer{}
}
