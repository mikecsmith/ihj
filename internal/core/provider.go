package core

import (
	"context"

	"github.com/mikecsmith/ihj/internal/document"
)

// Provider abstracts a work-tracking backend (Jira, GitHub, Trello, etc.).
// It is the primary interface consumed by the commands package.
//
// Implementations handle all backend-specific concerns internally:
// query building, status transitions, description format conversion, etc.
// The commands layer only speaks WorkItem and workspace slugs.
type Provider interface {
	// Search returns work items matching the named filter.
	// The provider translates the filter name into a backend-native query
	// (e.g., JQL for Jira, GraphQL for GitHub) using workspace config.
	// If noCache is true, cached data is bypassed.
	Search(ctx context.Context, filter string, noCache bool) ([]*WorkItem, error)

	// Get returns a single work item by its backend-specific ID.
	Get(ctx context.Context, id string) (*WorkItem, error)

	// Create persists a new work item and returns its assigned ID.
	Create(ctx context.Context, item *WorkItem) (string, error)

	// Update applies a set of changes to an existing work item.
	// The provider handles backend-specific mechanics (e.g., Jira
	// transitions for status changes, column moves for GitHub/Trello).
	Update(ctx context.Context, id string, changes *Changes) error

	// Comment adds a comment to a work item.
	Comment(ctx context.Context, id string, body string) error

	// Assign assigns the work item to the current authenticated user.
	Assign(ctx context.Context, id string) error

	// CurrentUser returns the authenticated user's identity.
	CurrentUser(ctx context.Context) (*User, error)

	// Capabilities returns the set of features this provider supports.
	// The UI layer uses this to gate feature visibility.
	Capabilities() Capabilities

	// ContentRenderer returns the provider's content format converter.
	ContentRenderer() ContentRenderer

	// FieldDefinitions returns metadata describing the provider's fields.
	// This drives manifest serialization, schema generation, and diff/apply
	// behaviour. Each FieldDef declares its type, visibility, and whether
	// it should be hoisted to the top level of the manifest YAML.
	FieldDefinitions() []FieldDef
}

// User represents an authenticated user across any backend.
type User struct {
	ID          string `json:"id"` // Backend-specific ID (accountId for Jira, login for GitHub)
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// Capabilities describes which optional features a provider supports.
// The UI layer checks these to decide what to render.
type Capabilities struct {
	HasSprints      bool // Jira-specific sprint management
	HasHierarchy    bool // Parent/child relationships (strong in Jira, weak in GitHub)
	HasTransitions  bool // Explicit workflow transitions (vs. direct status set)
	HasCustomFields bool // Backend supports arbitrary custom fields
	HasTypes        bool // Distinct issue types (vs. labels/convention)
	HasPriority     bool
	HasComponents   bool
}

// Changes represents a set of modifications to apply to a work item.
// Pointer fields use nil to indicate "no change". Fields map holds
// backend-specific changes (priority, parent, sprint, etc.).
type Changes struct {
	Summary  *string // nil = no change
	Type     *string
	Status   *string
	ParentID *string

	// Description AST — nil means no change.
	// Provider converts to native format via ContentRenderer.
	Description *document.Node

	// Backend-specific field changes (priority, parent, sprint, etc.)
	Fields map[string]any
}

// FieldType describes the data type of a provider field.
type FieldType string

const (
	FieldString      FieldType = "string"
	FieldEnum        FieldType = "enum"
	FieldStringArray FieldType = "string_array"
	FieldBool        FieldType = "bool"
	FieldAssignee    FieldType = "assignee" // User field that accepts "unassigned" / "none" to clear.
	FieldEmail       FieldType = "email"    // String validated as email format (e.g. reporter).
)

// FieldVisibility controls when a field appears in exports and whether
// it participates in diff/apply.
type FieldVisibility string

const (
	// FieldDefault fields are always included in export and diffed on apply.
	FieldDefault FieldVisibility = "default"
	// FieldExtended fields are only exported with --full but still diffed on apply.
	FieldExtended FieldVisibility = "extended"
	// FieldReadOnly fields are only exported with --full and never diffed.
	FieldReadOnly FieldVisibility = "readonly"
)

// FieldDef describes a single provider-specific field. Providers return
// a slice of these from FieldDefinitions() to drive manifest serialization,
// JSON Schema generation, and apply diffing.
type FieldDef struct {
	Key        string          `json:"key"`        // Map key in WorkItem.Fields (e.g. "priority", "assignee").
	Label      string          `json:"label"`      // Human-readable display name (e.g. "Priority", "Assignee").
	Type       FieldType       `json:"type"`       // Data type for schema generation and diff comparison.
	Enum       []string        `json:"enum"`       // Valid values when Type is FieldEnum.
	Visibility FieldVisibility `json:"visibility"` // Controls export inclusion and diff behaviour.
	TopLevel   bool            `json:"topLevel"`   // If true, serialize at item level rather than in the fields bag.
}

// ContentRenderer converts between a provider's native content format
// and the document AST used internally.
type ContentRenderer interface {
	// ParseContent converts from the backend's native format into an AST.
	// For Jira: raw is ADF JSON (map[string]any). For GitHub: raw is a markdown string.
	ParseContent(raw any) (*document.Node, error)

	// RenderContent converts an AST into the backend's native format.
	// For Jira: returns ADF map. For GitHub: returns markdown string.
	RenderContent(node *document.Node) (any, error)
}
