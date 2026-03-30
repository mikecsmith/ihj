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

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// AppModel is the top-level Bubble Tea model for the ihj TUI.
type AppModel struct {
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

	// fatalErr is set when an unrecoverable error occurs (e.g. auth failure
	// on background refresh). The TUI quits and the caller reads the error.
	fatalErr error
}

// NewAppModel creates the TUI application model with the given data.
func NewAppModel(rt *commands.Runtime, wsSess *commands.WorkspaceSession, factory commands.WorkspaceSessionFactory, ws *core.Workspace, filter string, items []*core.WorkItem, fetchedAt time.Time, ui *BubbleTeaUI) AppModel {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, ws, rt.Theme)
	keys := terminal.DefaultKeyMap()

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

	return AppModel{
		runtime: rt, wsSess: wsSess, factory: factory,
		ws: ws, filter: filter,
		list:      NewListModel(registry, styles, ws.StatusWeights, ws.TypeOrderMap),
		detail:    NewDetailModel(styles, registry, ws.Name, keys),
		popup:     NewPopupModel(styles, keys),
		styles:    styles,
		keys:      keys,
		registry:  registry,
		fetchedAt: fetchedAt,
		caps:      caps,
		ui:        ui,
	}
}

// Err returns any fatal error that caused the TUI to exit.
func (m AppModel) Err() error { return m.fatalErr }

func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.list.Init(), m.detail.Init(), m.tickCmd()}
	if m.wsSess.Provider != nil {
		// Pre-fetch the current user for comments/assign/create.
		provider := m.wsSess.Provider
		cmds = append(cmds, func() tea.Msg {
			user, err := provider.CurrentUser(context.TODO())
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
		}
		return m, nil

	case tea.KeyPressMsg:
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
		return m, nil

	case bridgeConfirmMsg:
		m.popup.ShowSelect("bridge-confirm", msg.prompt, []string{"Yes", "No"})
		return m, nil

	case bridgeInputMsg:
		m.popup.ShowInput("bridge-input", msg.prompt, msg.initial)
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
		m.detail = NewDetailModel(m.styles, m.registry, m.ws.Name, m.keys)
		m.detail.SetSize(m.previewContentW, m.previewContentH)
		m.syncDetail()
		if !msg.silent {
			m.setNotify(fmt.Sprintf("Loaded %d issues (%s)", len(msg.items), strings.ToUpper(msg.filter)))
		}
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
			m.detail.ScrollUp(3)
		} else if y >= m.listTop && y < m.listBottom {
			if m.list.cursor > 0 {
				m.list.cursor--
				m.syncDetail()
			}
		}
	case tea.MouseWheelDown:
		if y >= m.previewTop && y < m.previewBottom {
			m.detail.ScrollDown(3)
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
	// Global keys.
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Cancel) {
		// If navigated into a child, pop back first.
		if m.detail.CanGoBack() {
			m.detail.GoBack()
			return m, nil
		}
		if m.list.search.Value() != "" {
			m.list.search.SetValue("")
			m.list.applyFilter()
			return m, nil
		}
		return m, tea.Quit
	}

	// Alt-key actions (always available, don't interfere with search).
	if model, cmd, handled := m.handleAction(msg); handled {
		return model, cmd
	}

	// Navigation keys.
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

	// Preview scroll.
	case key.Matches(msg, m.keys.PreviewUp):
		m.detail.ScrollUp(3)
		return m, nil
	case key.Matches(msg, m.keys.PreviewDown):
		m.detail.ScrollDown(3)
		return m, nil

	// Navigate into child issues from preview.
	case key.Matches(msg, m.keys.EnterChild):
		if iss := m.detail.Issue(); iss != nil && len(iss.Children) > 0 {
			// Navigate to first child.
			m.detail.NavigateToChild(0)
		}
		return m, nil
	}

	// Number keys 1-9 navigate to nth child issue in preview.
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0]-'0') - 1
		if m.detail.NavigateToChild(idx) {
			return m, nil
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
			if result.Value == m.filter {
				m.setNotify("Already on filter: " + result.Value)
			} else {
				return m, m.switchFilter(result.Value)
			}
		}
	}

	return m, nil
}

