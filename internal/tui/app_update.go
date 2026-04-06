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

// Update is the Bubble Tea message handler. It routes messages to domain-
// specific handlers rather than inlining all logic in a single switch.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Resize ──
	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	// ── User input ──
	case tea.KeyPressMsg:
		// If help overlay is showing, help key dismisses it; other keys pass through.
		if m.showHelp && key.Matches(msg, m.keys.Help) {
			m.showHelp = false
			return m, nil
		}
		// If popup is active, route all keys to it.
		if m.popup.Active() {
			cmd, result := m.popup.Update(msg)
			if result != nil {
				return m.handlePopupResult(result)
			}
			return m, cmd
		}
		return m.handleKey(msg)

	case tea.MouseWheelMsg:
		if m.popup.Active() {
			return m, nil // Ignore mouse while popup is open.
		}
		return m.handleMouseWheel(msg)

	case tea.MouseClickMsg:
		if m.popup.Active() {
			return m, nil
		}
		return m.handleMouseClick(msg)

	// ── Tick ──
	case tickMsg:
		// Auto-clear notifications after 4 seconds.
		if m.notify != "" && !m.notifyAt.IsZero() && time.Since(m.notifyAt) > 4*time.Second {
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
		m.commandRunning = false
		if msg.err != nil {
			if !commands.IsCancelled(msg.err) {
				m.setNotify("Error: " + msg.err.Error())
			} else {
				m.setNotify("Cancelled")
			}
		}
		// Reload data from API to pick up any changes.
		return m, m.fetchFreshDataSilent(m.filter)

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
	prev := m.list.cursor
	m.list, cmd = m.list.Update(msg)
	if m.list.cursor != prev {
		m.syncDetail()
	}
	return m, cmd
}

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

// handleBridgeEditDoc prepares the editor and returns tea.ExecProcess to
// suspend the TUI while $EDITOR runs.
func (m AppModel) handleBridgeEditDoc(msg bridgeEditDocMsg) (tea.Model, tea.Cmd) {
	proc, tmpPath, err := terminal.PrepareEditor(m.ui.EditorCmd, msg.initial, msg.prefix, 0, "")
	if err != nil {
		m.ui.resolveEditDoc("", err)
		return m, nil
	}

	return m, tea.ExecProcess(proc, func(err error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()

		if err != nil {
			return bridgeEditorDoneMsg{err: fmt.Errorf("editor error: %w", err)}
		}
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return bridgeEditorDoneMsg{err: fmt.Errorf("reading editor output: %w", readErr)}
		}
		return bridgeEditorDoneMsg{content: string(content)}
	})
}

func (m AppModel) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	y := msg.Mouse().Y
	switch msg.Button {
	case tea.MouseWheelUp:
		if y >= m.detailTop && y < m.detailBottom {
			m.detail.ScrollUp(1)
		} else if y >= m.listTop && y < m.listBottom {
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
		}
	case tea.MouseWheelDown:
		if y >= m.detailTop && y < m.detailBottom {
			m.detail.ScrollDown(1)
		} else if y >= m.listTop && y < m.listBottom {
			if m.list.cursor < len(m.list.filtered)-1 {
				m.list.cursor++
				m.syncDetail()
			}
		}
	}
	return m, nil
}

func (m AppModel) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button == tea.MouseLeft {
		y := msg.Mouse().Y
		if y >= m.listTop && y < m.listBottom {
			clickedRow := y - m.listTop - 1
			if clickedRow >= 0 {
				targetIdx := m.list.offset + clickedRow
				if targetIdx >= 0 && targetIdx < len(m.list.filtered) {
					m.list.cursor = targetIdx
					m.syncDetail()
				}
			}
		}
	}
	return m, nil
}

func (m *AppModel) syncDetail() {
	// Don't reset the detail view while the user has navigated into
	// a child hierarchy — SetIssue clears history on mismatch.
	if m.detail.CanGoBack() {
		return
	}
	if sel := m.list.SelectedIssue(); sel != nil {
		m.detail.SetIssue(sel)
	}
}

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
	// Replace the registry with fresh data.
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
	// Swap session, workspace, and rebuild everything.
	m.wsSess = msg.wsSess
	m.ws = msg.wsSess.Workspace
	m.filter = commands.ResolveFilter("")

	// Rebuild styles for the new workspace.
	m.styles = terminal.NewStyles(terminal.DefaultTheme(), m.ws, m.runtime.Theme)

	// Update capabilities and disable unsupported bindings.
	m.caps = msg.wsSess.Provider.Capabilities()
	m.keys.Transition.SetEnabled(m.caps.HasTransitions)

	// Rebuild data and update styles on sub-models.
	m.fetchedAt = msg.fetchedAt
	m.registry = core.BuildRegistry(msg.items)
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

	// Update help styles.
	m.help.Styles.ShortKey = m.styles.ActionKey
	m.help.Styles.ShortDesc = m.styles.ActionDesc
	m.help.Styles.ShortSeparator = m.styles.ActionDesc
	m.help.Styles.FullKey = m.styles.ActionKey
	m.help.Styles.FullDesc = m.styles.ActionDesc
	m.help.Styles.FullSeparator = m.styles.ActionDesc
	m.help.Styles.Ellipsis = m.styles.ActionDesc

	m.setNotify(fmt.Sprintf("Switched to %s (%d issues)", m.ws.Name, len(msg.items)))
	return m, nil
}
