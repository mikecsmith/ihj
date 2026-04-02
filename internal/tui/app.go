// Package tui implements the Bubble Tea terminal user interface for ihj.
//
// The main model is AppModel, which composes a list pane, detail pane,
// and popup overlay. BubbleTeaUI implements the commands.UI interface,
// bridging between the business logic layer and the interactive TUI.
package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// ViewState represents which pane the user is looking at and interacting with.
type ViewState int

const (
	ViewList       ViewState = iota // Split layout, list pane focused.
	ViewDetail                      // Split layout, detail pane focused.
	ViewFullscreen                  // Detail pane fills the entire terminal.
)

// InputCapture controls where keystrokes are routed.
// In default (non-vim) mode, this is always CaptureNone — unmatched keys
// fall through to the search input passively.
type InputCapture int

const (
	CaptureNone    InputCapture = iota // Keys handled by current pane (navigation, actions).
	CaptureSearch                      // Keys routed to search input (vim /).
	CaptureCommand                     // Keys routed to command buffer (vim :).
)

// AppModel is the top-level Bubble Tea model for the ihj TUI.
type AppModel struct {
	ctx     context.Context
	runtime *commands.Runtime
	wsSess  *commands.WorkspaceSession
	factory commands.WorkspaceSessionFactory
	ws      *core.Workspace
	filter  string

	list   ListModel
	detail DetailModel
	popup  PopupModel
	styles *terminal.Styles
	keys   terminal.KeyMap

	width, height int
	notify        string
	notifyAt      time.Time // When notify was set (for auto-clear).
	loading       string    // Non-empty = show loading indicator (e.g. "Fetching issues...").
	ready         bool

	// Cached current user — fetched once at init, used for comments/assign/create.
	cachedUserName string

	// Cache age tracking — elapsed since data was fetched.
	fetchedAt time.Time // Zero value = demo mode → show ∞.

	// Layout zones (computed in recalcLayout, used for mouse routing).
	previewTop    int // Y offset of preview area start.
	previewBottom int // Y offset of preview area end.
	listTop       int // Y offset of list area start.
	listBottom    int // Y offset of list area end.

	// Issue registry for lookups (shared with detail model).
	registry map[string]*core.WorkItem

	// Cached layout dimensions (computed in recalcLayout, used in View).
	innerW          int
	previewContentW int
	previewContentH int
	previewTotalH   int
	listH           int

	// Provider capabilities — cached at init for gating actions.
	caps core.Capabilities

	// Bridge UI reference — used to resolve channel-based interactive methods.
	ui *BubbleTeaUI

	// True while a runCommand goroutine is executing — suppresses action keys.
	commandRunning bool

	// vimMode enables vim-style key bindings (normal/search/command modes).
	vimMode bool
	capture InputCapture // Where keystrokes are routed (only non-None in vim mode).
	cmdBuf  string       // Buffer for ":" command input in command mode.

	// Help bubble — renders key bindings with width-aware truncation.
	help     help.Model
	showHelp bool // Toggle full help view via '?'.

	// View state: which pane is active and how it's arranged.
	view ViewState
	// Configurable detail pane height as a percentage (20-80, default 55).
	detailPct int

	// fatalErr is set when an unrecoverable error occurs (e.g. auth failure
	// on background refresh). The TUI quits and the caller reads the error.
	fatalErr error
}

