package tui

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/storage"
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
	ws         *core.Workspace
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
		ws, schemaPath, metadata, bodyText, origStatus,
			initialDoc, cursorLine, searchPat, err := commands.PrepareUpsert(app, opts)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}
		return upsertPreparedMsg{
			ctx: &upsertContext{
				opts: opts, ws: ws, schemaPath: schemaPath,
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
		ws, err := app.Config.ResolveWorkspace(opts.Board)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}

		schemaDict := core.FrontmatterSchema(ws)
		schemaPath, err := storage.WriteSchema(app.CacheDir, ws.Slug, core.Frontmatter, schemaDict)
		if err != nil {
			return upsertPreparedMsg{err: fmt.Errorf("writing schema: %w", err)}
		}

		metadata, bodyText, origStatus := commands.PrepareCreateMetadata(app, ws, opts, selectedType)
		initialDoc := core.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
		cursorLine, searchPat := commands.CalculateCursor(initialDoc, metadata["summary"])

		return upsertPreparedMsg{
			ctx: &upsertContext{
				opts: opts, ws: ws, schemaPath: schemaPath,
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
			app, ctx.ws, ctx.opts, ctx.edited,
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

// runPostUpsertAndRefetch runs post-upsert actions (sprint/transition)
// FIRST, then re-fetches the issue from the API to get authoritative state.
// Sequential execution ensures the fetch reflects any completed transitions.
func (m *AppModel) runPostUpsertAndRefetch(ctx *upsertContext, issueKey string) tea.Cmd {
	app := m.app
	isCreate := !ctx.opts.IsEdit
	parentKey := ""
	if ctx.fm != nil {
		parentKey = ctx.fm["parent"]
	}

	return func() tea.Msg {
		// Step 1: Run post-upsert notifications (sprint + transition) FIRST.
		notifications := commands.PostUpsertNotifications(
			app, ctx.ws, ctx.fm, issueKey, ctx.origStatus,
		)

		// Step 2: NOW fetch the issue — it reflects the completed transition.
		item, fetchErr := app.Provider.Get(context.TODO(), issueKey)

		return postUpsertCompleteMsg{
			notifications: notifications,
			item:          item,
			issueKey:      issueKey,
			isCreate:      isCreate,
			parentKey:     parentKey,
			fetchErr:      fetchErr,
		}
	}
}

// launchEditor prepares and launches the editor via tea.ExecProcess.
func (m *AppModel) launchEditor(ctx *upsertContext, content string, cursorLine int, searchPat string) (tea.Model, tea.Cmd) {
	btui, ok := m.app.UI.(*BubbleTeaUI)
	if !ok {
		panic(fmt.Sprintf("fatal: expected m.app.UI to be *BubbleTeaUI, got %T", m.app.UI))
	}
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
