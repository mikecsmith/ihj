package core

import "github.com/mikecsmith/ihj/internal/document"

// Workspace represents a configured scope of work items from a specific
// provider. It is the backend-agnostic equivalent of what was previously
// called a "Board" in the Jira-specific configuration.
//
// A workspace has common fields (name, types, statuses) plus a typed
// provider-specific config. The provider field is a discriminator
// ("jira", "github", "trello") set from the YAML config.
type Workspace struct {
	Slug     string
	Name     string
	Provider string // Discriminator: "jira", "github", "trello"
	BaseURL  string // Server URL for browse links (e.g., "https://company.atlassian.net")

	// Types defines the work item types available in this workspace.
	Types []TypeConfig

	// Statuses defines the allowed status values, in display order.
	Statuses []string

	// Filters holds named query filters (e.g., "active", "me", "all").
	// The keys are user-visible names; values are provider-specific query fragments.
	Filters map[string]string

	// StatusWeights maps lowercase status names to sort weights.
	StatusWeights map[string]int

	// TypeOrderMap maps type name to ordering/display metadata.
	TypeOrderMap map[string]TypeOrderEntry

	// ProviderConfig holds the typed, provider-specific configuration.
	// After storage.LoadConfig, this is map[string]any; the composition root
	// hydrates it into a typed struct (e.g., *jira.Config) before passing
	// to the provider.
	ProviderConfig any
}

// TypeConfig describes a work item type within a workspace.
type TypeConfig struct {
	ID          int
	Name        string
	Order       int
	Color       string
	HasChildren bool
	Template    string
}

// TypeOrderEntry is the computed rendering metadata for a work item type.
type TypeOrderEntry struct {
	Order       int
	Color       string
	HasChildren bool
}

// Comment represents a comment on a work item.
type Comment struct {
	Author  string
	Created string
	Body    *document.Node
}
