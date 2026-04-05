package fakejira

import "encoding/json"

// The types in this file mirror the on-the-wire shape of Jira's REST API
// responses, limited to the fields ihj's jira package actually parses.
//
// They are intentionally defined locally (rather than imported from the
// jira package) so fakejira can emit JSON without forcing every type in
// internal/jira to be exported. The wire format is the only contract.

type wireUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
	Active      bool   `json:"active"`
}

type wireStatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type wireStatus struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	StatusCategory wireStatusCategory `json:"statusCategory"`
}

type wireIssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type wirePriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireComponent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireParentRef struct {
	Key    string               `json:"key"`
	ID     string               `json:"id"`
	Fields *wireParentSubFields `json:"fields,omitempty"`
}

type wireParentSubFields struct {
	Summary   string        `json:"summary"`
	Status    wireStatus    `json:"status"`
	IssueType wireIssueType `json:"issuetype"`
}

type wireComment struct {
	ID      string          `json:"id"`
	Author  *wireUser       `json:"author"`
	Body    json.RawMessage `json:"body"`
	Created string          `json:"created"`
	Updated string          `json:"updated"`
}

type wireCommentPage struct {
	Comments   []wireComment `json:"comments"`
	MaxResults int           `json:"maxResults"`
	Total      int           `json:"total"`
	StartAt    int           `json:"startAt"`
}

type wireSprintRef struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type wireIssue struct {
	Key    string                     `json:"key"`
	ID     string                     `json:"id"`
	Self   string                     `json:"self"`
	Fields map[string]json.RawMessage `json:"fields"`
}

type wireTransition struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	To   wireStatus `json:"to"`
}

type wireSearchResponse struct {
	Issues        []wireIssue `json:"issues"`
	Total         int         `json:"total"`
	NextPageToken string      `json:"nextPageToken,omitempty"`
	IsLast        bool        `json:"isLast"`
}

type wireTransitionsResponse struct {
	Transitions []wireTransition `json:"transitions"`
}

type wireSprintList struct {
	Values []wireSprintRef `json:"values"`
}

type wireCreateMetaIssueTypeList struct {
	IssueTypes []wireCreateMetaIssueType `json:"issueTypes"`
	Total      int                       `json:"total"`
}

type wireCreateMetaIssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type wireCreateMetaFieldList struct {
	Fields     []wireCreateMetaField `json:"fields"`
	MaxResults int                   `json:"maxResults"`
	StartAt    int                   `json:"startAt"`
	Total      int                   `json:"total"`
}

type wireCreateMetaField struct {
	FieldID       string           `json:"fieldId"`
	Key           string           `json:"key"`
	Name          string           `json:"name"`
	Required      bool             `json:"required"`
	HasDefault    bool             `json:"hasDefaultValue"`
	Operations    []string         `json:"operations"`
	Schema        wireFieldSchema  `json:"schema"`
	AllowedValues []map[string]any `json:"allowedValues,omitempty"`
}

type wireFieldSchema struct {
	Type     string `json:"type"`
	System   string `json:"system,omitempty"`
	Items    string `json:"items,omitempty"`
	Custom   string `json:"custom,omitempty"`
	CustomID int    `json:"customId,omitempty"`
}

type wireCreatedIssue struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}