// NewAppModel creates the TUI application model with the given data.
func NewAppModel(ctx context.Context, rt *commands.Runtime, wsSess *commands.WorkspaceSession, factory commands.WorkspaceSessionFactory, ws *core.Workspace, filter string, items []*core.WorkItem, fetchedAt time.Time, ui *BubbleTeaUI, vimMode bool, shortcuts map[string]string, detailPct int) AppModel {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, rt.Theme)
	keys := terminal.DefaultKeyMap()
	if vimMode {
		keys = terminal.VimKeyMap()
	} else {
		_ = keys.ApplyShortcuts(shortcuts) // Validated at config load.
	}

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	var caps core.Capabilities
	if wsSess.Provider != nil {
		caps = wsSess.Provider.Capabilities()
	}

	// Disable keybindings for unsupported capabilities.
	if !caps.HasTransitions {
		keys.Transition.SetEnabled(false)
	}

	h := help.New()
	h.ShortSeparator = " | "
	h.Styles.ShortKey = styles.ActionKey
	h.Styles.ShortDesc = styles.ActionDesc
	h.Styles.ShortSeparator = styles.ActionDesc
	h.Styles.FullKey = styles.ActionKey
	h.Styles.FullDesc = styles.ActionDesc
	h.Styles.FullSeparator = styles.ActionDesc
	h.Styles.Ellipsis = styles.ActionDesc

	if detailPct < 20 || detailPct > 80 {
		detailPct = 55
	}

	m := AppModel{
		ctx:     ctx,
		runtime: rt, wsSess: wsSess, factory: factory,
		ws: ws, filter: filter,
		list:      NewListModel(registry, styles, ws.StatusOrderMap, ws.TypeOrderMap),
		detail:    NewDetailModel(styles, registry, ws.Name, keys),
		popup:     NewPopupModel(styles, keys),
		styles:    styles,
		keys:      keys,
		registry:  registry,
		fetchedAt: fetchedAt,
		caps:      caps,
		ui:        ui,
		vimMode:   vimMode,
		help:      h,
		detailPct: detailPct,
	}

	// In vim mode, start in normal mode with search unfocused.
	if vimMode {
		m.list.search.Blur()
	}

	return m
}

// Err returns any fatal error that caused the TUI to exit.
func (m AppModel) Err() error { return m.fatalErr }

