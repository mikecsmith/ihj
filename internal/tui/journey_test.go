package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/testutil"
)

// ── Key helpers ──
//
// All key events are derived from the KeyMap so that future keybinding
// changes automatically propagate to the journey tests.

// keyMsg converts a key.Binding's first key string into a tea.KeyPressMsg.
// This ensures tests stay in sync with the actual keymap.
func keyMsg(b key.Binding) tea.KeyPressMsg {
	keys := b.Keys()
	if len(keys) == 0 {
		panic("keyMsg: binding has no keys")
	}
	return parseKeyStr(keys[0])
}

// parseKeyStr converts a key string (e.g. "alt+c", "ctrl+j", "esc") to a KeyPressMsg.
func parseKeyStr(s string) tea.KeyPressMsg {
	var mod tea.KeyMod
	parts := strings.Split(s, "+")

	// Parse modifier prefixes.
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl":
			mod |= tea.ModCtrl
		case "alt":
			mod |= tea.ModAlt
		case "shift":
			mod |= tea.ModShift
		}
	}

	// Parse the final key name.
	name := parts[len(parts)-1]
	var code rune
	switch name {
	case "enter":
		code = tea.KeyEnter
	case "esc":
		code = tea.KeyEscape
	case "up":
		code = tea.KeyUp
	case "down":
		code = tea.KeyDown
	case "left":
		code = tea.KeyLeft
	case "right":
		code = tea.KeyRight
	case "home":
		code = tea.KeyHome
	case "end":
		code = tea.KeyEnd
	case "pgup":
		code = tea.KeyPgUp
	case "pgdown":
		code = tea.KeyPgDown
	case "tab":
		code = tea.KeyTab
	case "backspace":
		code = tea.KeyBackspace
	case "delete":
		code = tea.KeyDelete
	case "space":
		code = tea.KeySpace
	default:
		if len(name) == 1 {
			code = rune(name[0])
		}
	}
	return tea.KeyPressMsg{Code: code, Mod: mod}
}

// interceptEditor configures the BubbleTeaUI to intercept bridgeEditDocMsg
// and resolve the editDocCh directly with transformed content, bypassing
// tea.ExecProcess entirely. This avoids external process dependencies
// and makes the test purely Go-driven.
func interceptEditor(ui *BubbleTeaUI, tm *teatest.TestModel, transform func(string) string) {
	baseSend := tm.Send
	ui.sendFn = func(msg tea.Msg) {
		if editMsg, ok := msg.(bridgeEditDocMsg); ok {
			ui.resolveEditDoc(transform(editMsg.initial), nil)
			return
		}
		baseSend(msg)
	}
}

// hasClipboard reports whether a clipboard utility is available.
func hasClipboard() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("pbcopy")
		return err == nil
	case "linux":
		for _, cmd := range []string{"wl-copy", "xclip", "xsel"} {
			if _, err := exec.LookPath(cmd); err == nil {
				return true
			}
		}
	}
	return false
}

// ── Test model construction ──

// keys is the shared keymap used across all journey tests.
var keys = terminal.DefaultKeyMap()
var vimKeys = terminal.VimKeyMap()

// buildJourneyModel creates a fully wired model for teatest journey tests.
// Pass custom items and vimMode; use journeyModel() for the common case.
func buildJourneyModel(t *testing.T, items []*core.WorkItem, vimMode bool) (AppModel, *BubbleTeaUI, *testutil.TestHarness) {
	t.Helper()
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat" // safe no-op editor for tests
	h := testutil.NewTestHarness(t, ui)
	h.Provider.SearchReturn = items
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, vimMode, nil, 0, true)
	m.ready = false // let teatest handle window sizing
	return m, ui, h
}

// journeyModel creates a default-mode model with standard test items.
func journeyModel(t *testing.T) (AppModel, *BubbleTeaUI, *testutil.TestHarness) {
	t.Helper()
	return buildJourneyModel(t, testutil.TestItems(), false)
}

// startJourney creates the teatest model and wires the BubbleTeaUI send function
// so that bridge messages are delivered through the test model's event loop.
// It also allocates the Events channel for event-driven assertions.
func startJourney(t *testing.T, m AppModel, ui *BubbleTeaUI) *teatest.TestModel {
	t.Helper()
	ui.Events = make(chan UIEvent, 100)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	ui.sendFn = tm.Send
	return tm
}

