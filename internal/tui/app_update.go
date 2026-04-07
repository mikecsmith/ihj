package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// ── Constants ───────────────────────────────────────────────────

const (
	// notifyAutoClearDuration is how long a notification stays visible
	// before the tick handler clears it.
	notifyAutoClearDuration = 4 * time.Second

	// scrollLines is the number of lines scrolled per mouse wheel tick.
	scrollLines = 1

	// listHeaderRows is the number of header rows above the first data
	// row in the list pane (column header). Used to translate mouse Y
	// coordinates to list indices.
	listHeaderRows = 1
)

// ── Message dispatcher ──────────────────────────────────────────

// Update is the Bubble Tea message handler. It routes messages to domain-
// specific handlers rather than inlining all logic in a single switch.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Resize ──
	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	// ── User input ──
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.MouseWheelMsg:
		if m.popup.Active() {
			return m, nil
		}
		return m.handleMouseWheel(msg)

	case tea.MouseClickMsg:
		if m.popup.Active() {
			return m, nil
		}
		return m.handleMouseClick(msg)

	// ── Tick ──
	case tickMsg:
		if m.notify != "" && !m.notifyAt.IsZero() && time.Since(m.notifyAt) > notifyAutoClearDuration {
			m.notify = ""
		}
		return m, m.tickCmd()

	// ── Bridge events ──
	case bridgeSelectMsg:
		m.popup.ShowSelect("bridge-select", msg.title, msg.options)
		m.ui.Emit(EventPopupSelect, "title", msg.title)
		return m, nil

	case bridgeConfirmMsg:
		m.popup.ShowSelect("bridge-confirm", msg.prompt, []string{"Yes", "No"})
		m.ui.Emit(EventPopupConfirm, "title", msg.prompt)
		return m, nil

	case bridgeInputMsg:
		m.popup.ShowInput("bridge-input", msg.prompt, msg.initial)
		m.ui.Emit(EventPopupInput, "title", msg.prompt)
		return m, nil

	case bridgeEditDocMsg:
		return m.handleBridgeEditDoc(msg)

	case bridgeEditorDoneMsg:
		m.ui.resolveEditDoc(msg.content, msg.err)
		return m, nil

	// ── Command lifecycle ──
	case commandCompleteMsg:
		return m.handleCommandComplete(msg)

	// ── Data lifecycle ──
	case userFetchedMsg:
		if msg.err == nil && msg.displayName != "" {
			m.cachedUserName = msg.displayName
		}
		return m, nil

	case dataReloadedMsg:
		return m.handleDataReloaded(msg)

	case workspaceSwitchedMsg:
		return m.handleWorkspaceSwitched(msg)

	// ── Notifications ──
	case notifyMsg:
		m.setNotify(msg.title + ": " + msg.message)
		return m, nil

	case statusMsg:
		m.setNotify(string(msg))
		return m, nil
	}

	// Pass through to list (search input etc).
	var cmd tea.Cmd
	previousCursor := m.list.cursor
	m.list, cmd = m.list.Update(msg)
	if m.list.cursor != previousCursor {
		m.syncDetail()
	}
	return m, cmd
}

// ── Resize ──────────────────────────────────────────────────────

func (m AppModel) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	firstRender := !m.ready
	m.width, m.height = msg.Width, msg.Height
	m.ready = true
	m.recalcLayout()
	m.popup.SetSize(m.width, m.height)
	if firstRender {
		m.syncDetail()
		m.ui.Emit(EventReady)
	}
	return m, nil
}

// ── Keyboard input ──────────────────────────────────────────────

func (m AppModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Help overlay: help key dismisses it; other keys pass through.
	if m.showHelp && key.Matches(msg, m.keys.Help) {
		m.showHelp = false
		return m, nil
	}
	// Popup intercepts all keys while active.
	if m.popup.Active() {
		cmd, result := m.popup.Update(msg)
		if result != nil {
			return m.handlePopupResult(result)
		}
		return m, cmd
	}
	return m.handleKey(msg)
}

// ── Mouse input ─────────────────────────────────────────────────

func (m AppModel) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	mouseRow := msg.Mouse().Y
	inDetailPane := mouseRow >= m.detailTop && mouseRow < m.detailBottom
	inListPane := mouseRow >= m.listTop && mouseRow < m.listBottom

	switch msg.Button {
	case tea.MouseWheelUp:
		if inDetailPane {
			m.detail.ScrollUp(scrollLines)
		} else if inListPane {
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
		}
	case tea.MouseWheelDown:
		if inDetailPane {
			m.detail.ScrollDown(scrollLines)
		} else if inListPane {
			if m.list.cursor < len(m.list.filtered)-1 {
				m.list.cursor++
				m.syncDetail()
			}
		}
	}
	return m, nil
}

func (m AppModel) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	mouseRow := msg.Mouse().Y
	inListPane := mouseRow >= m.listTop && mouseRow < m.listBottom
	if !inListPane {
		return m, nil
	}

	clickedRow := mouseRow - m.listTop - listHeaderRows
	if clickedRow < 0 {
		return m, nil
	}
	targetIndex := m.list.offset + clickedRow
	if targetIndex >= 0 && targetIndex < len(m.list.filtered) {
		m.list.cursor = targetIndex
		m.syncDetail()
	}
	return m, nil
}

// ── Detail sync ─────────────────────────────────────────────────

