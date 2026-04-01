package tui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/mikecsmith/ihj/internal/commands"
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

// journeyModel creates a fully wired model for teatest journey tests.
// The returned MockProvider can be used to verify API calls made during the journey.
func journeyModel(t *testing.T) (AppModel, *BubbleTeaUI, *testutil.MockProvider) {
	t.Helper()

	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat" // safe no-op editor for tests
	provider := testutil.NewMockProvider()
	// Populate SearchReturn so post-command data reloads succeed.
	provider.SearchReturn = items
	rt := testutil.NewTestRuntime(ui)
	rt.CacheDir = t.TempDir()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := func(slug string) (*commands.WorkspaceSession, error) {
		return &commands.WorkspaceSession{
			Runtime:   rt,
			Workspace: ws,
			Provider:  provider,
		}, nil
	}

	m := NewAppModel(context.Background(), rt, wsSess, factory, ws, "default", items, time.Now(), ui, false)
	m.ready = false // let teatest handle window sizing
	return m, ui, provider
}

// startJourney creates the teatest model and wires the BubbleTeaUI send function
// so that bridge messages are delivered through the test model's event loop.
func startJourney(t *testing.T, m AppModel, ui *BubbleTeaUI) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	ui.sendFn = tm.Send
	return tm
}

// waitForText blocks until the output contains the target string.
func waitForText(t *testing.T, tm *teatest.TestModel, target string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), target)
	}, teatest.WithDuration(5*time.Second))
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
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Wait for initial render with issue list.
	waitForText(t, tm, "TEST-1")

	// Start comment flow.
	tm.Send(keyMsg(keys.Comment))

	// Wait for the input popup.
	waitForText(t, tm, "Comment on TEST-1")

	// Type the comment text and submit.
	typeText(tm, "This is a test comment")
	tm.Send(keyMsg(keys.Submit))

	// Wait for the success notification.
	waitForText(t, tm, "Added comment to TEST-1")

	// Verify the provider received the comment.
	if len(provider.CommentCalls) != 1 {
		t.Fatalf("expected 1 comment call, got %d", len(provider.CommentCalls))
	}
	if provider.CommentCalls[0].ID != "TEST-1" {
		t.Errorf("comment call issue ID = %q, want TEST-1", provider.CommentCalls[0].ID)
	}
	if provider.CommentCalls[0].Body != "This is a test comment" {
		t.Errorf("comment call body = %q, want %q", provider.CommentCalls[0].Body, "This is a test comment")
	}
}

// ── Journey: Cancel a comment ──
//
// Flow: render → Comment key → popup appears → Cancel → "Cancelled" notification
func TestJourney_CommentCancel(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Comment))
	waitForText(t, tm, "Comment on TEST-1")

	// Cancel the popup.
	tm.Send(keyMsg(keys.Cancel))

	waitForText(t, tm, "Cancelled")

	if len(provider.CommentCalls) != 0 {
		t.Errorf("expected 0 comment calls after cancel, got %d", len(provider.CommentCalls))
	}
}

// ── Journey: Transition an issue ──
//
// Flow: render → Transition key → select popup → navigate to "In Review" → enter → notification
func TestJourney_Transition(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Transition))

	// Wait for the select popup with statuses.
	waitForText(t, tm, "Transition")

	// Navigate down to "In Review" (statuses: Backlog, To Do, In Progress, In Review, Done).
	// Popup starts at index 0 (Backlog). Press Down 3 times to reach "In Review".
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.EnterChild)) // Enter confirms popup selection.

	// Wait for success notification.
	waitForText(t, tm, "Moved to In Review")

	// Verify provider received the update.
	if len(provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(provider.UpdateCalls))
	}
	if provider.UpdateCalls[0].ID != "TEST-1" {
		t.Errorf("update call ID = %q, want TEST-1", provider.UpdateCalls[0].ID)
	}
	if provider.UpdateCalls[0].Changes.Status == nil || *provider.UpdateCalls[0].Changes.Status != "In Review" {
		t.Errorf("update call status = %v, want 'In Review'", provider.UpdateCalls[0].Changes.Status)
	}
}

// ── Journey: Transition via number key ──
//
// Flow: render → Transition key → select popup → press "4" (In Review) → notification
func TestJourney_TransitionByNumberKey(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Transition))
	waitForText(t, tm, "Transition")

	// Press '4' to select "In Review" directly (1-indexed).
	tm.Send(tea.KeyPressMsg{Code: '4'})

	waitForText(t, tm, "Moved to In Review")

	if len(provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(provider.UpdateCalls))
	}
}

// ── Journey: Assign issue to self ──
//
// Flow: render → Assign key → notification (no popup, Assign doesn't prompt)
func TestJourney_Assign(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Assign))

	waitForText(t, tm, "Assigned TEST-1")

	if len(provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(provider.AssignCalls))
	}
	if provider.AssignCalls[0] != "TEST-1" {
		t.Errorf("assign call = %q, want TEST-1", provider.AssignCalls[0])
	}
}

