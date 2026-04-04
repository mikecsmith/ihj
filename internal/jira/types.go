package jira

import "encoding/json"

// issue is the top-level issue object returned by search and get endpoints.
// Spec ref: IssueBean
type issue struct {
	Key    string      `json:"key"`
	ID     string      `json:"id"`
	Self   string      `json:"self"`
	Fields issueFields `json:"fields"`
}

// issueFields contains the standard and custom fields on an issue.
// Custom fields are captured in the Customs map via a custom unmarshaler.
// Spec ref: IssueBean.fields (additionalProperties: true)
type issueFields struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description,omitempty"` // ADF document, parsed by document package
	IssueType   issueType       `json:"issuetype"`
	Status      status          `json:"status"`
	Priority    priority        `json:"priority"`
	Assignee    *user           `json:"assignee"`
	Reporter    *user           `json:"reporter"`
	Parent      *parentRef      `json:"parent,omitempty"`
	Labels      []string        `json:"labels"`
	Components  []component     `json:"components"`
	Comment     *commentPage    `json:"comment,omitempty"`
	Created     string          `json:"created"`
	Updated     string          `json:"updated"`

	// Customs captures any field not listed above (customfield_XXXXX, etc).
	// Populated by the custom unmarshaler.
	Customs map[string]json.RawMessage `json:"-"`
}

// issueType represents the type of a Jira issue.
// Spec ref: IssueTypeDetails
type issueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
	Self    string `json:"self,omitempty"`
}

// status represents the workflow status of an issue.
// Spec ref: StatusDetails
type status struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Self           string         `json:"self,omitempty"`
	StatusCategory statusCategory `json:"statusCategory"`
}

// statusCategory groups statuses into buckets (to-do, in-progress, done).
// Spec ref: StatusCategory
type statusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"` // "new", "indeterminate", "done"
	Name string `json:"name"`
}

// priority represents issue priority.
// Spec ref: Priority
type priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// user represents a Jira user (assignee, reporter, comment author).
// Spec ref: UserDetails
type user struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
	Active      bool   `json:"active"`
	Self        string `json:"self,omitempty"`
}

// DisplayNameOrDefault returns the display name, falling back to a default.
func (u *user) DisplayNameOrDefault(fallback string) string {
	if u == nil || u.DisplayName == "" {
		return fallback
	}
	return u.DisplayName
}

// EmailOrDefault returns the email address, falling back to a default.
func (u *user) EmailOrDefault(fallback string) string {
	if u == nil || u.Email == "" {
		return fallback
	}
	return u.Email
}

// parentRef is a minimal reference to a parent issue.
// Spec ref: IssueBean.fields.parent (subset of IssueBean)
type parentRef struct {
	Key    string `json:"key"`
	ID     string `json:"id"`
	Fields *struct {
		Summary   string    `json:"summary"`
		Status    status    `json:"status"`
		IssueType issueType `json:"issuetype"`
	} `json:"fields,omitempty"`
}

