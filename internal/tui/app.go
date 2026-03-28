// Package tui implements the Bubble Tea terminal user interface for ihj.
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
	"github.com/mikecsmith/ihj/internal/document"
)

type AppModel struct {
	session *commands.Session
	ws     *core.Workspace
	filter string

	list   ListModel
	detail DetailModel
	popup  PopupModel
	styles *Styles
	keys   KeyMap

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

	// Popup context — stores data needed to complete an action after popup closes.
	popupTransitions []popupTransition // cached transitions for the popup select.

	// Extract context — tracks scope selection for two-step extract flow.
	extractIssueKey string   // issue being extracted
	extractScopes   []string // scope options shown in popup
	extractScopeIdx int      // selected scope index from first popup

	// Upsert state machine — edit/create flow split across message phases.
	upsertPhase upsertPhase
	upsertCtx   *upsertContext

	// Provider capabilities — cached at init for gating actions.
	caps core.Capabilities
}

func NewAppModel(session *commands.Session, ws *core.Workspace, filter string, items []*core.WorkItem, fetchedAt time.Time) AppModel {
	theme := DefaultTheme()
	styles := NewStyles(theme, ws)
	keys := DefaultKeyMap()

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	var caps core.Capabilities
	if session.Provider != nil {
		caps = session.Provider.Capabilities()
	}

	// Disable keybindings for unsupported capabilities.
	if !caps.HasTransitions {
		keys.Transition.SetEnabled(false)
	}

	return AppModel{
		session: session, ws: ws, filter: filter,
		list:      NewListModel(registry, styles, ws.StatusWeights, ws.TypeOrderMap),
		detail:    NewDetailModel(styles, registry, ws.Name, keys),
		popup:     NewPopupModel(styles, keys),
		styles:    styles,
		keys:      keys,
		registry:  registry,
		fetchedAt: fetchedAt,
		caps:      caps,
	}
}