func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.list.Init(), m.detail.Init(), m.tickCmd()}
	if m.wsSess.Provider != nil {
		// Pre-fetch the current user for comments/assign/create.
		provider := m.wsSess.Provider
		cmds = append(cmds, func() tea.Msg {
			user, err := provider.CurrentUser(m.ctx)
			if err != nil {
				return userFetchedMsg{err: err}
			}
			return userFetchedMsg{displayName: user.DisplayName}
		})
		// Background refresh validates auth and replaces stale cache.
		// Skip for demo mode (zero fetchedAt) where there's no real server.
		if !m.fetchedAt.IsZero() {
			cmds = append(cmds, m.fetchStartupData(m.filter))
		}
	}
	return tea.Batch(cmds...)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		firstRender := !m.ready
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.recalcLayout()
		m.popup.SetSize(m.width, m.height)
		if firstRender {
			m.syncDetail()
			m.ui.Emit("ready")
		}
		return m, nil

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

	case tickMsg:
		// Auto-clear notifications after 4 seconds.
		if m.notify != "" && !m.notifyAt.IsZero() && time.Since(m.notifyAt) > 4*time.Second {
			m.notify = ""
		}
		return m, m.tickCmd()

	// ── Bridge message handlers ──

	case bridgeSelectMsg:
		m.popup.ShowSelect("bridge-select", msg.title, msg.options)
		m.ui.Emit("popup:select", "title", msg.title)
		return m, nil

	case bridgeConfirmMsg:
		m.popup.ShowSelect("bridge-confirm", msg.prompt, []string{"Yes", "No"})
		m.ui.Emit("popup:confirm", "title", msg.prompt)
		return m, nil

	case bridgeInputMsg:
		m.popup.ShowInput("bridge-input", msg.prompt, msg.initial)
		m.ui.Emit("popup:input", "title", msg.prompt)
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
		m.loading = ""
		if msg.err != nil {
			if msg.startup {
				m.fatalErr = msg.err
				return m, tea.Quit
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

	case workspaceSwitchedMsg:
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
		m.list.styles = m.styles
		m.list.statusOrder = m.ws.StatusOrderMap
		m.list.typeOrder = m.ws.TypeOrderMap
		m.list.Rebuild(m.registry)
		m.detail = NewDetailModel(m.styles, m.registry, m.ws.Name, m.keys)
		m.detail.SetSize(m.previewContentW, m.previewContentH)
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
		if y >= m.previewTop && y < m.previewBottom {
			m.detail.ScrollUp(1)
		} else if y >= m.listTop && y < m.listBottom {
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
		}
	case tea.MouseWheelDown:
		if y >= m.previewTop && y < m.previewBottom {
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

func (m AppModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.vimMode {
		return m.handleKeyVim(msg)
	}

	// Global keys.
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Cancel) {
		// Esc: exit detail view → clear search → quit.
		if m.exitDetailView() {
			return m, nil
		}
		if m.list.search.Value() != "" {
			m.list.search.SetValue("")
			m.list.applyFilter()
			return m, nil
		}
		return m, tea.Quit
	}

	// Backspace: navigate back through child history, or exit detail view.
	if msg.Code == tea.KeyBackspace && m.view >= ViewDetail {
		if m.detail.CanGoBack() {
			m.detail.GoBack()
			m.recalcLayout()
			iss := m.detail.Issue()
			if iss != nil {
				m.ui.Emit("back", "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
			}
		} else {
			m.exitDetailView()
		}
		return m, nil
	}

	// Toggle full help.
	if key.Matches(msg, m.keys.Help) {
		m.showHelp = !m.showHelp
		return m, nil
	}

	// Enter: enter fullscreen mode (detail pane fills screen).
	if key.Matches(msg, m.keys.Focus) {
		m.enterFullscreen()
		return m, nil
	}

	// Tab: toggle pane focus (only in split layout).
	if key.Matches(msg, m.keys.Tab) && m.view != ViewFullscreen {
		if m.view == ViewList {
			m.focusDetail()
		} else {
			m.focusList()
		}
		return m, nil
	}

	// Actions (resolved via KeyMap — don't interfere with search).
	if model, cmd, handled := m.executeAction(m.resolveAction(msg)); handled {
		return model, cmd
	}

	// Navigation keys — when detail is focused, scroll detail instead of list.
	if m.view >= ViewDetail {
		switch {
		case key.Matches(msg, m.keys.Up):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.detail.ScrollDown(1)
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.detail.ScrollUp(m.previewContentH)
			return m, nil
		case key.Matches(msg, m.keys.PageDn):
			m.detail.ScrollDown(m.previewContentH)
			return m, nil
		case key.Matches(msg, m.keys.Home):
			m.detail.ScrollToTop()
			return m, nil
		case key.Matches(msg, m.keys.End):
			m.detail.ScrollToBottom()
			return m, nil
		case key.Matches(msg, m.keys.PreviewUp):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewDown):
			m.detail.ScrollDown(1)
			return m, nil
		}
	} else {
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.list.cursor < len(m.list.filtered)-1 {
				m.list.cursor++
				m.syncDetail()
			}
			return m, nil
		case key.Matches(msg, m.keys.Home):
			m.list.cursor = 0
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.End):
			m.list.cursor = max(0, len(m.list.filtered)-1)
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.list.cursor = max(0, m.list.cursor-m.list.visibleRows())
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PageDn):
			m.list.cursor = min(len(m.list.filtered)-1, m.list.cursor+m.list.visibleRows())
			m.syncDetail()
			return m, nil
		case key.Matches(msg, m.keys.PreviewUp):
			m.detail.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewDown):
			m.detail.ScrollDown(1)
			return m, nil
		}
	}

	// Hint keys navigate to child issues when detail pane is active.
	if m.view >= ViewDetail {
		if s := msg.String(); len([]rune(s)) == 1 {
			if idx := m.detail.ChildIndexForKey([]rune(s)[0]); idx >= 0 {
				m.detail.NavigateToChild(idx)
				m.recalcLayout()
				iss := m.detail.Issue()
				if iss != nil {
					m.ui.Emit("navigated", "id", iss.ID, "breadcrumb", m.detail.Breadcrumb())
				}
				return m, nil
			}
		}
	}

	// Everything else goes to search input.
	var cmd tea.Cmd
	prevQuery := m.list.search.Value()
	m.list.search, cmd = m.list.search.Update(msg)
	if m.list.search.Value() != prevQuery {
		m.list.applyFilter()
		m.syncDetail()
	}
	return m, cmd
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

