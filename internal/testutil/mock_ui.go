package testutil

import (
	"github.com/mikecsmith/ihj/internal/commands"
)

// Verify MockUI implements commands.UI at compile time.
var _ commands.UI = (*MockUI)(nil)

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
	ReviewDiffChanges []commands.FieldDiff
}

// Notification records a single Notify call.
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

func (m *MockUI) ReviewDiff(title string, changes []commands.FieldDiff, options []string) (int, error) {
	m.ReviewDiffChanges = changes
	return m.ReviewDiffReturn, m.ReviewDiffErr
}

func (m *MockUI) Status(message string) {
	// No-op in tests.
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

// LastNotification returns the most recent notification or panics.
func (m *MockUI) LastNotification() Notification {
	if len(m.Notifications) == 0 {
		panic("no notifications recorded")
	}
	return m.Notifications[len(m.Notifications)-1]
}
