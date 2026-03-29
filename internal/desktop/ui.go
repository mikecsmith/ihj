package desktop

import (
	"context"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// Compile-time check that DesktopUI satisfies commands.UI.
var _ commands.UI = (*DesktopUI)(nil)

// DesktopUI implements commands.UI by bridging Go blocking calls to the
// async Svelte popup system via Wails events. Interactive methods emit an
// event, then block on a channel until the frontend calls the corresponding
// Resolve* method.
type DesktopUI struct {
	ctx context.Context // Wails app context, set after Startup.
	mu  sync.Mutex      // Serializes interactive prompts (one popup at a time).

	selectCh    chan int
	confirmCh   chan bool
	editTextCh  chan editTextResponse // For EditDocument (BulkEditor)
	editDocCh   chan editDocResponse  // For EditDocument (form-based)
	promptCh    chan editTextResponse // For InputText and PromptText
	reviewCh    chan int
}

type editTextResponse struct {
	Text      string
	Cancelled bool
}

type editDocResponse struct {
	Metadata    map[string]string
	Description string
	Cancelled   bool
}

// NewDesktopUI creates an uninitialised DesktopUI. Call SetContext after
// Wails Startup to enable event emission.
func NewDesktopUI() *DesktopUI {
	return &DesktopUI{}
}

// SetContext stores the Wails app context needed for event emission.
func (u *DesktopUI) SetContext(ctx context.Context) {
	u.ctx = ctx
}

// ── Interactive methods (block until frontend resolves) ──

// Select presents a list of options and returns the chosen index.
func (u *DesktopUI) Select(title string, options []string) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.selectCh = make(chan int, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:select", map[string]any{
		"title":   title,
		"options": options,
	})

	select {
	case idx := <-u.selectCh:
		u.selectCh = nil
		return idx, nil
	case <-u.ctx.Done():
		u.selectCh = nil
		return -1, u.ctx.Err()
	}
}

// Confirm asks a yes/no question.
func (u *DesktopUI) Confirm(prompt string) (bool, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.confirmCh = make(chan bool, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:confirm", map[string]any{
		"prompt": prompt,
	})

	select {
	case yes := <-u.confirmCh:
		u.confirmCh = nil
		return yes, nil
	case <-u.ctx.Done():
		u.confirmCh = nil
		return false, u.ctx.Err()
	}
}

// InputText collects free-form text by reusing the prompt popup.
func (u *DesktopUI) InputText(prompt, initial string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.promptCh = make(chan editTextResponse, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:prompt", map[string]any{
		"prompt": prompt,
	})

	select {
	case res := <-u.promptCh:
		u.promptCh = nil
		if res.Cancelled {
			return "", nil
		}
		return res.Text, nil
	case <-u.ctx.Done():
		u.promptCh = nil
		return "", u.ctx.Err()
	}
}

// EditDocument parses YAML frontmatter, emits a structured form event,
// and re-serializes the user's edits back to a frontmatter document.
func (u *DesktopUI) EditDocument(initial, prefix string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Parse the YAML frontmatter into structured data for the form.
	metadata, body, parseErr := core.ParseFrontmatter(initial)
	if parseErr != nil {
		// Fall back to raw text editor if unparseable.
		return u.editDocumentRaw(initial)
	}

	u.editDocCh = make(chan editDocResponse, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:editdocument", map[string]any{
		"metadata":    metadata,
		"description": body,
	})

	select {
	case res := <-u.editDocCh:
		u.editDocCh = nil
		if res.Cancelled {
			return "", &commands.CancelledError{Operation: "edit"}
		}
		return core.BuildFrontmatterDoc("", res.Metadata, res.Description), nil
	case <-u.ctx.Done():
		u.editDocCh = nil
		return "", u.ctx.Err()
	}
}