func (m AppModel) handlePopupResult(result *PopupResult) (tea.Model, tea.Cmd) {
	// ── Bridge popup results ──
	switch result.ID {
	case "bridge-select":
		idx := result.Index
		if result.Canceled {
			idx = -1
		}
		m.ui.resolveSelect(idx)
		return m, nil

	case "bridge-confirm":
		yes := !result.Canceled && result.Index == 0
		m.ui.resolveConfirm(yes)
		return m, nil

	case "bridge-input":
		m.ui.resolveInput(result.Text, result.Canceled)
		return m, nil
	}

	// ── TUI-only popup results ──
	if result.Canceled {
		m.setNotify("Cancelled")
		return m, nil
	}

	switch result.ID {
	case "filter":
		if result.Value != "" {
			// Strip the bullet/spacing prefix added for display.
			selected := strings.TrimPrefix(result.Value, "● ")
			selected = strings.TrimPrefix(selected, "  ")
			if selected == m.filter {
				m.setNotify("Already on filter: " + selected)
			} else {
				return m, m.switchFilter(selected)
			}
		}

	case "workspace":
		if result.Value != "" {
			// Strip the bullet/spacing prefix, then resolve slug from name.
			name := strings.TrimPrefix(result.Value, "● ")
			name = strings.TrimPrefix(name, "  ")
			slug := m.resolveWorkspaceSlug(name)
			if slug == m.ws.Slug {
				m.setNotify("Already on workspace: " + name)
			} else {
				return m, m.switchWorkspace(slug)
			}
		}
	}

	return m, nil
}

// resolveAction maps a key press to an Action using the default (alt-key) bindings.
func (m *AppModel) resolveAction(msg tea.KeyPressMsg) Action {
	switch {
	case key.Matches(msg, m.keys.Refresh):
		return ActionRefresh
	case key.Matches(msg, m.keys.Filter):
		return ActionFilter
	case key.Matches(msg, m.keys.Assign):
		return ActionAssign
	case key.Matches(msg, m.keys.Transition):
		return ActionTransition
	case key.Matches(msg, m.keys.Open):
		return ActionOpen
	case key.Matches(msg, m.keys.Edit):
		return ActionEdit
	case key.Matches(msg, m.keys.Comment):
		return ActionComment
	case key.Matches(msg, m.keys.Branch):
		return ActionBranch
	case key.Matches(msg, m.keys.Extract):
		return ActionExtract
	case key.Matches(msg, m.keys.New):
		return ActionNew
	case key.Matches(msg, m.keys.Workspace):
		return ActionWorkspace
	default:
		return ActionNone
	}
}

// executeAction performs an action. Returns handled=false only for ActionNone.
func (m AppModel) executeAction(action Action) (tea.Model, tea.Cmd, bool) {
	if action == ActionNone {
		return m, nil, false
	}

	// Suppress actions while a command is running.
	if m.commandRunning {
		return m, nil, false
	}

	iss := m.list.SelectedIssue()

	switch action {
	case ActionComment:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Comment(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionExtract:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Extract(m.ctx, m.wsSess, issKey, commands.ExtractOptions{Copy: true})
			}), true
		}

	case ActionTransition:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Transition(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionAssign:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Assign(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionEdit:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Edit(m.ctx, m.wsSess, issKey, nil)
			}), true
		}

	case ActionOpen:
		if iss != nil {
			url := m.ws.BrowseURL(iss.ID)
			if url == "" {
				m.setNotify("No browse URL configured")
				return m, nil, true
			}
			go commands.OpenInBrowser(url) //nolint:errcheck
			m.setNotify("Opened " + iss.ID)
			return m, nil, true
		}

	case ActionBranch:
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Branch(m.ctx, m.wsSess, issKey)
			}), true
		}

	case ActionFilter:
		var others []string
		for name := range m.ws.Filters {
			if name != m.filter {
				others = append(others, name)
			}
		}
		if len(others) == 0 {
			m.setNotify("Only one filter available")
			return m, nil, true
		}
		sort.Strings(others)

		// Current filter first with bullet indicator, then the rest.
		filterNames := []string{"● " + m.filter}
		for _, name := range others {
			filterNames = append(filterNames, "  "+name)
		}
		m.popup.ShowSelect("filter", "Switch Filter", filterNames)
		m.ui.Emit("popup:select", "title", "Switch Filter")
		return m, nil, true

	case ActionRefresh:
		m.loading = "Refreshing..."
		return m, m.fetchFreshData(m.filter), true

	case ActionNew:
		return m, m.runCommand(func() error {
			return commands.Create(m.ctx, m.wsSess, nil)
		}), true

	case ActionWorkspace:
		slugs := make([]string, 0, len(m.runtime.Workspaces))
		for slug := range m.runtime.Workspaces {
			slugs = append(slugs, slug)
		}
		if len(slugs) <= 1 {
			m.setNotify("Only one workspace configured")
			return m, nil, true
		}
		sort.Strings(slugs)

		// Current workspace first with bullet indicator, then the rest.
		wsLabel := func(ws *core.Workspace) string {
			label := ws.Name
			if label == "" {
				label = ws.Slug
			}
			if ws.ServerAlias != "" {
				label += " (" + ws.ServerAlias + ")"
			}
			return label
		}
		names := []string{"● " + wsLabel(m.ws)}
		for _, slug := range slugs {
			if slug == m.ws.Slug {
				continue
			}
			names = append(names, "  "+wsLabel(m.runtime.Workspaces[slug]))
		}
		m.popup.ShowSelect("workspace", "Switch Workspace", names)
		m.ui.Emit("popup:select", "title", "Switch Workspace")
		return m, nil, true
	}

	return m, nil, false
}