// waitForEvent drains the UI event channel until an event with the given kind
// is found, returning it. Skips unrelated events. Fails after timeout.
func waitForEvent(t *testing.T, ui *BubbleTeaUI, kind EventKind) UIEvent {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case evt := <-ui.Events:
			if evt.Kind == kind {
				return evt
			}
		case <-timeout:
			t.Fatalf("timed out waiting for event %q", kind)
			return UIEvent{}
		}
	}
}

// typeText sends a string character by character.
// In bubbletea v2, textarea reads from msg.Text, not msg.Code.
func typeText(tm *teatest.TestModel, text string) {
	for _, ch := range text {
		tm.Send(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
}

// ── Journey: Comment on an issue ──
//
// Flow: render → Comment key → popup appears → type comment → submit → notification
func TestJourney_Comment(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Start comment flow.
	tm.Send(keyMsg(keys.Comment))

	// Wait for the input popup.
	evt := waitForEvent(t, ui, EventPopupInput)
	if !strings.Contains(evt.Data["title"], "Comment") {
		t.Errorf("popup title = %q, want Comment", evt.Data["title"])
	}

	// Type the comment text and submit.
	typeText(tm, "This is a test comment")
	tm.Send(keyMsg(keys.Submit))

	// Wait for the success notification.
	evt = waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Added comment") {
		t.Errorf("notify = %q, want 'Added comment'", evt.Data["message"])
	}

	// Verify the provider received the comment.
	if len(h.Provider.CommentCalls) != 1 {
		t.Fatalf("expected 1 comment call, got %d", len(h.Provider.CommentCalls))
	}
	if h.Provider.CommentCalls[0].ID != "TEST-1" {
		t.Errorf("comment call issue ID = %q, want TEST-1", h.Provider.CommentCalls[0].ID)
	}
	if h.Provider.CommentCalls[0].Body != "This is a test comment" {
		t.Errorf("comment call body = %q, want %q", h.Provider.CommentCalls[0].Body, "This is a test comment")
	}
}

// ── Journey: Cancel a comment ──
//
// Flow: render → Comment key → popup appears → Cancel → "Cancelled" notification
func TestJourney_CommentCancel(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Comment))
	waitForEvent(t, ui, EventPopupInput)

	// Cancel the popup.
	tm.Send(keyMsg(keys.Cancel))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Cancelled") {
		t.Errorf("notify = %q, want 'Cancelled'", evt.Data["message"])
	}

	if len(h.Provider.CommentCalls) != 0 {
		t.Errorf("expected 0 comment calls after cancel, got %d", len(h.Provider.CommentCalls))
	}
}

// ── Journey: Transition an issue ──
//
// Flow: render → Transition key → select popup → navigate to "In Review" → enter → notification
func TestJourney_Transition(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Transition))
	waitForEvent(t, ui, EventPopupSelect)

	// Navigate down to "In Review" (statuses: Backlog, To Do, In Progress, In Review, Done).
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Focus))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "In Review") {
		t.Errorf("notify = %q, want 'In Review'", evt.Data["message"])
	}

	if len(h.Provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(h.Provider.UpdateCalls))
	}
	if h.Provider.UpdateCalls[0].ID != "TEST-1" {
		t.Errorf("update call ID = %q, want TEST-1", h.Provider.UpdateCalls[0].ID)
	}
	if h.Provider.UpdateCalls[0].Changes.Status == nil || *h.Provider.UpdateCalls[0].Changes.Status != "In Review" {
		t.Errorf("update call status = %v, want 'In Review'", h.Provider.UpdateCalls[0].Changes.Status)
	}
}

// ── Journey: Transition via number key ──
//
// Flow: render → Transition key → select popup → press "4" (In Review) → notification
func TestJourney_TransitionByNumberKey(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Transition))
	waitForEvent(t, ui, EventPopupSelect)

	// Press '4' to select "In Review" directly (1-indexed).
	tm.Send(tea.KeyPressMsg{Code: '4'})

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "In Review") {
		t.Errorf("notify = %q, want 'In Review'", evt.Data["message"])
	}

	if len(h.Provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(h.Provider.UpdateCalls))
	}
}

