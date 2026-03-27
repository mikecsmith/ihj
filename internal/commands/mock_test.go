package commands

import (
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/storage"
)

// Verify MockUI implements UI at compile time.
var _ UI = (*MockUI)(nil)

// MockUI records all UI interactions for test assertions.
type MockUI struct {
	// Select behavior.
	SelectReturn int
	SelectErr    error
	SelectCalls  []string

	// Confirm behavior.
	ConfirmReturn bool
	ConfirmErr    error

	// EditText behavior.
	EditTextReturn string
	EditTextErr    error
	EditTextCalls  int

	// Notify records.
	Notifications []Notification

	// Clipboard records.
	ClipboardContents string
	ClipboardErr      error

	// PromptText behavior.
	PromptReturn string
	PromptErr    error

	// ReviewDiff records.
	ReviewDiffReturn  int
	ReviewDiffErr     error
	ReviewDiffChanges []FieldDiff
}

type Notification struct {
	Title   string
	Message string
}

func (m *MockUI) Select(title string, options []string) (int, error) {
	m.SelectCalls = append(m.SelectCalls, title)
	return m.SelectReturn, m.SelectErr
}

func (m *MockUI) Confirm(prompt string) (bool, error) {
	return m.ConfirmReturn, m.ConfirmErr
}

func (m *MockUI) EditText(initial, prefix string, cursorLine int, searchPattern string) (string, error) {
	m.EditTextCalls++
	return m.EditTextReturn, m.EditTextErr
}

func (m *MockUI) Notify(title, message string) {
	m.Notifications = append(m.Notifications, Notification{Title: title, Message: message})
}

func (m *MockUI) CopyToClipboard(text string) error {
	m.ClipboardContents = text
	return m.ClipboardErr
}

func (m *MockUI) PromptText(prompt string) (string, error) {
	return m.PromptReturn, m.PromptErr
}

func (m *MockUI) ReviewDiff(title string, changes []FieldDiff, options []string) (int, error) {
	m.ReviewDiffChanges = changes
	return m.ReviewDiffReturn, m.ReviewDiffErr
}

func (m *MockUI) Status(message string) {
	// No-op in tests.
}

// LastNotification returns the most recent notification or panics.
func (m *MockUI) LastNotification() Notification {
	if len(m.Notifications) == 0 {
		panic("no notifications recorded")
	}
	return m.Notifications[len(m.Notifications)-1]
}

// HasNotification checks if any notification matches the title.
func (m *MockUI) HasNotification(title string) bool {
	for _, n := range m.Notifications {
		if n.Title == title {
			return true
		}
	}
	return false
}

// NewTestSession creates a Session with a mock UI for testing.
func NewTestSession(ui *MockUI) *Session {
	return &Session{
		Config: &testConfig,
		UI:     ui,
		Out:    &discardWriter{},
		Err:    &discardWriter{},
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// Minimal test config.
var testConfig = storage.AppConfig{
	DefaultWorkspace: "eng",
	Workspaces: map[string]*core.Workspace{
		"eng": {
			Slug:     "eng",
			Name:     "Engineering",
			Provider: "jira",
			BaseURL:  "https://jira.test.com",
			Filters:  map[string]string{"active": "status != Done"},
			Statuses: []string{"To Do", "In Progress", "Done"},
			Types: []core.TypeConfig{
				{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
				{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true},
				{ID: 11, Name: "Task", Order: 30, Color: "default"},
				{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
				{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
			},
			StatusWeights: map[string]int{
				"to do": 0, "in progress": 1, "done": 2,
			},
			TypeOrderMap: map[string]core.TypeOrderEntry{
				"Epic":     {Order: 20, Color: "magenta", HasChildren: true},
				"Story":    {Order: 30, Color: "blue", HasChildren: true},
				"Task":     {Order: 30, Color: "default"},
				"Spike":    {Order: 30, Color: "yellow"},
				"Sub-task": {Order: 40, Color: "white"},
			},
			ProviderConfig: &jira.Config{
				Server:                "https://jira.test.com",
				BoardID:               1,
				ProjectKey:            "ENG",
				JQL:                   `project = "{project_key}"`,
				CustomFields:          map[string]int{"team": 15000},
				FormattedCustomFields: map[string]string{"team": "cf[15000]", "team_id": "customfield_15000"},
			},
		},
	},
}