// runCommand launches fn in a goroutine via tea.Cmd. The result is sent
// back as commandCompleteMsg, which triggers a data reload.
func (m *AppModel) runCommand(fn func() error) tea.Cmd {
	m.commandRunning = true
	return func() tea.Msg {
		err := fn()
		return commandCompleteMsg{err: err}
	}
}

// View renders the main application view for ihj
func (m AppModel) View() tea.View {
	if !m.ready {
		v := tea.NewView("\n  Loading...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	s := m.styles
	theme := terminal.DefaultTheme()
	outerBorderH := 2
	previewBorderH := 2

	previewContent := m.detail.View()

	// Border color indicates pane focus.
	previewBorderColor := theme.Muted
	if m.view >= ViewDetail {
		previewBorderColor = theme.Accent
	}

	// Breadcrumb bar: pinned at bottom of preview when navigated into children.
	if (m.view >= ViewDetail) && m.detail.CanGoBack() {
		previewContent += "\n" + m.renderBreadcrumbBar()
	}

	previewBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(previewBorderColor).
		Padding(0, 2).
		Width(m.innerW - previewBorderH).
		Height(m.previewContentH).
		MaxHeight(m.previewTotalH).
		Render(previewContent)

	var body string
	if m.view == ViewFullscreen {
		// Fullscreen mode: detail pane fills the screen.
		body = previewBox
	} else {
		searchBarLine := m.list.SearchView()
		divider := lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", m.innerW-previewBorderH))
		list := m.list.View()
		helpBar := m.renderHelpBar(m.innerW)
		body = lipgloss.JoinVertical(lipgloss.Left,
			previewBox,
			searchBarLine,
			divider,
			list,
			divider,
			helpBar,
		)
	}

	cacheAge := m.cacheAgeString()
	titleContent := fmt.Sprintf(" %s │ %s (%s) ",
		m.ws.Name, strings.ToUpper(m.filter), cacheAge)

	outerBorder := lipgloss.RoundedBorder()
	outerStyle := lipgloss.NewStyle().
		Border(outerBorder).
		BorderForeground(theme.Muted).
		Padding(1, 2).
		Width(m.width - outerBorderH).
		BorderTop(false).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true)

	topBorder := m.buildTopBorder(m.width-outerBorderH, outerBorder, titleContent, s)
	inner := outerStyle.Render(body)

	screen := lipgloss.JoinVertical(lipgloss.Left, topBorder, inner)

	if m.popup.Active() {
		screen = m.overlayPopup(screen)
	}

	if m.showHelp {
		screen = m.overlayHelp(screen)
	}

	screen = m.overlayToast(screen)

	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *AppModel) buildTopBorder(width int, border lipgloss.Border, title string, s *terminal.Styles) string {
	theme := terminal.DefaultTheme()
	borderFg := theme.Muted
	horizStyle := lipgloss.NewStyle().Foreground(borderFg)

	titleStyled := s.StatusBarKey.Render(title)
	titleW := lipgloss.Width(titleStyled)

	tl := horizStyle.Render(border.TopLeft)
	tr := horizStyle.Render(border.TopRight)
	horiz := border.Top

	// Center the title in the top border line.
	available := width - titleW
	if available < 4 {
		return horizStyle.Render(strings.Repeat(horiz, max(0, width+2)))
	}

	leftSeg := max(1, available/2-1)
	rightSeg := max(1, available-leftSeg-2)

	return tl +
		horizStyle.Render(strings.Repeat(horiz, leftSeg)) +
		titleStyled +
		horizStyle.Render(strings.Repeat(horiz, rightSeg)) +
		tr
}

func (m *AppModel) cacheAgeString() string {
	if m.fetchedAt.IsZero() {
		return "∞" // Demo mode.
	}
	elapsed := time.Since(m.fetchedAt).Truncate(time.Second)
	if elapsed < time.Minute {
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
}

func (m *AppModel) renderHelpBar(width int) string {
	if m.vimMode {
		return m.renderVimHelpBar(width)
	}

	return m.help.ShortHelpView(m.keys.ShortHelp())
}

// overlaySplice composites a rendered overlay onto the base screen at a given position.
func (m *AppModel) overlaySplice(base, overlay string, top, left int) string {
	if overlay == "" {
		return base
	}

	overlayLines := strings.Split(overlay, "\n")
	baseLines := strings.Split(base, "\n")

	boxW := lipgloss.Width(overlayLines[0])

	// Ensure base has enough lines.
	for len(baseLines) < m.height {
		baseLines = append(baseLines, "")
	}

	// Splice the overlay box into the background line-by-line.
	for i, pLine := range overlayLines {
		y := top + i
		if y >= len(baseLines) {
			break
		}

		bg := baseLines[y]
		bgW := lipgloss.Width(bg)

		if bgW < left {
			bg += strings.Repeat(" ", left-bgW)
			bgW = left
		}

		lStr := ansi.Truncate(bg, left, "")

		var right string
		if bgW > left+boxW {
			right = ansi.TruncateLeft(bg, left+boxW, "")
		}

		baseLines[y] = lStr + pLine + right
	}

	return strings.Join(baseLines, "\n")
}

func (m *AppModel) overlayPopup(base string) string {
	popup := m.popup.View()
	if popup == "" {
		return base
	}
	popupLines := strings.Split(popup, "\n")
	boxH := len(popupLines)
	boxW := lipgloss.Width(popupLines[0])
	top := max(0, (m.height-boxH)/2)
	left := max(0, (m.width-boxW)/2)
	return m.overlaySplice(base, popup, top, left)
}

// overlayHelp renders a WhichKey-style key binding panel at the bottom right.
func (m *AppModel) overlayHelp(base string) string {
	theme := terminal.DefaultTheme()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	descStyle := lipgloss.NewStyle().Foreground(theme.Text)
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Muted)
	hintStyle := lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)

	type group struct {
		name     string
		bindings []key.Binding
	}

	groups := []group{
		{"Navigation", []key.Binding{m.keys.Up, m.keys.Down, m.keys.Home, m.keys.End, m.keys.PageUp, m.keys.PageDn}},
		{"Preview", []key.Binding{m.keys.PreviewUp, m.keys.PreviewDown, m.keys.Focus, m.keys.Tab}},
		{"Actions", m.keys.ActionBindings()},
		{"General", []key.Binding{m.keys.Cancel, m.keys.Quit}},
	}

	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(groupStyle.Render(g.name) + "\n")
		for _, bind := range g.bindings {
			if !bind.Enabled() {
				continue
			}
			h := bind.Help()
			if h.Key == "" {
				continue
			}
			b.WriteString("  " + keyStyle.Render(h.Key) + " " + descStyle.Render(h.Desc) + "\n")
		}
	}

	hint := m.keys.Help.Help().Key + " close"
	b.WriteString("\n" + hintStyle.Render(hint))

	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.Muted).
		Padding(0, 2)

	box := boxStyle.Render(b.String())
	boxLines := strings.Split(box, "\n")
	boxH := len(boxLines)
	boxW := lipgloss.Width(boxLines[0])

	// Position: bottom right, right edge aligned with the detail view border.
	// Outer chrome: 1 border + 2 padding on each side = 3 per side, so 6 total inset.
	top := max(0, m.height-boxH-3)
	left := max(0, m.width-boxW-5)

	return m.overlaySplice(base, box, top, left)
}

