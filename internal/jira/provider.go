package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Provider implements core.Provider for Jira backends.
// It wraps the low-level API client and translates between
// Jira-specific types and the universal core.WorkItem model.
type Provider struct {
	client API
	board  *config.BoardConfig
	cfg    *config.Config
}

// Compile-time check that *Provider implements core.Provider.
var _ core.Provider = (*Provider)(nil)

// NewProvider creates a Jira provider for the given board configuration.
func NewProvider(client API, cfg *config.Config, board *config.BoardConfig) *Provider {
	return &Provider{
		client: client,
		board:  board,
		cfg:    cfg,
	}
}

// Search returns work items matching the named filter.
// The filter name is resolved against the board's configured filters
// and combined with the base JQL.
func (p *Provider) Search(_ context.Context, filter string) ([]*core.WorkItem, error) {
	jql, err := BuildJQL(p.board, filter, p.cfg.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	issues, err := FetchAllIssues(p.client, jql, p.cfg.FormattedCustomFields)
	if err != nil {
		return nil, err
	}

	items := make([]*core.WorkItem, 0, len(issues))
	for _, iss := range issues {
		items = append(items, issueToWorkItem(&iss))
	}
	return items, nil
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
	if item.Description != "" {
		ast, err := document.ParseMarkdownString(item.Description)
		if err != nil {
			return "", fmt.Errorf("parsing description: %w", err)
		}
		adfDesc = RenderADFValue(ast)
	}

	payload := BuildUpsertPayload(
		fm, adfDesc, p.board.Types, p.cfg.CustomFields,
		p.board.ProjectKey, p.board.TeamUUID,
	)

	created, err := p.client.CreateIssue(payload)
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}

	return created.Key, nil
}

// Update applies changes to an existing work item.
// Status changes are handled via Jira transitions internally.
func (p *Provider) Update(_ context.Context, id string, changes *core.Changes) error {
	fields := make(map[string]any)

	if changes.Summary != nil {
		fields["summary"] = *changes.Summary
	}

	if changes.Type != nil {
		for _, t := range p.board.Types {
			if strings.EqualFold(t.Name, *changes.Type) {
				fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
				break
			}
		}
	}

	if changes.Description != nil {
		if node, ok := changes.Description.(*document.Node); ok {
			fields["description"] = RenderADFValue(node)
		}
	}

	// Apply backend-specific field changes.
	for k, v := range changes.Fields {
		fields[k] = v
	}

	if len(fields) > 0 {
		if err := p.client.UpdateIssue(id, map[string]any{"fields": fields}); err != nil {
			return fmt.Errorf("updating issue %s: %w", id, err)
		}
	}

	// Handle status transition separately — Jira requires fetching
	// available transitions and executing the matching one.
	if changes.Status != nil {
		if err := PerformTransition(p.client, id, *changes.Status); err != nil {
			return fmt.Errorf("transitioning %s to '%s': %w", id, *changes.Status, err)
		}
	}

	return nil
}

// Comment adds a comment to a Jira issue.
// The body is markdown text, converted to ADF for the Jira API.
func (p *Provider) Comment(_ context.Context, id string, body string) error {
	ast, err := document.ParseMarkdownString(body)
	if err != nil {
		return fmt.Errorf("parsing comment: %w", err)
	}
	adfBody := RenderADFValue(ast)
	return p.client.AddComment(id, adfBody)
}

// Assign assigns the issue to the current authenticated user.
func (p *Provider) Assign(_ context.Context, id string) error {
	user, err := p.client.FetchMyself()
	if err != nil {
		return fmt.Errorf("fetching current user: %w", err)
	}
	return p.client.AssignIssue(id, user.AccountID)
}

// CurrentUser returns the authenticated Jira user.
func (p *Provider) CurrentUser(_ context.Context) (*core.User, error) {
	user, err := p.client.FetchMyself()
	if err != nil {
		return nil, err
	}
	return &core.User{
		ID:          user.AccountID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
	}, nil
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

// --- ADF ContentRenderer ---

// adfRenderer implements core.ContentRenderer for Jira's ADF format.
type adfRenderer struct{}

func (r *adfRenderer) ParseContent(raw any) (any, error) {
	switch v := raw.(type) {
	case json.RawMessage:
		return ParseADF(v)
	case []byte:
		return ParseADF(v)
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling ADF: %w", err)
		}
		return ParseADF(data)
	default:
		return nil, fmt.Errorf("unsupported ADF input type: %T", raw)
	}
}

func (r *adfRenderer) RenderContent(node any) (any, error) {
	n, ok := node.(*document.Node)
	if !ok {
		return nil, fmt.Errorf("expected *document.Node, got %T", node)
	}
	return RenderADFValue(n), nil
}

// --- Conversion helpers ---

// issueToWorkItem converts a Jira Issue to a core.WorkItem.
func issueToWorkItem(iss *Issue) *core.WorkItem {
	f := &iss.Fields

	descMD := ""
	if len(f.Description) > 0 && string(f.Description) != "null" {
		if ast, err := ParseADF(f.Description); err == nil {
			descMD = strings.TrimSpace(document.RenderMarkdown(ast))
		}
	}

	item := &core.WorkItem{
		ID:          iss.Key,
		Type:        f.IssueType.Name,
		Summary:     f.Summary,
		Status:      f.Status.Name,
		Description: descMD,
		Fields:      make(map[string]any),
	}

	// Populate flex fields.
	if f.Priority.Name != "" {
		item.Fields["priority"] = f.Priority.Name
	}
	if f.Parent != nil {
		item.Fields["parent"] = f.Parent.Key
	}
	if f.Assignee != nil {
		item.Fields["assignee"] = f.Assignee.DisplayName
	}
	if f.Reporter != nil {
		item.Fields["reporter"] = f.Reporter.DisplayName
	}
	if len(f.Labels) > 0 {
		item.Fields["labels"] = f.Labels
	}
	if len(f.Components) > 0 {
		names := make([]string, len(f.Components))
		for i, c := range f.Components {
			names[i] = c.Name
		}
		item.Fields["components"] = names
	}

	return item
}

// workItemToFrontmatter converts a core.WorkItem to the frontmatter map
// expected by BuildUpsertPayload.
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
	if v, ok := item.Fields["parent"].(string); ok && v != "" {
		fm["parent"] = v
	}
	return fm
}