func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.list.Init(), m.detail.Init(), m.tickCmd()}
	// Pre-fetch the current user for comments/assign/create.
	if m.session.Provider != nil {
		provider := m.session.Provider
		cmds = append(cmds, func() tea.Msg {
			user, err := provider.CurrentUser(context.TODO())
			if err != nil {
				return userFetchedMsg{err: err}
			}
			return userFetchedMsg{displayName: user.DisplayName}
		})
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

	case transitionsLoadedMsg:
		m.loading = ""
		// Transitions fetched async — now show the popup.
		if msg.err != nil {
			m.setNotify("Error: " + msg.err.Error())
			return m, nil
		}
		names := make([]string, len(msg.transitions))
		m.popupTransitions = make([]popupTransition, len(msg.transitions))
		for i, t := range msg.transitions {
			names[i] = t.Name
			m.popupTransitions[i] = popupTransition{ID: t.ID, Name: t.Name}
		}
		m.popup.ShowSelect("transition", fmt.Sprintf("Transition: %s", msg.issueKey), names)
		return m, nil

	case transitionDoneMsg:
		m.loading = ""
		if msg.err != nil {
			m.setNotify("Error: " + msg.err.Error())
		} else {
			// Update the issue's status in the local registry.
			if iss, ok := m.registry[msg.issueKey]; ok {
				iss.Status = msg.newStatus
			}
			m.detail.rebuildContent()
			m.setNotify(fmt.Sprintf("%s → %s", msg.issueKey, msg.newStatus))
		}
		return m, nil

	case commentDoneMsg:
		m.loading = ""
		if msg.err != nil {
			m.setNotify("Error: " + msg.err.Error())
		} else {
			// Append the new comment to the IssueView so it's visible immediately.
			if iss, ok := m.registry[msg.issueKey]; ok {
				iss.Comments = append(iss.Comments, msg.comment)
			}
			m.detail.rebuildContent()
			m.setNotify(fmt.Sprintf("Comment added to %s", msg.issueKey))
		}
		return m, nil

	case assignDoneMsg:
		m.loading = ""
		if msg.err != nil {
			m.setNotify("Error: " + msg.err.Error())
		} else {
			// Update the assignee in the local registry.
			if iss, ok := m.registry[msg.issueKey]; ok {
				if iss.Fields == nil {
					iss.Fields = make(map[string]any)
				}
				iss.Fields["assignee"] = msg.assignee
			}
			m.detail.rebuildContent()
			m.setNotify(fmt.Sprintf("Assigned %s to %s", msg.issueKey, msg.assignee))
		}
		return m, nil

	case commandDoneMsg:
		if msg.notify != "" {
			m.setNotify(msg.notify)
		}
		if msg.err != nil && !commands.IsCancelled(msg.err) {
			m.setNotify("Error: " + msg.err.Error())
		}
		return m, nil

	case upsertPreparedMsg:
		if msg.err != nil {
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
			m.setNotify("Error: " + msg.err.Error())
			return m, nil
		}
		m.upsertCtx = msg.ctx
		return m.launchEditor(msg.ctx, msg.ctx.initialDoc, msg.ctx.cursorLine, msg.ctx.searchPat)

	case upsertEditorDoneMsg:
		// Clean up temp file.
		if msg.ctx.tmpPath != "" {
			_ = os.Remove(msg.ctx.tmpPath)
			msg.ctx.tmpPath = ""
		}
		if msg.err != nil {
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
			m.setNotify("Editor error: " + msg.err.Error())
			return m, nil
		}
		// Check for no-change cancellation.
		if strings.TrimSpace(msg.ctx.edited) == strings.TrimSpace(msg.ctx.initialDoc) {
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
			m.setNotify("No changes — cancelled")
			return m, nil
		}
		m.upsertCtx = msg.ctx
		return m, m.submitMutation()

	case upsertSubmitResultMsg:
		if msg.errMsg != "" {
			// Recoverable error — show recovery popup.
			m.upsertPhase = upsertAwaitingRecovery
			m.upsertCtx = msg.ctx
			m.setNotify(msg.errMsg)
			m.popup.ShowSelect("upsert-recovery", "What now?", []string{
				"Re-edit",
				"Copy to clipboard and abort",
				"Abort",
			})
			return m, nil
		}
		if msg.err != nil {
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
			m.setNotify("Error: " + msg.err.Error())
			return m, nil
		}
		// Success — run post-mutation (sprint/transition), then re-fetch from API.
		m.setNotify(msg.notify)
		m.loading = "Syncing..."
		ctx := msg.ctx
		return m, m.runPostMutateAndRefetch(ctx, msg.issueKey)

	case postUpsertCompleteMsg:
		m.upsertPhase = upsertIdle
		m.upsertCtx = nil
		m.loading = ""
		// Show post-upsert notifications (sprint assignment, transition results).
		for _, n := range msg.notifications {
			m.setNotify(n)
		}
		if msg.fetchErr != nil {
			// Fetch failed — don't update registry with stale data.
			m.setNotify("Sync warning: " + msg.fetchErr.Error())
			return m, nil
		}
		if msg.item != nil {
			m.mergeIssueIntoRegistry(msg.item, msg.issueKey, msg.isCreate, msg.parentKey)
		}
		return m, nil

	case userFetchedMsg:
		if msg.err == nil && msg.displayName != "" {
			m.cachedUserName = msg.displayName
		}
		return m, nil

	case dataReloadedMsg:
		m.loading = ""
		if msg.err != nil {
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
		m.setNotify(fmt.Sprintf("Loaded %d issues (%s)", len(msg.items), strings.ToUpper(msg.filter)))
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
	if result.Canceled {
		// Reset upsert state if an upsert popup was cancelled.
		if result.ID == "upsert-type" || result.ID == "upsert-recovery" {
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
		}
		m.setNotify("Cancelled")
		return m, nil
	}

	iss := m.list.SelectedIssue()

	switch result.ID {
	case "transition":
		if result.Index >= 0 && result.Index < len(m.popupTransitions) {
			t := m.popupTransitions[result.Index]
			k := iss.ID
			provider := m.session.Provider
			m.loading = "Transitioning..."
			return m, func() tea.Msg {
				if err := provider.Update(context.TODO(), k, &core.Changes{Status: &t.Name}); err != nil {
					return transitionDoneMsg{issueKey: k, err: err}
				}
				return transitionDoneMsg{issueKey: k, newStatus: t.Name}
			}
		}

	case "comment":
		if result.Text != "" && iss != nil {
			return m, m.postCommentCmd(iss.ID, result.Text)
		}

	case "filter":
		if result.Value != "" {
			if result.Value == m.filter {
				m.setNotify("Already on filter: " + result.Value)
			} else {
				return m, m.switchFilter(result.Value)
			}
		}

	case "upsert-type":
		if result.Value != "" {
			ctx := m.upsertCtx
			m.upsertPhase = upsertAwaitingEditor
			return m, m.startCreatePrepare(ctx.workspace, result.Value, ctx.overrides)
		}
		m.upsertPhase = upsertIdle
		m.upsertCtx = nil
		return m, nil

	case "upsert-recovery":
		ctx := m.upsertCtx
		switch result.Index {
		case 0: // Re-edit
			return m.launchEditor(ctx, ctx.edited, 0, "")
		case 1: // Copy to clipboard and abort
			if clipErr := m.session.UI.CopyToClipboard(ctx.edited); clipErr != nil {
				m.setNotify("Warning: Could not copy to clipboard")
			} else {
				m.setNotify("Buffer copied to clipboard")
			}
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
		default: // Abort
			m.upsertPhase = upsertIdle
			m.upsertCtx = nil
			m.setNotify("Cancelled")
		}
		return m, nil

	case "extract-scope":
		if result.Index >= 0 && result.Index < len(m.extractScopes) {
			m.extractScopeIdx = result.Index
			m.popup.ShowInput("extract-prompt", "LLM Prompt: "+m.extractIssueKey, "Describe what you want the LLM to do...")
			return m, nil
		}

	case "extract-prompt":
		if result.Text != "" {
			prompt := result.Text
			issueKey := m.extractIssueKey
			scopeName := m.extractScopes[m.extractScopeIdx]
			registry := m.registry
			board := m.ws
			return m, m.async(func() (string, error) {
				keys := commands.CollectExtractKeys(issueKey, scopeName, registry)
				xml := commands.BuildExtractXML(prompt, keys, registry, board)
				if err := m.session.UI.CopyToClipboard(xml); err != nil {
					return "", err
				}
				return fmt.Sprintf("LLM context copied (%d issues)", len(keys)), nil
			})
		}
	}

	return m, nil
}

func (m AppModel) handleAction(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	iss := m.list.SelectedIssue()

	switch {
	case key.Matches(msg, m.keys.Comment):
		if iss != nil {
			m.popup.ShowInput("comment", "Comment on "+iss.ID, "Write your comment...")
			return m, nil, true
		}

	case key.Matches(msg, m.keys.Extract):
		if iss != nil {
			scopes := commands.ScopeOptions(iss.ParentID != "")
			m.extractIssueKey = iss.ID
			m.extractScopes = scopes
			m.popup.ShowSelect("extract-scope", "Extract Scope: "+iss.ID, scopes)
			return m, nil, true
		}

	case key.Matches(msg, m.keys.Transition):
		if iss != nil {
			// Show workspace statuses as available transitions.
			statuses := m.ws.Statuses
			pts := make([]popupTransition, len(statuses))
			for i, s := range statuses {
				pts[i] = popupTransition{ID: s, Name: s}
			}
			m.popupTransitions = pts
			names := make([]string, len(pts))
			for i, t := range pts {
				names[i] = t.Name
			}
			m.popup.ShowSelect("transition", "Transition: "+iss.ID, names)
			return m, nil, true
		}

	case key.Matches(msg, m.keys.Assign):
		if iss != nil {
			k := iss.ID
			provider := m.session.Provider
			userName := m.cachedUserName
			if userName == "" {
				m.setNotify("User not loaded yet — try again")
				return m, nil, true
			}
			m.loading = "Assigning..."
			return m, func() tea.Msg {
				if err := provider.Assign(context.TODO(), k); err != nil {
					return assignDoneMsg{issueKey: k, err: err}
				}
				return assignDoneMsg{issueKey: k, assignee: userName}
			}, true
		}

	case key.Matches(msg, m.keys.Edit):
		if iss != nil {
			m.upsertPhase = upsertAwaitingEditor
			return m, m.startEditPrepare(m.ws.Slug, iss.ID, nil), true
		}

	case key.Matches(msg, m.keys.Open):
		if iss != nil {
			url := m.ws.BaseURL + "/browse/" + iss.ID
			go commands.OpenInBrowser(url) //nolint:errcheck
			m.setNotify("Opened " + iss.ID)
			return m, nil, true
		}

	case key.Matches(msg, m.keys.Branch):
		if iss != nil {
			issKey := iss.ID
			summary := iss.Summary
			return m, func() tea.Msg {
				branchCmd := commands.GenerateBranchCmd(issKey, summary)
				if err := m.session.UI.CopyToClipboard(branchCmd); err != nil {
					return commandDoneMsg{notify: "Branch: " + branchCmd, err: nil}
				}
				return commandDoneMsg{notify: "Branch copied: " + branchCmd}
			}, true
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
		// Sort the types strictly by the Order integer defined in your YAML
		var types []core.TypeConfig
		types = append(types, m.ws.Types...)
		sort.Slice(types, func(i, j int) bool {
			return types[i].Order < types[j].Order
		})

		var typeNames []string
		for _, t := range types {
			typeNames = append(typeNames, t.Name)
		}

		if len(typeNames) == 0 {
			m.setNotify("No issue types configured")
			return m, nil, true
		}
		m.upsertPhase = upsertAwaitingTypeSelect
		m.upsertCtx = &upsertContext{workspace: m.ws.Slug}
		m.popup.ShowSelect("upsert-type", "Create New Issue", typeNames)
		return m, nil, true
	}

	return m, nil, false
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
	theme := DefaultTheme()
	outerBorderH := 2
	previewBorderH := 2

	// ── Render sections ────────────────────────────────────────
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
	divider := lipgloss.NewStyle().Foreground(DefaultTheme().Muted).Render(strings.Repeat("─", m.innerW-previewBorderH))

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

	// ── Outer border with title ────────────────────────────────
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

	// ── Compositing ────────────────────────────────────────────
	screen := lipgloss.JoinVertical(lipgloss.Left, topBorder, inner)

	// 1. Paint the toast (if active)
	screen = m.overlayToast(screen)

	// 2. Paint the popup over top of everything (if active)
	if m.popup.Active() {
		screen = m.overlayPopup(screen)
	}

	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *AppModel) buildTopBorder(width int, border lipgloss.Border, title string, s *Styles) string {
	theme := DefaultTheme()
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

		// 1. Pad the background if it's somehow shorter than the popup's starting X
		if bgW < padLeft {
			bg += strings.Repeat(" ", padLeft-bgW)
			bgW = padLeft
		}

		// 2. Keep the background up to the left edge of the popup
		left := ansi.Truncate(bg, padLeft, "")

		// 3. Keep the background from the right edge of the popup to the end
		var right string
		if bgW > padLeft+boxW {
			right = ansi.TruncateLeft(bg, padLeft+boxW, "")
		}

		// 4. Sandwich the popup safely in the middle!
		baseLines[y] = left + pLine + right
	}

	return strings.Join(baseLines, "\n")
}

// overlayToast composites a floating notification in the bottom right corner.
func (m *AppModel) overlayToast(base string) string {
	if m.notify == "" && m.loading == "" {
		return base
	}

	theme := DefaultTheme()

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

// --- Async ---

func (m *AppModel) async(fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		msg, err := fn()
		return commandDoneMsg{err: err, notify: msg}
	}
}

func (m *AppModel) setNotify(msg string) {
	m.notify = msg
	m.notifyAt = time.Now()
}

// postCommentCmd creates a tea.Cmd that parses markdown, posts the comment, and
// returns a commentDoneMsg. Used by both popup and inline comment paths.
func (m *AppModel) postCommentCmd(issueKey, text string) tea.Cmd {
	provider := m.session.Provider
	author := m.currentUserName()
	m.loading = "Posting comment..."
	return func() tea.Msg {
		if err := provider.Comment(context.TODO(), issueKey, text); err != nil {
			return commentDoneMsg{issueKey: issueKey, err: err}
		}
		// Parse the markdown for local display.
		ast, _ := document.ParseMarkdownString(text)
		return commentDoneMsg{
			issueKey: issueKey,
			comment: core.Comment{
				Author:  author,
				Created: time.Now().Format("2 Jan 2006 15:04"),
				Body:    ast,
			},
		}
	}
}

// mergeIssueIntoRegistry updates or adds an issue in the registry, then
// rebuilds the list and detail views. Used by postUpsertCompleteMsg.
func (m *AppModel) mergeIssueIntoRegistry(item *core.WorkItem, issueKey string, isCreate bool, parentKey string) {
	if existing, ok := m.registry[issueKey]; ok {
		// Edit — merge API response into existing entry,
		// preserving children links already in memory.
		existing.Summary = item.Summary
		existing.Type = item.Type
		existing.Status = item.Status
		existing.Fields = item.Fields
		if item.Description != nil {
			existing.Description = item.Description
		}
		if item.ParentID != "" {
			existing.ParentID = item.ParentID
		}
		if len(item.Comments) > 0 {
			existing.Comments = item.Comments
		}
	} else {
		// Create — add new entry to registry.
		if isCreate && parentKey != "" {
			item.ParentID = parentKey
		}
		m.registry[issueKey] = item
		core.LinkChildren(m.registry)
	}
	m.list.Rebuild(m.registry)
	m.detail.rebuildContent()
	m.syncDetail()
}

// currentUserName returns the cached user's display name, or "You" if not cached.
func (m *AppModel) currentUserName() string {
	if m.cachedUserName != "" {
		return m.cachedUserName
	}
	return "You"
}

// switchFilter loads data for the new filter. Uses stale cache immediately
// if available, then always fetches fresh data in the background.
func (m *AppModel) switchFilter(filter string) tea.Cmd {
	// Always fetch from API for now.
	// TODO: add caching at the provider level via middleware.
	m.loading = "Loading " + strings.ToUpper(filter) + "..."
	return m.fetchFreshData(filter)
}

// fetchFreshData fetches fresh data from the API for a given filter.
func (m *AppModel) fetchFreshData(filter string) tea.Cmd {
	provider := m.session.Provider
	return func() tea.Msg {
		items, err := provider.Search(context.TODO(), filter, true)
		if err != nil {
			return dataReloadedMsg{filter: filter, err: err}
		}
		return dataReloadedMsg{
			filter:    filter,
			items:     items,
			fetchedAt: time.Now(),
		}
	}
}
