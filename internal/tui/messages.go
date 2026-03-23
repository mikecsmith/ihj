package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/jira"
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

// commentDoneMsg carries a completed comment back to update the IssueView.
type commentDoneMsg struct {
	issueKey string
	comment  jira.CommentView
	err      error
}

// assignDoneMsg carries a completed assignment back to update the IssueView.
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
	user *client.User
	err  error
}

// dataReloadedMsg carries fresh issue data after a filter switch or refresh.
type dataReloadedMsg struct {
	filter    string
	views     []jira.IssueView
	fetchedAt time.Time
	err       error
}

// issueFetchedMsg carries a single re-fetched issue after upsert.
type issueFetchedMsg struct {
	view      *jira.IssueView
	issueKey  string
	isCreate  bool
	parentKey string // for create: parent from frontmatter
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

type upsertPostDoneMsg struct {
	notifications []string
}