// component represents a project component.
// Spec ref: ProjectComponent (subset)
type component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// commentPage wraps the paginated comment list embedded in issue fields.
// Spec ref: PageOfComments
type commentPage struct {
	Comments   []comment `json:"comments"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
}

// comment represents a single issue comment.
// Spec ref: Comment
type comment struct {
	ID      string          `json:"id"`
	Author  *user           `json:"author"`
	Body    json.RawMessage `json:"body"` // ADF document
	Created string          `json:"created"`
	Updated string          `json:"updated"`
}

// searchRequest is the POST body for /rest/api/3/search/jql.
// Spec ref: SearchRequestBean
type searchRequest struct {
	JQL           string   `json:"jql"`
	Fields        []string `json:"fields"`
	MaxResults    int      `json:"maxResults"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// searchResponse is the response from the search endpoint.
// Spec ref: SearchResults
type searchResponse struct {
	Issues        []issue `json:"issues"`
	Total         int     `json:"total"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	IsLast        bool    `json:"isLast"`
}

// transitionsResponse wraps the list returned by GET /issue/{key}/transitions.
// Spec ref: Transitions
type transitionsResponse struct {
	Transitions []transition `json:"transitions"`
}

// transition represents a valid status change for an issue.
// Spec ref: IssueTransition
type transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   status `json:"to"`
}

// fieldDefinition from GET /rest/api/3/field.
// Spec ref: FieldDetails
type fieldDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// createMetaIssueType from GET /rest/api/3/issue/createmeta/{projectKey}/issuetypes.
type createMetaIssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// createMetaIssueTypeList wraps the paginated createmeta issue type list.
type createMetaIssueTypeList struct {
	IssueTypes []createMetaIssueType `json:"issueTypes"`
	Total      int                   `json:"total"`
}

// createMetaField from GET /rest/api/3/issue/createmeta/{projectKey}/issuetypes/{issueTypeId}.
type createMetaField struct {
	FieldID       string          `json:"fieldId"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Required      bool            `json:"required"`
	HasDefault    bool            `json:"hasDefaultValue"`
	Operations    []string        `json:"operations"`
	Schema        fieldSchema     `json:"schema"`
	AllowedValues json.RawMessage `json:"allowedValues,omitempty"`
}

// createMetaFieldList wraps the paginated createmeta field list.
type createMetaFieldList struct {
	Fields     []createMetaField `json:"fields"`
	MaxResults int               `json:"maxResults"`
	StartAt    int               `json:"startAt"`
	Total      int               `json:"total"`
}

// fieldSchema describes the type metadata for a Jira field.
type fieldSchema struct {
	Type     string `json:"type"`
	System   string `json:"system,omitempty"`
	Items    string `json:"items,omitempty"`
	Custom   string `json:"custom,omitempty"`
	CustomID int    `json:"customId,omitempty"`
}

// project from GET /rest/api/3/project/{key}.
// Spec ref: Project (subset)
type project struct {
	ID         string      `json:"id"`
	Key        string      `json:"key"`
	Name       string      `json:"name"`
	IssueTypes []issueType `json:"issueTypes"`
}

// jiraFilter from GET /rest/api/3/filter/{id}.
// Spec ref: Filter (subset)
type jiraFilter struct {
	ID  string `json:"id"`
	JQL string `json:"jql"`
}

// agileBoard from GET /rest/agile/1.0/board.
type agileBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "scrum", "kanban", "simple"
}

// agileBoardList wraps the paginated board list.
type agileBoardList struct {
	Values []agileBoard `json:"values"`
}

// sprint from GET /rest/agile/1.0/board/{id}/sprint.
type sprint struct {
	ID    int    `json:"id"`
	State string `json:"state"` // "active", "closed", "future"
	Name  string `json:"name"`
}

// sprintList wraps the paginated sprint list.
type sprintList struct {
	Values []sprint `json:"values"`
}

// boardConfiguration from GET /rest/agile/1.0/board/{id}/configuration.
type boardConfiguration struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Filter struct {
		ID string `json:"id"`
	} `json:"filter"`
	ColumnConfig struct {
		Columns []boardColumn `json:"columns"`
	} `json:"columnConfig"`
}

// boardColumn represents a single column in the board configuration.
type boardColumn struct {
	Name     string `json:"name"`
	Statuses []struct {
		ID string `json:"id"`
	} `json:"statuses"`
}

// createdIssue is the response from POST /rest/api/3/issue.
// Spec ref: CreatedIssue
type createdIssue struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// UnmarshalJSON implements custom unmarshaling for issueFields to capture
// both known fields and arbitrary custom fields (customfield_XXXXX).
func (f *issueFields) UnmarshalJSON(data []byte) error {
	// First unmarshal known fields using an alias to avoid recursion.
	type Alias issueFields
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*f = issueFields(alias)

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
func (f *issueFields) CustomString(fieldID string) string {
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

// CustomSprint extracts the active sprint name from a sprint custom field.
// Jira returns sprints as a JSON array of objects with id, state, and name.
// Returns the name of the first "active" sprint, falling back to the most
// recent sprint by array position.
func (f *issueFields) CustomSprint(fieldID string) string {
	raw, ok := f.Customs[fieldID]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var sprints []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &sprints); err != nil || len(sprints) == 0 {
		return ""
	}

	// Prefer the active sprint.
	for _, s := range sprints {
		if s.State == "active" {
			return s.Name
		}
	}
	// Fall back to the last sprint in the array.
	return sprints[len(sprints)-1].Name
}
