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

// BubbleTeaUI implements commands.UI for the full-screen TUI.
// Interactive methods block on channels while the Bubble Tea event loop
// shows popups or launches $EDITOR via tea.ExecProcess.
type BubbleTeaUI struct {
	EditorCmd string
	program   *tea.Program // Set when TUI is running.
	keys      terminal.KeyMap

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

// ── Fire-and-forget methods ──

func (b *BubbleTeaUI) Notify(title, message string) {
	if b.program != nil {
		b.program.Send(notifyMsg{title: title, message: message})
		return
	}
	fmt.Fprintf(os.Stderr, "  %s: %s\n", title, message)
}

func (b *BubbleTeaUI) Status(message string) {
	if b.program != nil {
		b.program.Send(statusMsg(message))
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
	b.mu.Lock()
	defer b.mu.Unlock()
	b.selectCh = make(chan int, 1)
	b.program.Send(bridgeSelectMsg{title: title, options: options})
	idx := <-b.selectCh
	b.selectCh = nil
	return idx, nil
}

func (b *BubbleTeaUI) Confirm(prompt string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.confirmCh = make(chan bool, 1)
	b.program.Send(bridgeConfirmMsg{prompt: prompt})
	yes := <-b.confirmCh
	b.confirmCh = nil
	return yes, nil
}

func (b *BubbleTeaUI) InputText(prompt, initial string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inputCh = make(chan inputResponse, 1)
	b.program.Send(bridgeInputMsg{prompt: prompt, initial: initial})
	resp := <-b.inputCh
	b.inputCh = nil
	if resp.cancelled {
		return "", nil
	}
	return resp.text, nil
}

func (b *BubbleTeaUI) PromptText(prompt string) (string, error) {
	return b.InputText(prompt, "")
}

func (b *BubbleTeaUI) EditDocument(initial, prefix string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.editDocCh = make(chan editDocResponse, 1)
	b.program.Send(bridgeEditDocMsg{initial: initial, prefix: prefix})
	resp := <-b.editDocCh
	b.editDocCh = nil
	return resp.content, resp.err
}

func (b *BubbleTeaUI) ReviewDiff(title string, changes []commands.FieldDiff, options []string) (int, error) {
	// ReviewDiff is only used by the apply command which isn't invoked from TUI mode.
	return -1, fmt.Errorf("ReviewDiff is not supported in TUI mode")
}

// ── Resolve helpers (called by AppModel.Update) ──

func (b *BubbleTeaUI) resolveSelect(index int) {
	if b.selectCh != nil {
		b.selectCh <- index
	}
}

func (b *BubbleTeaUI) resolveConfirm(yes bool) {
	if b.confirmCh != nil {
		b.confirmCh <- yes
	}
}

func (b *BubbleTeaUI) resolveInput(text string, cancelled bool) {
	if b.inputCh != nil {
		b.inputCh <- inputResponse{text: text, cancelled: cancelled}
	}
}

func (b *BubbleTeaUI) resolveEditDoc(content string, err error) {
	if b.editDocCh != nil {
		b.editDocCh <- editDocResponse{content: content, err: err}
	}
}

// ── Internal message types ──

type notifyMsg struct {
	title   string
	message string
}

type statusMsg string