// ── Journey: Assign issue to self ──
//
// Flow: render → Assign key → notification (no popup, Assign doesn't prompt)
func TestJourney_Assign(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Assign))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Assigned TEST-1") {
		t.Errorf("notify = %q, want 'Assigned TEST-1'", evt.Data["message"])
	}

	if len(h.Provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(h.Provider.AssignCalls))
	}
	if h.Provider.AssignCalls[0] != "TEST-1" {
		t.Errorf("assign call = %q, want TEST-1", h.Provider.AssignCalls[0])
	}
}

// ── Journey: Navigate then act ──
//
// Flow: render → Down → Assign key → notification references TEST-2
func TestJourney_NavigateThenAssign(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Move cursor down to the second item (TEST-2).
	tm.Send(keyMsg(keys.Down))

	tm.Send(keyMsg(keys.Assign))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Assigned TEST-2") {
		t.Errorf("notify = %q, want 'Assigned TEST-2'", evt.Data["message"])
	}

	if len(h.Provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(h.Provider.AssignCalls))
	}
	if h.Provider.AssignCalls[0] != "TEST-2" {
		t.Errorf("assign call = %q, want TEST-2", h.Provider.AssignCalls[0])
	}
}

// ── Journey: Filter switch ──
//
// Flow: add second filter → Filter key → popup → select "backlog" → data reload
func TestJourney_FilterSwitch(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	h.WS.Filters["backlog"] = "status = Backlog"
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, false, nil, 0, true)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Filter))
	waitForEvent(t, ui, EventPopupSelect)

	// "default" is index 0, "backlog" is index 1. Press Down then Enter.
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Focus))

	// After selecting a filter, a notification confirms the switch.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "BACKLOG") {
		t.Errorf("notify = %q, want 'BACKLOG'", evt.Data["message"])
	}
}

// ── Journey: Command guard prevents concurrent actions ──
//
// Verifies that pressing an action key while a popup is active
// doesn't launch a second command.
func TestJourney_CommandGuard(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Start a comment flow (this shows a popup).
	tm.Send(keyMsg(keys.Comment))
	waitForEvent(t, ui, EventPopupInput)

	// While the popup is active, try pressing Assign.
	// The popup captures all keys, so Assign shouldn't fire.
	tm.Send(keyMsg(keys.Assign))

	// Cancel the comment.
	tm.Send(keyMsg(keys.Cancel))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Cancelled") {
		t.Errorf("notify = %q, want 'Cancelled'", evt.Data["message"])
	}

	// Assign should NOT have been called.
	if len(h.Provider.AssignCalls) != 0 {
		t.Errorf("expected 0 assign calls while popup was active, got %d", len(h.Provider.AssignCalls))
	}
}

// ── Journey: Edit an issue ──
//
// Flow: render → Edit key → interceptEditor transforms doc → submit → notification
func TestJourney_Edit(t *testing.T) {
	m, ui, h := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Intercept editor: replace the summary line in the frontmatter.
	interceptEditor(ui, tm, func(doc string) string {
		return strings.Replace(doc, "summary: Epic One", "summary: Epic One Edited", 1)
	})

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Edit))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Updated") {
		t.Errorf("notify = %q, want 'Updated'", evt.Data["message"])
	}

	if len(h.Provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(h.Provider.UpdateCalls))
	}
	if h.Provider.UpdateCalls[0].ID != "TEST-1" {
		t.Errorf("update call ID = %q, want TEST-1", h.Provider.UpdateCalls[0].ID)
	}
	if h.Provider.UpdateCalls[0].Changes.Summary == nil || *h.Provider.UpdateCalls[0].Changes.Summary != "Epic One Edited" {
		t.Errorf("update call summary = %v, want 'Epic One Edited'", h.Provider.UpdateCalls[0].Changes.Summary)
	}
}

// ── Journey: Edit cancel (no changes) ──
//
// Flow: render → Edit key → interceptEditor returns unchanged doc → "Cancelled" notification
func TestJourney_EditCancel(t *testing.T) {
	m, ui, _ := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Intercept editor: return doc unchanged (no edits).
	interceptEditor(ui, tm, func(doc string) string { return doc })

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Edit))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Cancelled") {
		t.Errorf("notify = %q, want 'Cancelled'", evt.Data["message"])
	}
}

