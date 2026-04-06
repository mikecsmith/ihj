// Package tui implements the Bubble Tea terminal user interface for ihj.
//
// The main model is AppModel, which composes a list pane, detail pane,
// and popup overlay. BubbleTeaUI implements the commands.UI interface,
// bridging between the business logic layer and the interactive TUI.
//
// File organisation:
//   - app.go            — model definition, constructor, init
//   - app_actions.go    — action execution (comment, edit, transition, etc.)
//   - app_fetch.go      — data fetching, filter/workspace switching
//   - app_helpers.go    — small utility methods (setNotify, resolveWorkspaceSlug)
//   - app_keys.go       — keystroke routing, navigation, popup results
//   - app_layout.go     — layout calculation, view state transitions
//   - app_overlays.go   — floating UI layers (toast, popup, help panel)
//   - app_update.go     — Update() dispatcher and domain handlers
//   - app_view.go       — View() composition, chrome (borders, footer, breadcrumb)
//   - detail.go         — detail pane model, navigation, scroll
//   - detail_render.go  — detail pane rendering (metadata, children, comments)
//   - list.go           — list pane model, data, filtering
//   - list_render.go    — list pane rendering (table, cards, tree glyphs)
package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"

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
	detailTop    int // Y offset of detail area start.
	detailBottom int // Y offset of detail area end.
	listTop      int // Y offset of list area start.
	listBottom   int // Y offset of list area end.

	// Issue registry for lookups (shared with detail model).
	registry map[string]*core.WorkItem

	// Cached layout dimensions (computed in recalcLayout, used in View).
	innerW         int
	detailContentW int
	detailContentH int
	detailTotalH   int
	listH          int

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
	help        help.Model
	showHelp    bool // Toggle full help view via '?'.
	showHelpBar bool // Config-driven: show/hide the help bar.

	// View state: which pane is active and how it's arranged.
	view ViewState
	// Configurable detail pane height as a percentage (20-80, default 55).
	detailPct int

	// fatalErr is set when an unrecoverable error occurs (e.g. auth failure
	// on background refresh). The TUI quits and the caller reads the error.
	fatalErr error
}

// NewAppModel creates the TUI application model with the given data.
func NewAppModel(ctx context.Context, rt *commands.Runtime, wsSess *commands.WorkspaceSession, factory commands.WorkspaceSessionFactory, ws *core.Workspace, filter string, items []*core.WorkItem, fetchedAt time.Time, ui *BubbleTeaUI, vimMode bool, shortcuts map[string]string, detailPct int, showHelpBar bool) AppModel {
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
	fieldDefs := ws.AllFieldDefs()

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
		list:        NewListModel(registry, styles, ws.StatusOrderMap, ws.TypeOrderMap, fieldDefs),
		detail:      NewDetailModel(styles, registry, ws, keys),
		popup:       NewPopupModel(styles, keys),
		styles:      styles,
		keys:        keys,
		registry:    registry,
		fetchedAt:   fetchedAt,
		caps:        caps,
		ui:          ui,
		vimMode:     vimMode,
		help:        h,
		showHelpBar: showHelpBar,
		detailPct:   detailPct,
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
			cmds = append(cmds, m.fetchData(m.filter, fetchOpts{silent: true, startup: true}))
		}
	}
	return tea.Batch(cmds...)
}