// ── Journey: Navigate then act ──
//
// Flow: render → Down → Assign key → notification references TEST-2
func TestJourney_NavigateThenAssign(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	// Move cursor down to the second item (TEST-2).
	tm.Send(keyMsg(keys.Down))

	tm.Send(keyMsg(keys.Assign))

	waitForText(t, tm, "Assigned TEST-2")

	if len(provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(provider.AssignCalls))
	}
	if provider.AssignCalls[0] != "TEST-2" {
		t.Errorf("assign call = %q, want TEST-2", provider.AssignCalls[0])
	}
}

// ── Journey: Filter switch ──
//
// Flow: add second filter → Filter key → popup → select "backlog" → data reload
func TestJourney_FilterSwitch(t *testing.T) {
	ws := testutil.TestWorkspace()
	ws.Filters["backlog"] = "status = Backlog"

	items := testutil.TestItems()
	ui := NewBubbleTeaUI()
	provider := testutil.NewMockProvider()
	provider.SearchReturn = items
	rt := testutil.NewTestRuntime(ui)
	rt.CacheDir = t.TempDir()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := func(slug string) (*commands.WorkspaceSession, error) {
		return &commands.WorkspaceSession{
			Runtime:   rt,
			Workspace: ws,
			Provider:  provider,
		}, nil
	}

	m := NewAppModel(context.Background(), rt, wsSess, factory, ws, "default", items, time.Now(), ui, false)
	m.ready = false
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Filter))

	// Wait for filter popup.
	waitForText(t, tm, "Switch Filter")

	// "default" is index 0, "backlog" is index 1. Press Down then Enter.
	tm.Send(keyMsg(keys.Down))
	tm.Send(keyMsg(keys.EnterChild))

	// After selecting a filter, the app triggers a data reload.
	waitForText(t, tm, "BACKLOG")
}

// ── Journey: Command guard prevents concurrent actions ──
//
// Verifies that pressing an action key while a popup is active
// doesn't launch a second command.
func TestJourney_CommandGuard(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	// Start a comment flow (this shows a popup).
	tm.Send(keyMsg(keys.Comment))
	waitForText(t, tm, "Comment on TEST-1")

	// While the popup is active, try pressing Assign.
	// The popup captures all keys, so Assign shouldn't fire.
	tm.Send(keyMsg(keys.Assign))

	// Cancel the comment.
	tm.Send(keyMsg(keys.Cancel))
	waitForText(t, tm, "Cancelled")

	// Assign should NOT have been called.
	if len(provider.AssignCalls) != 0 {
		t.Errorf("expected 0 assign calls while popup was active, got %d", len(provider.AssignCalls))
	}
}

// ── Journey: Edit an issue ──
//
// Flow: render → Edit key → interceptEditor transforms doc → submit → notification
func TestJourney_Edit(t *testing.T) {
	m, ui, provider := journeyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Intercept editor: replace the summary line in the frontmatter.
	interceptEditor(ui, tm, func(doc string) string {
		return strings.Replace(doc, "summary: Epic One", "summary: Epic One Edited", 1)
	})

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Edit))

	// Wait for the success notification.
	waitForText(t, tm, "Updated")

	// Verify provider received the update with changed summary.
	if len(provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(provider.UpdateCalls))
	}
	if provider.UpdateCalls[0].ID != "TEST-1" {
		t.Errorf("update call ID = %q, want TEST-1", provider.UpdateCalls[0].ID)
	}
	if provider.UpdateCalls[0].Changes.Summary == nil || *provider.UpdateCalls[0].Changes.Summary != "Epic One Edited" {
		t.Errorf("update call summary = %v, want 'Epic One Edited'", provider.UpdateCalls[0].Changes.Summary)
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

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Edit))

	waitForText(t, tm, "Cancelled")
}

// ── Journey: Create a new issue ──
//
// Flow: render → New key → select type popup → pick "Task" → interceptEditor adds summary → notification
func TestJourney_Create(t *testing.T) {
	m, ui, provider := journeyModel(t)
	provider.CreatePrefix = "TEST"
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	// Intercept editor: replace the empty summary with a real one.
	interceptEditor(ui, tm, func(doc string) string {
		return strings.Replace(doc, "summary:", "summary: Brand New Task", 1)
	})

	waitForText(t, tm, "TEST-1")

	// Press New key to start create flow.
	tm.Send(keyMsg(keys.New))

	// Wait for type selection popup.
	waitForText(t, tm, "Create New Issue")

	// Select "Task" (types: Epic=1, Story=2, Task=3, Spike=4, Sub-task=5).
	tm.Send(tea.KeyPressMsg{Code: '3'})

	// Wait for the creation success notification.
	waitForText(t, tm, "Created")

	// Verify provider received the create call.
	if provider.CreateCounter != 1 {
		t.Fatalf("expected 1 create call, got %d", provider.CreateCounter)
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

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Extract))

	// Wait for scope selection popup.
	waitForText(t, tm, "Selected issue only")

	// Select "Selected issue only" (index 0).
	tm.Send(keyMsg(keys.EnterChild))

	// Wait for prompt input popup (the title contains "Prompt" and "XML context").
	waitForText(t, tm, "XML context")

	// Type a prompt and submit via ctrl+s (the second Submit binding).
	// alt+enter (the first Submit binding) can conflict with the textarea's
	// newline handling in some terminal emulators, so we use ctrl+s.
	typeText(tm, "Summarize this issue")
	tm.Send(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	// Wait for the clipboard success notification.
	waitForText(t, tm, "LLM Ready")
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

	waitForText(t, tm, "TEST-1")

	tm.Send(keyMsg(keys.Branch))

	// Branch copies a git checkout command and shows a notification.
	waitForText(t, tm, "Branch")
}