// ── Journey: Create a new issue ──
//
// Flow: render → New key → select type popup → pick "Task" → interceptEditor adds summary → notification
func TestJourney_Create(t *testing.T) {
	m, ui, h := journeyModel(t)
	h.Provider.CreatePrefix = "TEST"
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Intercept editor: replace the empty summary with a real one.
	interceptEditor(ui, tm, func(doc string) string {
		return strings.Replace(doc, "summary:", "summary: Brand New Task", 1)
	})

	waitForEvent(t, ui, EventReady)

	// Press New key to start create flow.
	tm.Send(keyMsg(keys.New))

	// Wait for type selection popup.
	waitForEvent(t, ui, EventPopupSelect)

	// Select "Task" (types: Epic=1, Story=2, Task=3, Spike=4, Sub-task=5).
	tm.Send(tea.KeyPressMsg{Code: '3'})

	// Wait for the creation success notification.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Created") {
		t.Errorf("notify = %q, want 'Created'", evt.Data["message"])
	}

	if h.Provider.CreateCounter != 1 {
		t.Fatalf("expected 1 create call, got %d", h.Provider.CreateCounter)
	}
}

// ── Journey: Extract (LLM context) ──
//
// Flow: render → Extract key → select scope popup → pick scope → input prompt → submit → clipboard notification
func TestJourney_Extract(t *testing.T) {
	if !hasClipboard() {
		t.Skip("no clipboard utility available, skipping extract journey test")
	}

	m, ui, _ := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Extract))

	// Wait for scope selection popup.
	waitForEvent(t, ui, EventPopupSelect)

	// Select "Selected issue only" (index 0).
	tm.Send(keyMsg(keys.Focus))

	// Wait for prompt input popup.
	waitForEvent(t, ui, EventPopupInput)

	// Type a prompt and submit via ctrl+s.
	typeText(tm, "Summarize this issue")
	tm.Send(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	// Wait for the clipboard success notification.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "LLM Ready") {
		t.Errorf("notify = %q, want 'LLM Ready'", evt.Data["message"])
	}
}

// ── Journey: Branch (copy branch command) ──
//
// Flow: render → Branch key → clipboard copy → notification
func TestJourney_Branch(t *testing.T) {
	if !hasClipboard() {
		t.Skip("no clipboard utility available, skipping branch journey test")
	}

	m, ui, _ := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Branch))

	// Branch copies a git checkout command and shows a notification.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Branch") {
		t.Errorf("notify = %q, want 'Branch'", evt.Data["message"])
	}
}

// ── Vim Mode Journey Tests ──
//
// These verify that end-to-end flows work with vim-mode single-char keys.

// vimJourneyModel creates a vim-mode model with standard test items.
func vimJourneyModel(t *testing.T) (AppModel, *BubbleTeaUI, *testutil.TestHarness) {
	t.Helper()
	return buildJourneyModel(t, testutil.TestItems(), true)
}

// ── Vim Journey: Comment on an issue ──
//
// Flow: render → 'c' → popup → type comment → submit → notification
func TestVimJourney_Comment(t *testing.T) {
	m, ui, h := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Press 'c' (vim single-char key for Comment).
	tm.Send(keyMsg(vimKeys.Comment))
	waitForEvent(t, ui, EventPopupInput)

	typeText(tm, "Vim comment")
	tm.Send(keyMsg(vimKeys.Submit))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Added comment") {
		t.Errorf("notify = %q, want 'Added comment'", evt.Data["message"])
	}

	if len(h.Provider.CommentCalls) != 1 {
		t.Fatalf("expected 1 comment call, got %d", len(h.Provider.CommentCalls))
	}
	if h.Provider.CommentCalls[0].Body != "Vim comment" {
		t.Errorf("comment body = %q, want %q", h.Provider.CommentCalls[0].Body, "Vim comment")
	}
}

// ── Vim Journey: Navigate with j/k then assign ──
//
// Flow: render → j (down) → 'a' (assign) → notification references TEST-2
func TestVimJourney_NavigateThenAssign(t *testing.T) {
	m, ui, h := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// 'j' moves down in vim mode.
	tm.Send(keyMsg(vimKeys.Down))

	// 'a' assigns in vim mode.
	tm.Send(keyMsg(vimKeys.Assign))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Assigned TEST-2") {
		t.Errorf("notify = %q, want 'Assigned TEST-2'", evt.Data["message"])
	}

	if len(h.Provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(h.Provider.AssignCalls))
	}
	if h.Provider.AssignCalls[0] != "TEST-2" {
		t.Errorf("assign call = %q, want TEST-2", h.Provider.AssignCalls[0])
	}
}

