package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

type tickMsg time.Time

// tickCmd fires once per second to update the cache age display.
func (m AppModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// userFetchedMsg carries the cached user from the initial FetchMyself call.
type userFetchedMsg struct {
	displayName string
	err         error
}

// dataReloadedMsg carries fresh issue data after a filter switch or refresh.
type dataReloadedMsg struct {
	filter    string
	items     []*core.WorkItem
	fetchedAt time.Time
	err       error
	silent    bool // If true, skip the "Loaded N issues" notification.
}

// ── Bridge message types ──
// Sent by BubbleTeaUI methods via program.Send, handled by AppModel.Update.

type bridgeSelectMsg struct {
	title   string
	options []string
}

type bridgeConfirmMsg struct {
	prompt string
}

type bridgeInputMsg struct {
	prompt  string
	initial string
}

type bridgeEditDocMsg struct {
	initial string
	prefix  string
}

type bridgeEditorDoneMsg struct {
	content string
	err     error
}

// commandCompleteMsg is sent when a runCommand goroutine finishes.
type commandCompleteMsg struct {
	err error
}