// overlayToast composites a floating notification in the bottom right corner.
func (m *AppModel) overlayToast(base string) string {
	if m.notify == "" && m.loading == "" {
		return base
	}

	theme := terminal.DefaultTheme()

	// Determine state and colors.
	msg := m.notify
	icon := "●"
	color := theme.Accent

	if m.loading != "" {
		msg = m.loading
		icon = "⟳"
		color = theme.Warning
	}

	// Render the sleek toast box.
	toastStr := lipgloss.NewStyle().Foreground(color).Render(icon) + " " + msg
	toast := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 1).
		Render(toastStr)

	toastLines := strings.Split(toast, "\n")
	toastH := len(toastLines)
	toastW := lipgloss.Width(toastLines[0])

	// Position: bottom right, pinned just inside the outer border padding.
	top := m.height - toastH - 3
	left := m.width - toastW - 4

	if top < 0 || left < 0 {
		return base
	}

	return m.overlaySplice(base, toast, top, left)
}

func (m *AppModel) recalcLayout() {
	outerBorderV := 2 // top + bottom
	outerPadV := 2    // 1 top + 1 bottom
	outerBorderH := 2 // left + right
	outerPadH := 4    // 2 left + 2 right

	previewBorderV := 2
	previewBorderH := 2
	previewPadH := 4 // 2 left + 2 right padding inside preview border

	searchH := 1
	helpH := 2
	chromeH := searchH + helpH

	m.innerW = max(m.width-outerBorderH-outerPadH, 20)

	m.previewContentW = m.innerW - previewBorderH - previewPadH

	innerH := max(m.height-outerBorderV-outerPadV, 8)

	if m.view == ViewFullscreen {
		// Fullscreen mode: detail pane fills the entire terminal.
		m.previewTotalH = innerH - outerPadV
		m.listH = 0
	} else {
		pct := float64(m.detailPct) / 100.0
		m.previewTotalH = int(math.Ceil(float64(innerH-chromeH) * pct))
		m.listH = innerH - chromeH - m.previewTotalH
		if m.listH < 3 {
			m.listH = 3
			m.previewTotalH = innerH - chromeH - m.listH
		}
	}

	m.previewContentH = max(m.previewTotalH-previewBorderV, 2)

	// Reserve 1 line for breadcrumb bar only when navigated into children.
	detailH := m.previewContentH
	if (m.view >= ViewDetail) && m.detail.CanGoBack() {
		detailH = max(detailH-1, 1)
	}
	m.detail.SetSize(m.previewContentW, detailH)
	m.list.SetSize(m.innerW, m.listH)
	m.help.SetWidth(m.innerW)

	// Mouse zones.
	m.previewTop = 3 // outer border top (1) + outer pad top (1) + preview border top (1)
	m.previewBottom = m.previewTop + m.previewContentH

	m.listTop = m.previewBottom + previewBorderV - 1 + searchH
	m.listBottom = m.listTop + m.listH
}

