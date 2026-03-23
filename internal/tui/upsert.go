package tui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
)

// --- Upsert state machine ---

type upsertPhase int

const (
	upsertIdle               upsertPhase = iota
	upsertAwaitingTypeSelect             // create: waiting for type popup
	upsertAwaitingEditor                 // editor running via tea.ExecProcess
	upsertAwaitingRecovery               // validation failed, recovery popup shown
)

// upsertContext holds state that persists across the upsert phases.
type upsertContext struct {
	opts       commands.UpsertOpts
	board      *config.BoardConfig
	schemaPath string
	metadata   map[string]string
	bodyText   string
	origStatus string
	tmpPath    string // temp file path, managed across phases
	initialDoc string // original doc for no-change detection
	cursorLine int
	searchPat  string
	edited     string            // content after editor returns
	fm         map[string]string // parsed frontmatter (for postUpsert)
}

// --- Upsert helper methods ---

// startUpsertPrepare runs the pre-editor phase as a tea.Cmd (edit mode).
func (m *AppModel) startUpsertPrepare(opts commands.UpsertOpts) tea.Cmd {
	app := m.app
	return func() tea.Msg {
		board, schemaPath, metadata, bodyText, origStatus,
			initialDoc, cursorLine, searchPat, err := commands.PrepareUpsert(app, opts)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}
		return upsertPreparedMsg{
			ctx: &upsertContext{
				opts: opts, board: board, schemaPath: schemaPath,
				metadata: metadata, bodyText: bodyText,
				origStatus: origStatus, initialDoc: initialDoc,
				cursorLine: cursorLine, searchPat: searchPat,
			},
		}
	}
}

// startUpsertPrepareCreate runs the pre-editor phase for create mode.
func (m *AppModel) startUpsertPrepareCreate(opts commands.UpsertOpts, selectedType string) tea.Cmd {
	app := m.app
	return func() tea.Msg {
		board, err := app.Config.ResolveBoard(opts.Board)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}

		schemaDict := config.FrontmatterSchema(app.Config, board)
		schemaPath, err := config.WriteFrontmatterSchema(app.CacheDir, board.Slug, schemaDict)
		if err != nil {
			return upsertPreparedMsg{err: fmt.Errorf("writing schema: %w", err)}
		}

		metadata, bodyText, origStatus := commands.PrepareCreateMetadata(app, board, opts, selectedType)
		initialDoc := config.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
		cursorLine, searchPat := commands.CalculateCursor(initialDoc, metadata["summary"])

		return upsertPreparedMsg{
			ctx: &upsertContext{
				opts: opts, board: board, schemaPath: schemaPath,
				metadata: metadata, bodyText: bodyText,
				origStatus: origStatus, initialDoc: initialDoc,
				cursorLine: cursorLine, searchPat: searchPat,
			},
		}
	}
}

// submitUpsert runs parsing, validation, and API submission as a tea.Cmd.
func (m *AppModel) submitUpsert() tea.Cmd {
	ctx := m.upsertCtx
	app := m.app
	return func() tea.Msg {
		issueKey, fm, recoverableMsg, err := commands.SubmitUpsert(
			app, ctx.board, ctx.opts, ctx.edited,
		)
		if recoverableMsg != "" {
			return upsertSubmitResultMsg{ctx: ctx, err: err, errMsg: recoverableMsg}
		}
		if err != nil {
			return upsertSubmitResultMsg{ctx: ctx, err: err}
		}
		action := "Updated"
		if !ctx.opts.IsEdit {
			action = "Created"
		}
		ctx.fm = fm // store for postUpsert
		return upsertSubmitResultMsg{
			ctx: ctx, issueKey: issueKey,
			notify: fmt.Sprintf("%s %s", action, issueKey),
		}
	}
}

// runPostUpsertAndRefetch runs post-upsert actions (sprint/transition),
// then re-fetches the issue from the API to get authoritative state.
func (m *AppModel) runPostUpsertAndRefetch(ctx *upsertContext, issueKey string) tea.Cmd {
	app := m.app
	isCreate := !ctx.opts.IsEdit
	parentKey := ""
	if ctx.fm != nil {
		parentKey = ctx.fm["parent"]
	}

	return tea.Batch(
		// 1. Handle background tasks (Sprints, Transitions)
		func() tea.Msg {
			notifications := commands.PostUpsertNotifications(
				app, ctx.board, ctx.fm, issueKey, ctx.origStatus,
			)
			return upsertPostDoneMsg{notifications: notifications}
		},
		// 2. Direct Fetch (Authoritative State)
		func() tea.Msg {
			// Hit the direct endpoint /issue/{key} - No JQL lag!
			view, err := jira.FetchIssueByKey(app.Client, issueKey, app.Config.FormattedCustomFields)
			if err != nil {
				return issueFetchedMsg{issueKey: issueKey, err: err}
			}

			return issueFetchedMsg{
				view:      view,
				issueKey:  issueKey,
				isCreate:  isCreate,
				parentKey: parentKey,
			}
		},
	)
}

// launchEditor prepares and launches the editor via tea.ExecProcess.
func (m *AppModel) launchEditor(ctx *upsertContext, content string, cursorLine int, searchPat string) (tea.Model, tea.Cmd) {
	btui := m.app.UI.(*BubbleTeaUI)
	proc, tmpPath, err := btui.PrepareEditor(content, "jira_", cursorLine, searchPat)
	if err != nil {
		m.upsertPhase = upsertIdle
		m.upsertCtx = nil
		m.setNotify("Error: " + err.Error())
		return m, nil
	}
	ctx.tmpPath = tmpPath
	m.upsertPhase = upsertAwaitingEditor
	return m, tea.ExecProcess(proc, func(err error) tea.Msg {
		if err != nil {
			return upsertEditorDoneMsg{ctx: ctx, err: err}
		}
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return upsertEditorDoneMsg{ctx: ctx, err: readErr}
		}
		ctx.edited = string(content)
		return upsertEditorDoneMsg{ctx: ctx}
	})
}
