package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/core"
)

// --- Tick ---

type tickMsg time.Time

// tickCmd fires once per second to update the cache age display.
func (m AppModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- Action result messages ---

// popupTransition holds a cached transition for mapping popup selection back to API call.
type popupTransition struct {
	ID   string
	Name string
}

// transitionsLoadedMsg is sent when async transition fetch completes.
type transitionsLoadedMsg struct {
	issueKey    string
	transitions []popupTransition
	err         error
}

// transitionDoneMsg carries a successful status change back to the TUI.
type transitionDoneMsg struct {
	issueKey  string
	newStatus string
	err       error
}

// commentDoneMsg carries a completed comment back to update the issue.
type commentDoneMsg struct {
	issueKey string
	comment  core.Comment
	err      error
}

// assignDoneMsg carries a completed assignment back to update the issue.
type assignDoneMsg struct {
	issueKey string
	assignee string
	err      error
}

type commandDoneMsg struct {
	err    error
	notify string
}

// --- Data lifecycle messages ---

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
}

// --- Upsert messages ---

type upsertPreparedMsg struct {
	ctx *upsertContext
	err error
}

type upsertEditorDoneMsg struct {
	ctx *upsertContext
	err error
}

type upsertSubmitResultMsg struct {
	ctx      *upsertContext
	issueKey string
	notify   string
	errMsg   string // non-empty = recoverable error
	err      error
}

// postUpsertCompleteMsg is the single result of the sequential post-upsert
// pipeline: notifications first (sprint/transition), then issue re-fetch.
// This avoids the race where a concurrent fetch could return stale state
// before the transition completes.
type postUpsertCompleteMsg struct {
	notifications []string
	item          *core.WorkItem
	issueKey      string
	isCreate      bool
	parentKey     string
	fetchErr      error
}
