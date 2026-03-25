package core

// Workspace represents a configured scope of work items from a specific
// backend provider. It is the backend-agnostic equivalent of what was
// previously called a "Board" in the Jira-specific configuration.
//
// A workspace has common fields (name, types, statuses) plus a typed
// backend-specific config (e.g., JiraConfig, GitHubConfig). The backend
// is inferred from which backend config key is present in the YAML.
type Workspace struct {
	Slug    string
	Name    string
	Backend string // Inferred from config key: "jira", "github", "trello"

	// Types defines the work item types available in this workspace.
	Types []TypeConfig

	// Statuses defines the allowed status values, in display order.
	Statuses []string

	// BackendConfig holds the typed, backend-specific configuration.
	// At runtime this will be a concrete struct pointer (e.g., *jira.Config).
	BackendConfig any
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

// Comment represents a comment on a work item for display purposes.
type Comment struct {
	Author  string
	Created string
	Body    string // Rendered markdown
}
