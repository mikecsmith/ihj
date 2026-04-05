package fakejira

import (
	"fmt"
	"time"
)

// Entity types are fakejira's internal, mutable representation of a
// Jira-flavoured demo dataset. The server converts these to wire types on
// every response so consumers always see a clean, freshly-serialised JSON
// body matching the real Jira API shape.

type entUser struct {
	AccountID   string
	DisplayName string
	Email       string
}

type entStatus struct {
	ID          string
	Name        string
	CategoryKey string // "new", "indeterminate", "done"
}

type entIssueType struct {
	ID      string
	Name    string
	Subtask bool
}

type entPriority struct {
	ID   string
	Name string
}

type entSprint struct {
	ID      int
	Name    string
	State   string // "active", "future", "closed"
	BoardID int
}

type entIssue struct {
	Key         string
	ID          string
	Summary     string
	Description any // ADF document (map[string]any) — passed through as-is

	TypeID     string
	StatusID   string
	PriorityID string
	AssigneeID string
	ReporterID string
	ParentKey  string

	Labels     []string
	Components []string
	SprintID   int // 0 = no sprint

	Comments []entComment
	Created  time.Time
	Updated  time.Time

	// Customs holds custom-field values keyed by "customfield_XXXXX".
	// Values are raw Go types that we JSON-encode on response.
	Customs map[string]any
}

type entComment struct {
	ID       string
	AuthorID string
	Body     any // ADF
	Created  time.Time
}

// statusCategoryName returns the display name for a given category key.
func statusCategoryName(key string) string {
	switch key {
	case "new":
		return "To Do"
	case "indeterminate":
		return "In Progress"
	case "done":
		return "Done"
	default:
		return key
	}
}

// statusCategoryID returns Jira's built-in statusCategory ID for a key.
func statusCategoryID(key string) int {
	switch key {
	case "new":
		return 2
	case "indeterminate":
		return 4
	case "done":
		return 3
	default:
		return 1
	}
}

// fmtISO formats a time in Jira's returned timestamp format.
func fmtISO(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000-0700")
}

// issueKey builds a project-scoped issue key.
func issueKey(project string, seq int64) string {
	return fmt.Sprintf("%s-%d", project, seq)
}