func (m AppModel) handleAction(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// Suppress action keys while a command is running.
	if m.commandRunning {
		return m, nil, false
	}

	iss := m.list.SelectedIssue()

	switch {
	case key.Matches(msg, m.keys.Comment):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Comment(m.wsSess, issKey)
			}), true
		}

	case key.Matches(msg, m.keys.Extract):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Extract(m.wsSess, issKey, commands.ExtractOptions{Copy: true})
			}), true
		}

	case key.Matches(msg, m.keys.Transition):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Transition(m.wsSess, issKey)
			}), true
		}

	case key.Matches(msg, m.keys.Assign):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Assign(m.wsSess, issKey)
			}), true
		}

	case key.Matches(msg, m.keys.Edit):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Edit(m.wsSess, issKey, nil)
			}), true
		}

	case key.Matches(msg, m.keys.Open):
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

	case key.Matches(msg, m.keys.Branch):
		if iss != nil {
			issKey := iss.ID
			return m, m.runCommand(func() error {
				return commands.Branch(m.wsSess, issKey)
			}), true
		}

	case key.Matches(msg, m.keys.Filter):
		var custom []string
		for name := range m.ws.Filters {
			if name != "default" {
				custom = append(custom, name)
			}
		}
		// Alphabetize the map output
		sort.Strings(custom)

		filterNames := []string{"default"}
		filterNames = append(filterNames, custom...)

		if len(filterNames) <= 1 {
			m.setNotify("Only one filter available")
			return m, nil, true
		}
		m.popup.ShowSelect("filter", "Switch Filter", filterNames)
		return m, nil, true

	case key.Matches(msg, m.keys.Refresh):
		m.loading = "Refreshing..."
		return m, m.fetchFreshData(m.filter), true

	case key.Matches(msg, m.keys.New):
		return m, m.runCommand(func() error {
			return commands.Create(m.wsSess, nil)
		}), true
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

	previewBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 2).
		Width(m.innerW - previewBorderH).
		Height(m.previewContentH).
		MaxHeight(m.previewTotalH).
		Render(previewContent)

	searchBarLine := m.list.SearchView()
	divider := lipgloss.NewStyle().Foreground(terminal.DefaultTheme().Muted).Render(strings.Repeat("─", m.innerW-previewBorderH))

	list := m.list.View()
	helpBar := m.renderHelpBar(m.innerW)
	// Stack the core layout
	body := lipgloss.JoinVertical(lipgloss.Left,
		previewBox,
		searchBarLine,
		divider,
		list,
		divider,
		helpBar,
	)

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
	screen = m.overlayToast(screen)

	if m.popup.Active() {
		screen = m.overlayPopup(screen)
	}

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
	s := m.styles
	// Dynamically build the help bar from our KeyMap definitions
	keys := []key.Binding{
		m.keys.Refresh, m.keys.Filter, m.keys.Assign, m.keys.Transition,
		m.keys.Open, m.keys.Edit, m.keys.Comment, m.keys.Branch,
		m.keys.Extract, m.keys.New,
	}

	var parts []string
	for _, k := range keys {
		if k.Enabled() {
			parts = append(parts, s.ActionKey.Render(k.Help().Key)+" "+s.ActionDesc.Render(k.Help().Desc))
		}
	}
	bar := strings.Join(parts, s.ActionDesc.Render(" | "))

	bar = lipgloss.NewStyle().MaxWidth(width).Render(bar)

	return bar
}

func (m *AppModel) overlayPopup(base string) string {
	popup := m.popup.View()
	if popup == "" {
		return base
	}

	popupLines := strings.Split(popup, "\n")
	baseLines := strings.Split(base, "\n")

	boxH := len(popupLines)
	boxW := lipgloss.Width(popupLines[0])
	padTop := max(0, (m.height-boxH)/2)
	padLeft := max(0, (m.width-boxW)/2)

	// Ensure base has enough lines.
	for len(baseLines) < m.height {
		baseLines = append(baseLines, "")
	}

	// Splice the popup box into the background line-by-line
	for i, pLine := range popupLines {
		y := padTop + i
		if y >= len(baseLines) {
			break
		}

		bg := baseLines[y]
		bgW := lipgloss.Width(bg)

		if bgW < padLeft {
			bg += strings.Repeat(" ", padLeft-bgW)
			bgW = padLeft
		}

		left := ansi.Truncate(bg, padLeft, "")

		var right string
		if bgW > padLeft+boxW {
			right = ansi.TruncateLeft(bg, padLeft+boxW, "")
		}

		baseLines[y] = left + pLine + right
	}

	return strings.Join(baseLines, "\n")
}

// overlayToast composites a floating notification in the bottom right corner.
func (m *AppModel) overlayToast(base string) string {
	if m.notify == "" && m.loading == "" {
		return base
	}

	theme := terminal.DefaultTheme()

	// Determine state and colors
	msg := m.notify
	icon := "●"
	color := theme.Accent

	if m.loading != "" {
		msg = m.loading
		icon = "⟳"
		color = theme.Warning
	}

	// Render the sleek toast box
	toastStr := lipgloss.NewStyle().Foreground(color).Render(icon) + " " + msg
	toastBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Padding(0, 1).
		Render(toastStr)

	toastLines := strings.Split(toastBox, "\n")
	baseLines := strings.Split(base, "\n")

	toastH := len(toastLines)
	toastW := lipgloss.Width(toastLines[0])

	// Position: Bottom right, pinned just inside the outer border padding
	padTop := m.height - toastH - 3
	padLeft := m.width - toastW - 4

	// If the terminal is microscopic, just abort the toast
	if padTop < 0 || padLeft < 0 {
		return base
	}

	// Splice the toast into the background line-by-line
	for i, tLine := range toastLines {
		y := padTop + i
		if y >= len(baseLines) {
			break
		}

		bg := baseLines[y]
		bgW := lipgloss.Width(bg)

		if bgW < padLeft {
			bg += strings.Repeat(" ", padLeft-bgW)
			bgW = padLeft
		}

		left := ansi.Truncate(bg, padLeft, "")
		var right string
		if bgW > padLeft+toastW {
			right = ansi.TruncateLeft(bg, padLeft+toastW, "")
		}

		baseLines[y] = left + tLine + right
	}

	return strings.Join(baseLines, "\n")
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

	m.previewTotalH = int(math.Ceil(float64(innerH-chromeH) * 0.55))
	m.listH = innerH - chromeH - m.previewTotalH
	if m.listH < 3 {
		m.listH = 3
		m.previewTotalH = innerH - chromeH - m.listH
	}

	m.previewContentH = max(m.previewTotalH-previewBorderV, 2)

	m.detail.SetSize(m.previewContentW, m.previewContentH)
	m.list.SetSize(m.innerW, m.listH)

	// Mouse zones.
	m.previewTop = 3 // outer border top (1) + outer pad top (1) + preview border top (1)
	m.previewBottom = m.previewTop + m.previewContentH

	m.listTop = m.previewBottom + previewBorderV - 1 + searchH
	m.listBottom = m.listTop + m.listH
}

func (m *AppModel) setNotify(msg string) {
	m.notify = msg
	m.notifyAt = time.Now()
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
		items, err := provider.Search(context.TODO(), filter, true)
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
		items, err := provider.Search(context.TODO(), filter, true)
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