func (m *AppModel) syncDetail() {
	// Don't reset the detail view while the user has navigated into
	// a child hierarchy — SetIssue clears history on mismatch.
	if m.detail.CanGoBack() {
		return
	}
	if selected := m.list.SelectedIssue(); selected != nil {
		m.detail.SetIssue(selected)
	}
}

// ── Bridge editor ───────────────────────────────────────────────

// handleBridgeEditDoc prepares the editor and returns tea.ExecProcess to
// suspend the TUI while $EDITOR runs.
func (m AppModel) handleBridgeEditDoc(msg bridgeEditDocMsg) (tea.Model, tea.Cmd) {
	process, tempPath, err := terminal.PrepareEditor(m.ui.EditorCmd, msg.initial, msg.prefix, 0, "")
	if err != nil {
		m.ui.resolveEditDoc("", err)
		return m, nil
	}

	return m, tea.ExecProcess(process, func(err error) tea.Msg {
		defer func() { _ = os.Remove(tempPath) }()

		if err != nil {
			return bridgeEditorDoneMsg{err: fmt.Errorf("editor error: %w", err)}
		}
		content, readErr := os.ReadFile(tempPath)
		if readErr != nil {
			return bridgeEditorDoneMsg{err: fmt.Errorf("reading editor output: %w", readErr)}
		}
		return bridgeEditorDoneMsg{content: string(content)}
	})
}

// ── Command lifecycle ───────────────────────────────────────────

func (m AppModel) handleCommandComplete(msg commandCompleteMsg) (tea.Model, tea.Cmd) {
	m.commandRunning = false
	if msg.err != nil {
		if !commands.IsCancelled(msg.err) {
			m.setNotify("Error: " + msg.err.Error())
		} else {
			m.setNotify("Cancelled")
		}
	}
	// Reload data from API to pick up any changes.
	return m, m.fetchData(m.filter, fetchOpts{silent: true})
}

// ── Data lifecycle ──────────────────────────────────────────────

// handleDataReloaded processes fresh issue data after a filter switch or refresh.
func (m AppModel) handleDataReloaded(msg dataReloadedMsg) (tea.Model, tea.Cmd) {
	m.loading = ""
	if msg.err != nil {
		if msg.startup {
			m.fatalErr = msg.err
			return m, m.quitCmd()
		}
		m.setNotify("Reload error: " + msg.err.Error())
		return m, nil
	}

	m.filter = msg.filter
	m.fetchedAt = msg.fetchedAt
	m.registry = core.BuildRegistry(msg.items)
	core.LinkChildren(m.registry)
	m.list.Rebuild(m.registry)
	m.detail.UpdateRegistry(m.registry)
	m.syncDetail()

	if !msg.silent {
		m.setNotify(fmt.Sprintf("Loaded %d issues (%s)", len(msg.items), strings.ToUpper(msg.filter)))
	}
	return m, nil
}

// handleWorkspaceSwitched processes the result of a workspace switch request.
// Swaps session, workspace, styles, and rebuilds all sub-models.
func (m AppModel) handleWorkspaceSwitched(msg workspaceSwitchedMsg) (tea.Model, tea.Cmd) {
	m.loading = ""
	if msg.err != nil {
		m.setNotify("Workspace error: " + msg.err.Error())
		return m, nil
	}

	m.wsSess = msg.wsSess
	m.ws = msg.wsSess.Workspace
	m.filter = commands.ResolveFilter("")
	m.styles = terminal.NewStyles(m.styles.Theme(), m.ws, m.runtime.Theme)

	m.caps = msg.wsSess.Provider.Capabilities()
	m.keys.Transition.SetEnabled(m.caps.HasTransitions)

	m.rebuildSubModels(msg.items, msg.fetchedAt)
	m.applyHelpStyles()

	m.setNotify(fmt.Sprintf("Switched to %s (%d issues)", m.ws.Name, len(msg.items)))
	return m, nil
}

// rebuildSubModels replaces the issue registry and propagates fresh styles
// and workspace config to all sub-models after a workspace switch.
func (m *AppModel) rebuildSubModels(items []*core.WorkItem, fetchedAt time.Time) {
	m.fetchedAt = fetchedAt
	m.registry = core.BuildRegistry(items)
	core.LinkChildren(m.registry)

	fieldDefs := m.ws.AllFieldDefs()
	m.list.styles = m.styles
	m.list.fieldDefs = fieldDefs
	m.list.statusOrder = m.ws.StatusOrderMap
	m.list.typeOrder = m.ws.TypeOrderMap
	m.list.Rebuild(m.registry)

	m.detail = NewDetailModel(m.styles, m.registry, m.ws, m.keys)
	m.detail.SetSize(m.detailContentW, m.detailContentH)

	m.popup.styles = m.styles
	m.popup.SetSize(m.width, m.height)

	m.syncDetail()
}

// applyHelpStyles propagates the current styles to the help bar model.
func (m *AppModel) applyHelpStyles() {
	styles := m.styles
	m.help.Styles.ShortKey = styles.ActionKey
	m.help.Styles.ShortDesc = styles.ActionDesc
	m.help.Styles.ShortSeparator = styles.ActionDesc
	m.help.Styles.FullKey = styles.ActionKey
	m.help.Styles.FullDesc = styles.ActionDesc
	m.help.Styles.FullSeparator = styles.ActionDesc
	m.help.Styles.Ellipsis = styles.ActionDesc
}