// View state transitions. These centralise side effects (search focus,
// layout recalculation, event emission) so callers don't manage them.

func (m *AppModel) enterFullscreen() {
	m.view = ViewFullscreen
	m.list.search.Blur()
	m.recalcLayout()
	m.ui.Emit("focus:entered")
}

func (m *AppModel) focusDetail() {
	m.view = ViewDetail
	m.list.search.Blur()
	m.ui.Emit("pane:detail")
}

func (m *AppModel) focusList() {
	m.view = ViewList
	if !m.vimMode {
		m.list.search.Focus()
	}
	m.ui.Emit("pane:list")
}

// exitDetailView exits the current detail view state (fullscreen or pane focus).
// Returns true if an action was taken.
func (m *AppModel) exitDetailView() bool {
	wasFullscreen := m.view == ViewFullscreen
	if m.view < ViewDetail {
		return false
	}
	event := "pane:list"
	if wasFullscreen {
		event = "focus:exited"
	}
	m.view = ViewList
	m.detail.ClearHistory()
	if !m.vimMode {
		m.list.search.Focus()
	}
	m.recalcLayout()
	m.syncDetail()
	m.ui.Emit(event)
	return true
}

// renderBreadcrumbBar renders the pinned breadcrumb line for the detail pane.
// Shows the navigation path with contextual key hints.
func (m *AppModel) renderBreadcrumbBar() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	iss := m.detail.Issue()
	if iss == nil {
		return ""
	}

	if !m.detail.CanGoBack() {
		return ""
	}

	// Show full path: ancestor → ancestor → current  ⌫ ␛
	crumbParts := make([]string, 0, 4)
	bc := m.detail.Breadcrumb()
	ids := strings.Split(bc, " → ")
	for i, id := range ids {
		if i == len(ids)-1 {
			crumbParts = append(crumbParts, lipgloss.NewStyle().Bold(true).Render(id))
		} else {
			crumbParts = append(crumbParts, dimStyle.Render(id))
		}
	}
	sep := dimStyle.Render(" → ")
	breadcrumb := strings.Join(crumbParts, sep)
	hint := dimStyle.Render("  ⌫ ␛")
	return breadcrumb + hint
}