// editDocumentRaw falls back to the BulkEditor for unparseable documents.
func (u *DesktopUI) editDocumentRaw(initial string) (string, error) {
	u.editTextCh = make(chan editTextResponse, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:edittext", map[string]any{
		"initial": initial,
	})

	select {
	case res := <-u.editTextCh:
		u.editTextCh = nil
		if res.Cancelled {
			return "", &commands.CancelledError{Operation: "edit"}
		}
		return res.Text, nil
	case <-u.ctx.Done():
		u.editTextCh = nil
		return "", u.ctx.Err()
	}
}

// PromptText asks for a single line of text input.
func (u *DesktopUI) PromptText(prompt string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.promptCh = make(chan editTextResponse, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:prompt", map[string]any{
		"prompt": prompt,
	})

	select {
	case res := <-u.promptCh:
		u.promptCh = nil
		if res.Cancelled {
			return "", &commands.CancelledError{Operation: "prompt"}
		}
		return res.Text, nil
	case <-u.ctx.Done():
		u.promptCh = nil
		return "", u.ctx.Err()
	}
}

// ReviewDiff presents field-level changes and asks the user to choose an action.
func (u *DesktopUI) ReviewDiff(title string, changes []commands.FieldDiff, options []string) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Convert FieldDiff to a serialisable form for the frontend.
	diffs := make([]map[string]string, len(changes))
	for i, c := range changes {
		diffs[i] = map[string]string{
			"field": c.Field,
			"old":   c.Old,
			"new":   c.New,
		}
	}

	u.reviewCh = make(chan int, 1)
	wailsRuntime.EventsEmit(u.ctx, "ui:reviewdiff", map[string]any{
		"title":   title,
		"changes": diffs,
		"options": options,
	})

	select {
	case idx := <-u.reviewCh:
		u.reviewCh = nil
		return idx, nil
	case <-u.ctx.Done():
		u.reviewCh = nil
		return -1, u.ctx.Err()
	}
}

// ── Fire-and-forget methods ──

// Notify displays a toast message in the frontend.
func (u *DesktopUI) Notify(title, message string) {
	if u.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(u.ctx, "ui:notify", map[string]any{
		"title":   title,
		"message": message,
	})
}

// Status shows a transient progress message.
func (u *DesktopUI) Status(message string) {
	if u.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(u.ctx, "ui:status", map[string]any{
		"message": message,
	})
}

// CopyToClipboard copies text to the system clipboard.
func (u *DesktopUI) CopyToClipboard(text string) error {
	return wailsRuntime.ClipboardSetText(u.ctx, text)
}

// ── Resolve methods (called by frontend via App bindings) ──

// ResolveSelect unblocks a pending Select call.
func (u *DesktopUI) ResolveSelect(index int) {
	if u.selectCh != nil {
		u.selectCh <- index
	}
}

// ResolveConfirm unblocks a pending Confirm call.
func (u *DesktopUI) ResolveConfirm(yes bool) {
	if u.confirmCh != nil {
		u.confirmCh <- yes
	}
}

// ResolveEditText unblocks a pending editDocumentRaw (BulkEditor) call.
func (u *DesktopUI) ResolveEditText(text string, cancelled bool) {
	if u.editTextCh != nil {
		u.editTextCh <- editTextResponse{Text: text, Cancelled: cancelled}
	}
}

// ResolveEditDocument unblocks a pending EditDocument (form) call.
func (u *DesktopUI) ResolveEditDocument(metadata map[string]string, description string, cancelled bool) {
	if u.editDocCh != nil {
		u.editDocCh <- editDocResponse{Metadata: metadata, Description: description, Cancelled: cancelled}
	}
}

// ResolvePromptText unblocks a pending PromptText call.
func (u *DesktopUI) ResolvePromptText(text string, cancelled bool) {
	if u.promptCh != nil {
		u.promptCh <- editTextResponse{Text: text, Cancelled: cancelled}
	}
}

// ResolveReviewDiff unblocks a pending ReviewDiff call.
func (u *DesktopUI) ResolveReviewDiff(index int) {
	if u.reviewCh != nil {
		u.reviewCh <- index
	}
}
