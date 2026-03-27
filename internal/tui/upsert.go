package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// --- Edit/Create state machine ---

type upsertPhase int

const (
	upsertIdle               upsertPhase = iota
	upsertAwaitingTypeSelect             // create: waiting for type popup
	upsertAwaitingEditor                 // editor running via tea.ExecProcess
	upsertAwaitingRecovery               // validation failed, recovery popup shown
)

// upsertContext holds state that persists across the edit/create phases.
type upsertContext struct {
	isEdit     bool
	workspace  string
	issueKey   string // edit only
	overrides  map[string]string
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
	fm         map[string]string // parsed frontmatter (for post-actions)
}

// --- Edit/Create helper methods ---

// startEditPrepare runs the pre-editor phase for edit mode as a tea.Cmd.
func (m *AppModel) startEditPrepare(workspace, issueKey string, overrides map[string]string) tea.Cmd {
	s := m.session
	return func() tea.Msg {
		ws, schemaPath, metadata, bodyText, origStatus,
			initialDoc, cursorLine, searchPat, err := commands.PrepareEdit(s, workspace, issueKey, overrides)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}
		return upsertPreparedMsg{
			ctx: &upsertContext{
				isEdit: true, workspace: workspace, issueKey: issueKey,
				overrides: overrides, ws: ws, schemaPath: schemaPath,
				metadata: metadata, bodyText: bodyText,
				origStatus: origStatus, initialDoc: initialDoc,
				cursorLine: cursorLine, searchPat: searchPat,
			},
		}
	}
}

// startCreatePrepare runs the pre-editor phase for create mode as a tea.Cmd.
func (m *AppModel) startCreatePrepare(workspace, selectedType string, overrides map[string]string) tea.Cmd {
	s := m.session
	return func() tea.Msg {
		ws, schemaPath, metadata, bodyText, origStatus,
			initialDoc, cursorLine, searchPat, err := commands.PrepareCreate(s, workspace, selectedType, overrides)
		if err != nil {
			return upsertPreparedMsg{err: err}
		}
		return upsertPreparedMsg{
			ctx: &upsertContext{
				isEdit: false, workspace: workspace,
				overrides: overrides, ws: ws, schemaPath: schemaPath,
				metadata: metadata, bodyText: bodyText,
				origStatus: origStatus, initialDoc: initialDoc,
				cursorLine: cursorLine, searchPat: searchPat,
			},
		}
	}
}

// submitMutation runs parsing, validation, and API submission as a tea.Cmd.
func (m *AppModel) submitMutation() tea.Cmd {
	ctx := m.upsertCtx
	s := m.session
	return func() tea.Msg {
		if ctx.isEdit {
			fm, recoverableMsg, err := commands.SubmitEdit(
				s, ctx.ws, ctx.issueKey, ctx.edited, ctx.origStatus,
			)
			if recoverableMsg != "" {
				return upsertSubmitResultMsg{ctx: ctx, err: err, errMsg: recoverableMsg}
			}
			if err != nil {
				return upsertSubmitResultMsg{ctx: ctx, err: err}
			}
			ctx.fm = fm
			return upsertSubmitResultMsg{
				ctx: ctx, issueKey: ctx.issueKey,
				notify: fmt.Sprintf("Updated %s", ctx.issueKey),
			}
		}

		// Create flow.
		issueKey, fm, recoverableMsg, err := commands.SubmitCreate(s, ctx.edited)
		if recoverableMsg != "" {
			return upsertSubmitResultMsg{ctx: ctx, err: err, errMsg: recoverableMsg}
		}
		if err != nil {
			return upsertSubmitResultMsg{ctx: ctx, err: err}
		}
		ctx.fm = fm
		return upsertSubmitResultMsg{
			ctx: ctx, issueKey: issueKey,
			notify: fmt.Sprintf("Created %s", issueKey),
		}
	}
}

// runPostMutateAndRefetch runs post-mutation actions, then re-fetches
// the issue from the API to get authoritative state.
func (m *AppModel) runPostMutateAndRefetch(ctx *upsertContext, issueKey string) tea.Cmd {
	s := m.session
	isCreate := !ctx.isEdit
	parentKey := ""
	if ctx.fm != nil {
		parentKey = ctx.fm["parent"]
	}

	return func() tea.Msg {
		var notifications []string

		// Post-create: transition to target status if needed.
		if isCreate {
			if newStatus := ctx.fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, ctx.origStatus) {
				if err := s.Provider.Update(context.TODO(), issueKey, &core.Changes{Status: &newStatus}); err != nil {
					notifications = append(notifications, fmt.Sprintf("Warning: could not transition to '%s': %v", newStatus, err))
				} else {
					notifications = append(notifications, fmt.Sprintf("%s → %s", issueKey, newStatus))
				}
			}
		} else {
			// Edit: status transition already handled by SubmitEdit → Provider.Update.
			if newStatus := ctx.fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, ctx.origStatus) {
				notifications = append(notifications, fmt.Sprintf("%s → %s", issueKey, newStatus))
			}
		}

		// Sprint assignment via provider (for both create and edit).
		if strings.EqualFold(ctx.fm["sprint"], "true") {
			if err := s.Provider.Update(context.TODO(), issueKey, &core.Changes{
				Fields: map[string]any{"sprint": true},
			}); err != nil {
				notifications = append(notifications, fmt.Sprintf("Sprint Error: %v", err))
			} else {
				notifications = append(notifications, fmt.Sprintf("Added %s to active sprint", issueKey))
			}
		}

		// Re-fetch to get authoritative state.
		item, fetchErr := s.Provider.Get(context.TODO(), issueKey)

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
	btui, ok := m.session.UI.(*BubbleTeaUI)
	if !ok {
		panic(fmt.Sprintf("fatal: expected m.session.UI to be *BubbleTeaUI, got %T", m.session.UI))
	}
	proc, tmpPath, err := btui.PrepareEditor(content, "ihj_", cursorLine, searchPat)
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
