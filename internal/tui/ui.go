// Package tui implements the Bubble Tea terminal user interface for ihj.
//
// BubbleTeaUI implements commands.UI using a channel-based bridge pattern:
// interactive methods send messages to the Bubble Tea program and block on
// channels until the Update loop resolves them via popups or tea.ExecProcess.
package tui

import (
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/terminal"
)

// Compile-time check that BubbleTeaUI satisfies commands.UI.
var _ commands.UI = (*BubbleTeaUI)(nil)

// EventKind identifies a TUI state transition. Typed constants provide
// compile-time safety — raw strings won't pass where EventKind is expected.
type EventKind string

const (
	EventReady          EventKind = "ready"
	EventViewList       EventKind = "view:list"
	EventViewDetail     EventKind = "view:detail"
	EventViewFullscreen EventKind = "view:fullscreen"
	EventNavigated      EventKind = "navigated"
	EventBack           EventKind = "back"
	EventNotify         EventKind = "notify"
	EventPopupSelect    EventKind = "popup:select"
	EventPopupConfirm   EventKind = "popup:confirm"
	EventPopupInput     EventKind = "popup:input"
)

// UIEvent represents a state transition in the TUI.
// Tests can observe these via BubbleTeaUI.Events to assert on behavior
// without parsing rendered terminal output.
type UIEvent struct {
	Kind EventKind
	Data map[string]string // optional key-value payload
}

// BubbleTeaUI implements commands.UI for the full-screen TUI.
// Interactive methods block on channels while the Bubble Tea event loop
// shows popups or launches $EDITOR via tea.ExecProcess.
type BubbleTeaUI struct {
	EditorCmd string
	program   *tea.Program  // Set when TUI is running.
	sendFn    func(tea.Msg) // Override for tests (e.g. teatest.TestModel.Send).
	keys      terminal.KeyMap

	// Events receives state transition events. Nil in production;
	// tests allocate a buffered channel to observe behavior.
	Events chan UIEvent

	mu        sync.Mutex
	selectCh  chan int
	confirmCh chan bool
	inputCh   chan inputResponse
	editDocCh chan editDocResponse
}

type inputResponse struct {
	text      string
	cancelled bool
}

type editDocResponse struct {
	content string
	err     error
}

// NewBubbleTeaUI creates a new BubbleTeaUI instance with default keybindings.
func NewBubbleTeaUI() *BubbleTeaUI {
	return &BubbleTeaUI{
		keys: terminal.DefaultKeyMap(),
	}
}

// SetProgram attaches the running Bubble Tea program for suspend/resume.
func (b *BubbleTeaUI) SetProgram(p *tea.Program) {
	b.program = p
}

// Emit sends a state transition event if an observer is listening.
// No-op in production (Events is nil).
func (b *BubbleTeaUI) Emit(kind EventKind, kv ...string) {
	if b.Events == nil {
		return
	}
	var data map[string]string
	if len(kv) >= 2 {
		data = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			data[kv[i]] = kv[i+1]
		}
	}
	select {
	case b.Events <- UIEvent{Kind: kind, Data: data}:
	default:
	}
}

// send delivers a message to the running Bubble Tea program.
// Tests can override this via sendFn to route through teatest.TestModel.Send.
func (b *BubbleTeaUI) send(msg tea.Msg) {
	if b.sendFn != nil {
		b.sendFn(msg)
		return
	}
	b.program.Send(msg)
}

// ── Fire-and-forget methods ──

func (b *BubbleTeaUI) Notify(title, message string) {
	if b.program != nil || b.sendFn != nil {
		b.send(notifyMsg{title: title, message: message})
		return
	}
	fmt.Fprintf(os.Stderr, "  %s: %s\n", title, message)
}

func (b *BubbleTeaUI) Status(message string) {
	if b.program != nil || b.sendFn != nil {
		b.send(statusMsg(message))
		return
	}
	fmt.Fprintf(os.Stderr, "  %s\n", message)
}

func (b *BubbleTeaUI) CopyToClipboard(text string) error {
	return terminal.CopyToClipboard(text)
}

// ── Channel-bridge interactive methods ──

func (b *BubbleTeaUI) Select(title string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, nil
	}

	ch := make(chan int, 1)

	b.mu.Lock()
	b.selectCh = ch
	b.mu.Unlock()

	b.send(bridgeSelectMsg{title: title, options: options})

	idx := <-ch

	b.mu.Lock()
	b.selectCh = nil
	b.mu.Unlock()

	return idx, nil
}

func (b *BubbleTeaUI) Confirm(prompt string) (bool, error) {
	ch := make(chan bool, 1)

	b.mu.Lock()
	b.confirmCh = ch
	b.mu.Unlock()

	b.send(bridgeConfirmMsg{prompt: prompt})
	yes := <-ch

	b.mu.Lock()
	b.confirmCh = nil
	b.mu.Unlock()

	return yes, nil
}

func (b *BubbleTeaUI) InputText(prompt, initial string) (string, error) {
	ch := make(chan inputResponse, 1)

	b.mu.Lock()
	b.inputCh = ch
	b.mu.Unlock()

	b.send(bridgeInputMsg{prompt: prompt, initial: initial})
	resp := <-ch

	b.mu.Lock()
	b.inputCh = nil
	b.mu.Unlock()

	if resp.cancelled {
		return "", nil
	}
	return resp.text, nil
}

func (b *BubbleTeaUI) PromptText(prompt string) (string, error) {
	return b.InputText(prompt, "")
}

func (b *BubbleTeaUI) EditDocument(initial, prefix string) (string, error) {
	ch := make(chan editDocResponse, 1)

	b.mu.Lock()
	b.editDocCh = ch
	b.mu.Unlock()

	b.send(bridgeEditDocMsg{initial: initial, prefix: prefix})
	resp := <-ch

	b.mu.Lock()
	b.editDocCh = nil
	b.mu.Unlock()

	return resp.content, resp.err
}

func (b *BubbleTeaUI) PromptSecret(prompt string) (string, error) {
	// Auth commands run in headless mode; this fallback is for completeness.
	return b.PromptText(prompt)
}

func (b *BubbleTeaUI) ReviewDiff(title string, changes []commands.FieldDiff, options []string) (int, error) {
	// ReviewDiff is only used by the apply command which isn't invoked from TUI mode.
	return -1, fmt.Errorf("ReviewDiff is not supported in TUI mode")
}

// ── Resolve helpers (called by AppModel.Update) ──

func (b *BubbleTeaUI) resolveSelect(index int) {
	b.mu.Lock()
	ch := b.selectCh
	b.mu.Unlock()
	if ch != nil {
		ch <- index
	}
}

func (b *BubbleTeaUI) resolveConfirm(yes bool) {
	b.mu.Lock()
	ch := b.confirmCh
	b.mu.Unlock()
	if ch != nil {
		ch <- yes
	}
}

func (b *BubbleTeaUI) resolveInput(text string, cancelled bool) {
	b.mu.Lock()
	ch := b.inputCh
	b.mu.Unlock()
	if ch != nil {
		ch <- inputResponse{text: text, cancelled: cancelled}
	}
}

func (b *BubbleTeaUI) resolveEditDoc(content string, err error) {
	b.mu.Lock()
	ch := b.editDocCh
	b.mu.Unlock()
	if ch != nil {
		ch <- editDocResponse{content: content, err: err}
	}
}

// ── Internal message types ──

type notifyMsg struct {
	title   string
	message string
}

type statusMsg string