func (m *AppModel) setNotify(msg string) {
	m.notify = msg
	m.notifyAt = time.Now()
	m.ui.Emit("notify", "message", msg)
}

// resolveWorkspaceSlug finds the workspace slug for a display label.
// Labels may include a server alias suffix like "My Team (prod-jira)".
func (m *AppModel) resolveWorkspaceSlug(label string) string {
	for slug, ws := range m.runtime.Workspaces {
		name := ws.Name
		if name == "" {
			name = slug
		}
		candidate := name
		if ws.ServerAlias != "" {
			candidate += " (" + ws.ServerAlias + ")"
		}
		if candidate == label {
			return slug
		}
	}
	return label // Fallback: treat label as slug.
}

// switchWorkspace creates a new session via the factory and fetches data.
func (m *AppModel) switchWorkspace(slug string) tea.Cmd {
	m.loading = "Switching to " + slug + "..."
	factory := m.factory
	ctx := m.ctx
	return func() tea.Msg {
		wsSess, err := factory(slug)
		if err != nil {
			return workspaceSwitchedMsg{slug: slug, err: err}
		}
		filter := commands.ResolveFilter("")
		items, searchErr := wsSess.Provider.Search(ctx, filter, true)
		if searchErr != nil {
			return workspaceSwitchedMsg{slug: slug, err: searchErr}
		}
		return workspaceSwitchedMsg{
			slug:      slug,
			wsSess:    wsSess,
			items:     items,
			fetchedAt: time.Now(),
		}
	}
}

// switchFilter loads data for the new filter. Uses stale cache immediately
// if available, then always fetches fresh data in the background.
func (m *AppModel) switchFilter(filter string) tea.Cmd {
	m.loading = "Loading " + strings.ToUpper(filter) + "..."
	return m.fetchFreshData(filter)
}

// fetchFreshData fetches fresh data from the API for a given filter.
func (m *AppModel) fetchFreshData(filter string) tea.Cmd {
	return m.fetchData(filter, false)
}

// fetchStartupData fetches fresh data on startup. Errors are treated as fatal
// (e.g. auth failures) and cause the TUI to exit.
func (m *AppModel) fetchStartupData(filter string) tea.Cmd {
	provider := m.wsSess.Provider
	return func() tea.Msg {
		items, err := provider.Search(m.ctx, filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err, startup: true}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
			silent:    true,
		}
	}
}

// fetchFreshDataSilent fetches fresh data without showing a notification.
// Used for background reloads after commands complete.
func (m *AppModel) fetchFreshDataSilent(filter string) tea.Cmd {
	return m.fetchData(filter, true)
}

func (m *AppModel) fetchData(filter string, silent bool) tea.Cmd {
	provider := m.wsSess.Provider
	return func() tea.Msg {
		items, err := provider.Search(m.ctx, filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err, silent: silent}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
			silent:    silent,
		}
	}
}
