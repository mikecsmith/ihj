// Package client provides typed HTTP access to the Jira REST API (v3)
// and the Jira Software Agile API.
//
// Types are derived from the Atlassian OpenAPI spec at:
//   https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
//
// Only the subset of schemas that ihj actually uses are included.
// Field names match the JSON keys from the spec.
package client

import "encoding/json"

// ──────────────────────────────────────────────────────────────
// Core Issue Types (from IssueBean / Fields in the OpenAPI spec)
// ──────────────────────────────────────────────────────────────

// Issue is the top-level issue object returned by search and get endpoints.
// Spec ref: IssueBean
type Issue struct {
	Key    string      `json:"key"`
	ID     string      `json:"id"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains the standard and custom fields on an issue.
// Custom fields are captured in the Customs map via a custom unmarshaler.
// Spec ref: IssueBean.fields (additionalProperties: true)
type IssueFields struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description,omitempty"` // ADF document, parsed by document package
	IssueType   IssueType       `json:"issuetype"`
	Status      Status          `json:"status"`
	Priority    Priority        `json:"priority"`
	Assignee    *User           `json:"assignee"`
	Reporter    *User           `json:"reporter"`
	Parent      *ParentRef      `json:"parent,omitempty"`
	Labels      []string        `json:"labels"`
	Components  []Component     `json:"components"`
	Comment     *CommentPage    `json:"comment,omitempty"`
	Created     string          `json:"created"`
	Updated     string          `json:"updated"`

	// Customs captures any field not listed above (customfield_XXXXX, etc).
	// Populated by the custom unmarshaler.
	Customs map[string]json.RawMessage `json:"-"`
}

// IssueType represents the type of a Jira issue.
// Spec ref: IssueTypeDetails
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
	Self    string `json:"self,omitempty"`
}

// Status represents the workflow status of an issue.
// Spec ref: StatusDetails
type Status struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Self           string         `json:"self,omitempty"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

// StatusCategory groups statuses into buckets (to-do, in-progress, done).
// Spec ref: StatusCategory
type StatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"` // "new", "indeterminate", "done"
	Name string `json:"name"`
}

// Priority represents issue priority.
// Spec ref: Priority
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// User represents a Jira user (assignee, reporter, comment author).
// Spec ref: UserDetails
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
	Active      bool   `json:"active"`
	Self        string `json:"self,omitempty"`
}

// DisplayNameOrDefault returns the display name, falling back to a default.
func (u *User) DisplayNameOrDefault(fallback string) string {
	if u == nil || u.DisplayName == "" {
		return fallback
	}
	return u.DisplayName
}

// ParentRef is a minimal reference to a parent issue.
// Spec ref: IssueBean.fields.parent (subset of IssueBean)
type ParentRef struct {
	Key    string `json:"key"`
	ID     string `json:"id"`
	Fields *struct {
		Summary   string    `json:"summary"`
		Status    Status    `json:"status"`
		IssueType IssueType `json:"issuetype"`
	} `json:"fields,omitempty"`
}

// Component represents a project component.
// Spec ref: ProjectComponent (subset)
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Comments
// ──────────────────────────────────────────────────────────────