// ── Vim Journey: Search then transition ──
//
// Flow: render → / (search) → type query → Enter (exit search) → 't' (transition) → select → notification
func TestVimJourney_SearchThenTransition(t *testing.T) {
	m, ui, h := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter search mode.
	tm.Send(keyMsg(vimKeys.Search))

	// Type search query.
	typeText(tm, "TEST-2")

	// Exit search mode (Enter).
	tm.Send(keyMsg(vimKeys.Focus))

	// Transition the filtered issue.
	tm.Send(keyMsg(vimKeys.Transition))
	waitForEvent(t, ui, EventPopupSelect)

	// Navigate down to "In Review" and confirm.
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.Focus))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "In Review") {
		t.Errorf("notify = %q, want 'In Review'", evt.Data["message"])
	}

	if len(h.Provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(h.Provider.UpdateCalls))
	}
}

// ── Journey: Workspace switch ──
//
// Flow: render → Workspace key → popup appears → select second workspace → data loads
func TestJourney_WorkspaceSwitch(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	// Add a second workspace.
	ws2 := testutil.TestWorkspace()
	ws2.Slug = "platform"
	ws2.Name = "Platform"
	ws2.ServerAlias = "prod-jira"
	h.Runtime.Workspaces[ws2.Slug] = ws2
	h.Factory = func(slug string) (*commands.WorkspaceSession, error) {
		ws := h.Runtime.Workspaces[slug]
		return &commands.WorkspaceSession{
			Runtime:   h.Runtime,
			Workspace: ws,
			Provider:  h.Provider,
		}, nil
	}

	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, false, nil, 0, true)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Open workspace popup.
	tm.Send(keyMsg(keys.Workspace))
	waitForEvent(t, ui, EventPopupSelect)

	// Current workspace "Engineering" is at index 0. Select "Platform" at index 1.
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Focus))

	// After switching, the notification should confirm the new workspace.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Switched to Platform") {
		t.Errorf("notify = %q, want 'Switched to Platform'", evt.Data["message"])
	}
}

// ── Vim Journey: Command mode quit ──
//
// Flow: render → : (command mode) → type "q" → Enter → quit
func TestVimJourney_CommandQuit(t *testing.T) {
	m, ui, _ := vimJourneyModel(t)
	tm := startJourney(t, m, ui)

	waitForEvent(t, ui, EventReady)

	// Enter command mode and type :q.
	tm.Send(keyMsg(vimKeys.Command))
	typeText(tm, "q")
	tm.Send(keyMsg(vimKeys.Focus))

	// Verify the program quit.
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	if fm == nil {
		t.Fatal("FinalModel should not be nil after :q")
	}
}

// ── Journey: Focus mode and pane navigation ──

// journeyModelWithChildren creates a model with a single-child chain:
// TEST-1 (epic) → TEST-10 (story) → TEST-20 (task).
func journeyModelWithChildren(t *testing.T) (AppModel, *BubbleTeaUI, *testutil.TestHarness) {
	t.Helper()
	return buildJourneyModel(t, testutil.TestChildChain(), false)
}

