package core

import (
	"time"

	"github.com/mikecsmith/ihj/internal/document"
)

// Provider constants — only define constants for providers that have code.
const (
	ProviderJira = "jira"
	ProviderDemo = "demo"
)

// DefaultCacheTTL is the cache freshness duration when no override is configured.
const DefaultCacheTTL = 15 * time.Minute

// Workspace represents a configured scope of work items from a specific
// provider. Each workspace has common fields (name, types, statuses) plus
// provider-specific configuration. The Provider field is a discriminator
// (e.g., ProviderJira, ProviderDemo) that determines how ProviderConfig
// is interpreted.
type Workspace struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`    // Discriminator: "jira", "github", "trello"
	ServerAlias string `json:"serverAlias"` // Key into credential store for token lookup
	BaseURL     string `json:"baseUrl"`     // Server URL for browse links

	// Types defines the work item types available in this workspace.
	Types []TypeConfig `json:"types"`

	// Statuses defines the allowed status values, in display order.
	Statuses []StatusConfig `json:"statuses"`

	// Filters holds named query filters (e.g., "active", "me", "all").
	// The keys are user-visible names; values are provider-specific query fragments.
	Filters map[string]string `json:"filters"`

	// CacheTTL is the duration for which cached data is considered fresh.
	// Resolved at config load: workspace cache_ttl > global cache_ttl > DefaultCacheTTL.
	CacheTTL time.Duration `json:"-"`

	// Guidance holds custom LLM guidance text for the extract command.
	// Resolved at config load: workspace guidance > global guidance > DefaultGuidance.
	Guidance string `json:"-"`

	// Internal — not serialized for frontend.
	StatusOrderMap map[string]StatusOrderEntry `json:"-"`
	TypeOrderMap   map[string]TypeOrderEntry   `json:"-"`
	ProviderConfig any                         `json:"-"`
}

// StatusConfig describes a work item status within a workspace.
type StatusConfig struct {
	Name  string `json:"name"`
	Order int    `json:"order"`
	Color string `json:"color"`
}

// TypeConfig describes a work item type within a workspace.
type TypeConfig struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Order       int    `json:"order"`
	Color       string `json:"color"`
	HasChildren bool   `json:"hasChildren"`
	Template    string `json:"template"`

	// Fields holds the resolved FieldDefs for this type, populated by the
	// provider from API metadata (e.g. createmeta). Not part of config.
	Fields FieldDefs `json:"-"`

	// ExtraFields holds per-type opted-in fields from the config `fields:` map.
	// Keys are user-chosen aliases, values are provider-specific field IDs
	// (e.g. Jira custom field numeric IDs). Parsed from config; the provider
	// uses these during field resolution.
	ExtraFields map[string]int `json:"-"`
}

// TypeOrderEntry is the computed rendering metadata for a work item type.
type TypeOrderEntry struct {
	Order       int
	Color       string
	HasChildren bool
}

// StatusOrderEntry is the computed rendering metadata for a work item status.
type StatusOrderEntry struct {
	Weight int
	Color  string
}

// BrowseURL returns the web URL for viewing a work item by ID.
// Returns empty string if BaseURL is not configured.
func (ws *Workspace) BrowseURL(id string) string {
	if ws.BaseURL == "" {
		return ""
	}
	return ws.BaseURL + "/browse/" + id
}

// Comment represents a comment on a work item.
type Comment struct {
	Author  string
	Created string
	Body    *document.Node
}