// CommentPage wraps the paginated comment list embedded in issue fields.
// Spec ref: PageOfComments
type CommentPage struct {
	Comments   []Comment `json:"comments"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
}

// Comment represents a single issue comment.
// Spec ref: Comment
type Comment struct {
	ID      string          `json:"id"`
	Author  *User           `json:"author"`
	Body    json.RawMessage `json:"body"` // ADF document
	Created string          `json:"created"`
	Updated string          `json:"updated"`
}

// ──────────────────────────────────────────────────────────────
// Search
// ──────────────────────────────────────────────────────────────

// SearchRequest is the POST body for /rest/api/3/search/jql.
// Spec ref: SearchRequestBean
type SearchRequest struct {
	JQL           string   `json:"jql"`
	Fields        []string `json:"fields"`
	MaxResults    int      `json:"maxResults"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// SearchResponse is the response from the search endpoint.
// Spec ref: SearchResults
type SearchResponse struct {
	Issues        []Issue `json:"issues"`
	Total         int     `json:"total"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	IsLast        bool    `json:"isLast"`
}

// ──────────────────────────────────────────────────────────────
// Transitions
// ──────────────────────────────────────────────────────────────

// TransitionsResponse wraps the list returned by GET /issue/{key}/transitions.
// Spec ref: Transitions
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// Transition represents a valid status change for an issue.
// Spec ref: IssueTransition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

// ──────────────────────────────────────────────────────────────
// Fields, Statuses, Projects, Issue Types (metadata endpoints)
// ──────────────────────────────────────────────────────────────

// FieldDefinition from GET /rest/api/3/field.
// Spec ref: FieldDetails
type FieldDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Project from GET /rest/api/3/project/{key}.
// Spec ref: Project (subset)
type Project struct {
	ID         string      `json:"id"`
	Key        string      `json:"key"`
	Name       string      `json:"name"`
	IssueTypes []IssueType `json:"issueTypes"`
}

// Filter from GET /rest/api/3/filter/{id}.
// Spec ref: Filter (subset)
type Filter struct {
	ID  string `json:"id"`
	JQL string `json:"jql"`
}

// ──────────────────────────────────────────────────────────────
// Agile API types (Jira Software REST API)
// These come from /rest/agile/1.0/ endpoints, not the platform API.
// ──────────────────────────────────────────────────────────────

// AgileBoard from GET /rest/agile/1.0/board.
type AgileBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "scrum", "kanban", "simple"
}

// AgileBoardList wraps the paginated board list.
type AgileBoardList struct {
	Values []AgileBoard `json:"values"`
}

// Sprint from GET /rest/agile/1.0/board/{id}/sprint.
type Sprint struct {
	ID    int    `json:"id"`
	State string `json:"state"` // "active", "closed", "future"
	Name  string `json:"name"`
}

// SprintList wraps the paginated sprint list.
type SprintList struct {
	Values []Sprint `json:"values"`
}

// BoardConfiguration from GET /rest/agile/1.0/board/{id}/configuration.
type BoardConfiguration struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Filter struct {
		ID string `json:"id"`
	} `json:"filter"`
	ColumnConfig struct {
		Columns []BoardColumn `json:"columns"`
	} `json:"columnConfig"`
}

// BoardColumn represents a single column in the board configuration.
type BoardColumn struct {
	Name     string `json:"name"`
	Statuses []struct {
		ID string `json:"id"`
	} `json:"statuses"`
}

// ──────────────────────────────────────────────────────────────
// Create/Update response
// ──────────────────────────────────────────────────────────────

// CreatedIssue is the response from POST /rest/api/3/issue.
// Spec ref: CreatedIssue
type CreatedIssue struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// ──────────────────────────────────────────────────────────────
// Custom fields unmarshaling
// ──────────────────────────────────────────────────────────────

// UnmarshalJSON implements custom unmarshaling for IssueFields to capture
// both known fields and arbitrary custom fields (customfield_XXXXX).
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	// First unmarshal known fields using an alias to avoid recursion.
	type Alias IssueFields
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*f = IssueFields(alias)

	// Then capture everything into a raw map and extract custom fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Known field keys to exclude from Customs.
	known := map[string]bool{
		"summary": true, "description": true, "issuetype": true,
		"status": true, "priority": true, "assignee": true,
		"reporter": true, "parent": true, "labels": true,
		"components": true, "comment": true, "created": true,
		"updated": true, "subtasks": true,
	}

	f.Customs = make(map[string]json.RawMessage)
	for k, v := range raw {
		if !known[k] {
			f.Customs[k] = v
		}
	}

	return nil
}

// CustomString extracts a string value from a custom field.
// Handles both plain strings and {"value": "..."} / {"name": "..."} objects.
func (f *IssueFields) CustomString(fieldID string) string {
	raw, ok := f.Customs[fieldID]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	// Try plain string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try object with value or name.
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if v, ok := obj["value"].(string); ok {
			return v
		}
		if v, ok := obj["name"].(string); ok {
			return v
		}
	}

	return ""
}