func TestJourney_FocusMode_EnterAndEsc(t *testing.T) {
	m, ui, _ := journeyModelWithChildren(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter focus mode.
	tm.Send(keyMsg(keys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	// Esc exits focus mode.
	tm.Send(keyMsg(keys.Cancel))
	waitForEvent(t, ui, EventViewList)

	// Quit from list view.
	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_TabPaneFocus(t *testing.T) {
	m, ui, _ := journeyModelWithChildren(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Tab switches focus to detail pane.
	tm.Send(keyMsg(keys.Tab))
	waitForEvent(t, ui, EventViewDetail)

	// Tab again returns focus to list.
	tm.Send(keyMsg(keys.Tab))
	waitForEvent(t, ui, EventViewList)

	// Esc with list focused (no search) quits.
	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_ChildNavigation_HintKeys(t *testing.T) {
	m, ui, _ := journeyModelWithChildren(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter focus mode.
	tm.Send(keyMsg(keys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	// Press '0' to navigate to the sole child (TEST-10).
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})
	evt := waitForEvent(t, ui, EventNavigated)
	if evt.Data["id"] != "TEST-10" {
		t.Errorf("navigated to %q, want TEST-10", evt.Data["id"])
	}
	if evt.Data["breadcrumb"] != "TEST-1 "+core.GlyphArrow+" TEST-10" {
		t.Errorf("breadcrumb = %q, want %q", evt.Data["breadcrumb"], "TEST-1 "+core.GlyphArrow+" TEST-10")
	}

	// Press '0' again to navigate to grandchild (TEST-20).
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})
	evt = waitForEvent(t, ui, EventNavigated)
	if evt.Data["id"] != "TEST-20" {
		t.Errorf("navigated to %q, want TEST-20", evt.Data["id"])
	}

	// Backspace pops back to TEST-10.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	evt = waitForEvent(t, ui, EventBack)
	if evt.Data["id"] != "TEST-10" {
		t.Errorf("back to %q, want TEST-10", evt.Data["id"])
	}

	// Backspace again pops to TEST-1.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	evt = waitForEvent(t, ui, EventBack)
	if evt.Data["id"] != "TEST-1" {
		t.Errorf("back to %q, want TEST-1", evt.Data["id"])
	}

	// Backspace at root exits focus mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	waitForEvent(t, ui, EventViewList)

	// Back in list view — quit cleanly.
	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_ChildNavigation_EscExitsImmediately(t *testing.T) {
	m, ui, _ := journeyModelWithChildren(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter focus, navigate to sole child.
	tm.Send(keyMsg(keys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})
	evt := waitForEvent(t, ui, EventNavigated)
	if evt.Data["id"] != "TEST-10" {
		t.Errorf("navigated to %q, want TEST-10", evt.Data["id"])
	}

	// Esc should exit focus mode entirely (not pop child history).
	tm.Send(keyMsg(keys.Cancel))
	waitForEvent(t, ui, EventViewList)

	// Back in list view — quit cleanly.
	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_ChildNavigation_OnlyWhenDetailFocused(t *testing.T) {
	m, ui, _ := journeyModelWithChildren(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Without focus/tab, pressing '0' should go to search input, not child nav.
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})

	// No "navigated" event should fire. Verify by checking the channel is empty
	// after a brief delay (the key goes to search, not child nav).
	select {
	case evt := <-ui.Events:
		t.Fatalf("unexpected event %q when detail not focused", evt.Kind)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event.
	}

	// Clear search and quit.
	tm.Send(keyMsg(keys.Cancel)) // Clear search
	tm.Send(keyMsg(keys.Cancel)) // Quit
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_FocusMode_VimMode(t *testing.T) {
	// Uses the child chain — test only navigates one level so the 3-item chain works.
	m, ui, _ := buildJourneyModel(t, testutil.TestChildChain(), true)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter focus mode.
	tm.Send(keyMsg(vimKeys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	// Navigate to sole child via '0'.
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})
	evt := waitForEvent(t, ui, EventNavigated)
	if evt.Data["id"] != "TEST-10" {
		t.Errorf("navigated to %q, want TEST-10", evt.Data["id"])
	}

	// Backspace pops back to parent.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	evt = waitForEvent(t, ui, EventBack)
	if evt.Data["id"] != "TEST-1" {
		t.Errorf("back to %q, want TEST-1", evt.Data["id"])
	}

	// Esc exits focus.
	tm.Send(keyMsg(vimKeys.Cancel))
	waitForEvent(t, ui, EventViewList)

	// Quit via :q
	tm.Send(keyMsg(vimKeys.Command))
	typeText(tm, "q")
	tm.Send(keyMsg(vimKeys.Focus))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_LayoutConfig_DetailPct(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	// Use 70% detail height.
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, false, nil, 70, true)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Should render without error.
	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_LayoutConfig_HideHelpBar(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	// Help bar hidden.
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, false, nil, 0, false)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Navigation still works — focus mode enters and exits cleanly.
	tm.Send(keyMsg(keys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	tm.Send(keyMsg(keys.Cancel))
	waitForEvent(t, ui, EventViewList)

	// Help overlay still toggleable via '?'.
	tm.Send(keyMsg(keys.Help))
	// No event for help overlay — just verify it doesn't crash.

	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestJourney_LayoutConfig_HideHelpBar_VimMode(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	// Vim mode with help bar hidden — mode indicator should still render.
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, true, nil, 0, false)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Focus mode works.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForEvent(t, ui, EventViewFullscreen)

	// Search mode works — '/' enters, Esc exits.
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	// Command mode works — ':q' quits.
	tm.Send(tea.KeyPressMsg{Code: ':', Text: ":"})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

// ── HintKeys integration: verify hints render for many children ──

func TestJourney_ManyChildren_HintOverflow(t *testing.T) {
	// Create parent with 12 children to verify hints extend into letters.
	parent := &core.WorkItem{
		ID: "PAR-1", Summary: "Parent", Type: "Epic", Status: "In Progress",
		Fields: map[string]any{"priority": "High", "assignee": "Dev", "created": "1 Jan 2025", "updated": "1 Jan 2025"},
	}
	items := []*core.WorkItem{parent}
	for i := range 12 {
		items = append(items, &core.WorkItem{
			ID: fmt.Sprintf("CHD-%02d", i), Summary: fmt.Sprintf("Child %d", i),
			Type: "Task", Status: "To Do", ParentID: "PAR-1",
			Fields: map[string]any{"priority": "Medium", "created": "1 Jan 2025", "updated": "1 Jan 2025"},
		})
	}

	m, ui, _ := buildJourneyModel(t, items, false)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Enter focus mode to see children.
	tm.Send(keyMsg(keys.Focus))
	waitForEvent(t, ui, EventViewFullscreen)

	// Navigate using 'a' key — digits 0-9 label the first 10 children,
	// so 'a' reaches the 11th child. IDs are zero-padded so lex order
	// matches numeric order: CHD-00..CHD-11, making index 10 = CHD-10.
	tm.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})
	evt := waitForEvent(t, ui, EventNavigated)
	if evt.Data["id"] != "CHD-10" {
		t.Errorf("navigated to %q, want CHD-10", evt.Data["id"])
	}

	// Backspace back, then exit.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	waitForEvent(t, ui, EventBack)
	tm.Send(keyMsg(keys.Cancel))
	waitForEvent(t, ui, EventViewList)
	tm.Send(keyMsg(keys.Cancel))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
}

// ── Journey: Edge cases ──

func TestJourney_FilterSingleFilter(t *testing.T) {
	// Workspace has only the default filter — pressing Filter shows notification.
	m, ui, _ := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Filter))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Only one filter") {
		t.Errorf("notify = %q, want 'Only one filter'", evt.Data["message"])
	}
}

func TestJourney_WorkspaceSingleWorkspace(t *testing.T) {
	// Runtime has only one workspace — pressing Workspace shows notification.
	m, ui, _ := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	tm.Send(keyMsg(keys.Workspace))

	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "Only one workspace") {
		t.Errorf("notify = %q, want 'Only one workspace'", evt.Data["message"])
	}
}

// ── Journey: Error handling ──

func TestJourney_StartupAuthError_Quits(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	// Provider returns items for the initial load but errors on the
	// background startup refresh (Search is called again by fetchStartupData).
	h.Provider.SearchReturn = items
	h.Provider.SearchErr = fmt.Errorf("HTTP 401: Unauthorized")

	// Non-zero fetchedAt triggers the startup refresh in Init().
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Now(), ui, false, nil, 0, true)
	m.ready = false
	tm := startJourney(t, m, ui)

	// The startup refresh fails with 401 → fatalErr is set → program quits.
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	if app, ok := fm.(AppModel); ok {
		if app.Err() == nil {
			t.Fatal("expected fatal error after startup auth failure")
		}
		if !strings.Contains(app.Err().Error(), "401") {
			t.Errorf("Err() = %q, want '401'", app.Err())
		}
	} else {
		t.Fatal("FinalModel is not an AppModel")
	}
}

func TestJourney_RefreshError_ShowsNotification(t *testing.T) {
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	h := testutil.NewTestHarness(t, ui)
	items := testutil.TestItems()
	h.Provider.SearchReturn = items

	// Zero fetchedAt skips startup refresh — avoids race with the error we set below.
	m := NewAppModel(context.Background(), h.Runtime, h.Session, h.Factory, h.WS, "default", items, time.Time{}, ui, false, nil, 0, true)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForEvent(t, ui, EventReady)

	// Now make the provider fail, then trigger a manual refresh.
	h.Provider.SearchErr = fmt.Errorf("network timeout")
	tm.Send(keyMsg(keys.Refresh))

	// Non-startup errors show a notification instead of quitting.
	evt := waitForEvent(t, ui, EventNotify)
	if !strings.Contains(evt.Data["message"], "network timeout") {
		t.Errorf("notify = %q, want 'network timeout'", evt.Data["message"])
	}
}
