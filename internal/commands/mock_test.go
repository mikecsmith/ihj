package commands

import (
	"fmt"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/ui"
)

// Verify MockUI implements ui.UI at compile time.
var _ ui.UI = (*MockUI)(nil)

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
	ReviewDiffChanges []ui.Change
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

func (m *MockUI) ReviewDiff(title string, changes []ui.Change, options []string) (int, error) {
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

// NewTestApp creates an App with a mock UI and optional test client.
func NewTestApp(ui *MockUI) *App {
	return &App{
		Config: &testConfig,
		UI:     ui,
		Out:    &discardWriter{},
		Err:    &discardWriter{},
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// Minimal test config.
var testConfig = config.Config{
	Server:       "https://jira.test.com",
	DefaultBoard: "eng",
	CustomFields: map[string]int{"team": 15000},
	Boards: map[string]*config.BoardConfig{
		"eng": {
			ID:          1,
			Name:        "Engineering",
			Slug:        "eng",
			ProjectKey:  "ENG",
			JQL:         `project = "{project_key}"`,
			Filters:     map[string]string{"active": "status != Done"},
			Transitions: []string{"To Do", "In Progress", "Done"},
			Types: []config.IssueTypeConfig{
				{ID: 9, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},

				{ID: 10, Name: "Story", Order: 30, Color: "blue", HasChildren: true},
				{ID: 11, Name: "Task", Order: 30, Color: "default"},
				{ID: 13, Name: "Spike", Order: 30, Color: "yellow"},
				{ID: 12, Name: "Sub-task", Order: 40, Color: "white"},
			},
		},
	},
	FormattedCustomFields: map[string]string{"team": "cf[15000]", "team_id": "customfield_15000"},
}

// Need to import config - add at the top
func init() {
	// Ensure test config boards have computed fields.
	for slug, b := range testConfig.Boards {
		b.Slug = slug
		b.TypeOrderMap = make(map[string]config.TypeOrderEntry)
		for _, t := range b.Types {
			b.TypeOrderMap[fmt.Sprintf("%d", t.ID)] = config.TypeOrderEntry{
				Order: t.Order, Color: t.Color, HasChildren: t.HasChildren,
			}
		}
	}
}