// ── Vim Mode Journey Tests ──
//
// These verify that end-to-end flows work with vim-mode single-char keys.

// vimJourneyModel creates a fully wired vim-mode model for teatest journey tests.
func vimJourneyModel(t *testing.T) (AppModel, *BubbleTeaUI, *testutil.MockProvider) {
	t.Helper()

	ws := testutil.TestWorkspace()
	items := testutil.TestItems()
	ui := NewBubbleTeaUI()
	ui.EditorCmd = "cat"
	provider := testutil.NewMockProvider()
	provider.SearchReturn = items
	rt := testutil.NewTestRuntime(ui)
	rt.CacheDir = t.TempDir()
	wsSess := &commands.WorkspaceSession{
		Runtime:   rt,
		Workspace: ws,
		Provider:  provider,
	}
	factory := func(slug string) (*commands.WorkspaceSession, error) {
		return &commands.WorkspaceSession{
			Runtime:   rt,
			Workspace: ws,
			Provider:  provider,
		}, nil
	}

	m := NewAppModel(context.Background(), rt, wsSess, factory, ws, "default", items, time.Now(), ui, true)
	m.ready = false
	return m, ui, provider
}

// ── Vim Journey: Comment on an issue ──
//
// Flow: render → 'c' → popup → type comment → submit → notification
func TestVimJourney_Comment(t *testing.T) {
	m, ui, provider := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	// Press 'c' (vim single-char key for Comment).
	tm.Send(keyMsg(vimKeys.Comment))

	waitForText(t, tm, "Comment on TEST-1")

	typeText(tm, "Vim comment")
	tm.Send(keyMsg(vimKeys.Submit))

	waitForText(t, tm, "Added comment to TEST-1")

	if len(provider.CommentCalls) != 1 {
		t.Fatalf("expected 1 comment call, got %d", len(provider.CommentCalls))
	}
	if provider.CommentCalls[0].Body != "Vim comment" {
		t.Errorf("comment body = %q, want %q", provider.CommentCalls[0].Body, "Vim comment")
	}
}

// ── Vim Journey: Navigate with j/k then assign ──
//
// Flow: render → j (down) → 'a' (assign) → notification references TEST-2
func TestVimJourney_NavigateThenAssign(t *testing.T) {
	m, ui, provider := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	// 'j' moves down in vim mode.
	tm.Send(keyMsg(vimKeys.Down))

	// 'a' assigns in vim mode.
	tm.Send(keyMsg(vimKeys.Assign))

	waitForText(t, tm, "Assigned TEST-2")

	if len(provider.AssignCalls) != 1 {
		t.Fatalf("expected 1 assign call, got %d", len(provider.AssignCalls))
	}
	if provider.AssignCalls[0] != "TEST-2" {
		t.Errorf("assign call = %q, want TEST-2", provider.AssignCalls[0])
	}
}

// ── Vim Journey: Search then transition ──
//
// Flow: render → / (search) → type query → Enter (exit search) → 't' (transition) → select → notification
func TestVimJourney_SearchThenTransition(t *testing.T) {
	m, ui, provider := vimJourneyModel(t)
	tm := startJourney(t, m, ui)
	defer func() { _ = tm.Quit() }()

	waitForText(t, tm, "TEST-1")

	// Enter search mode.
	tm.Send(keyMsg(vimKeys.Search))

	// Type search query.
	typeText(tm, "TEST-2")

	// Exit search mode (Enter).
	tm.Send(keyMsg(vimKeys.EnterChild))

	// Transition the filtered issue.
	tm.Send(keyMsg(vimKeys.Transition))

	waitForText(t, tm, "Transition")

	// Navigate down to "In Review" and confirm.
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.Down))
	tm.Send(keyMsg(vimKeys.EnterChild))

	waitForText(t, tm, "Moved to In Review")

	if len(provider.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(provider.UpdateCalls))
	}
}

// ── Vim Journey: Command mode quit ──
//
// Flow: render → : (command mode) → type "q" → Enter → quit
func TestVimJourney_CommandQuit(t *testing.T) {
	m, ui, _ := vimJourneyModel(t)
	tm := startJourney(t, m, ui)

	waitForText(t, tm, "TEST-1")

	// Enter command mode and type :q.
	tm.Send(keyMsg(vimKeys.Command))
	typeText(tm, "q")
	tm.Send(keyMsg(vimKeys.EnterChild))

	// Verify the program quit.
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	if fm == nil {
		t.Fatal("FinalModel should not be nil after :q")
	}
}
