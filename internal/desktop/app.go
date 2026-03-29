// Package desktop implements the Wails v2 backend for the ihj desktop app.
// It exposes methods that Wails auto-binds to the Svelte frontend.
package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// App is the Wails-bound backend. Its exported methods become callable
// from the frontend via the generated TypeScript bindings.
type App struct {
	ctx     context.Context
	session *commands.WorkspaceSession
	factory commands.WorkspaceSessionFactory
	rt      *commands.Runtime
	ui      *DesktopUI
}

// NewApp creates a new App backed by the given session, factory, runtime, and UI bridge.
func NewApp(session *commands.WorkspaceSession, factory commands.WorkspaceSessionFactory, rt *commands.Runtime, ui *DesktopUI) *App {
	return &App{
		session: session,
		factory: factory,
		rt:      rt,
		ui:      ui,
	}
}

// Startup is called by Wails when the app starts. It stores the context
// needed for runtime calls (clipboard, browser, etc.).
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.ui.SetContext(ctx)
}

// DomReady is called when the frontend DOM is ready.
func (a *App) DomReady(_ context.Context) {}

// ── Data ──

// GetWorkspace returns the current workspace configuration.
func (a *App) GetWorkspace() *core.Workspace {
	return a.session.Workspace
}

// Search returns work items matching the named filter.
// Items are returned as a sorted, hierarchical tree (roots with nested children).
// Pass noCache=true to bypass the disk cache and fetch fresh data.
func (a *App) Search(filter string, noCache bool) ([]viewItem, error) {
	items, err := a.session.Provider.Search(a.ctx, filter, noCache)
	if err != nil {
		return nil, err
	}

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	roots := core.Roots(registry)
	core.SortItems(roots, a.session.Workspace.StatusWeights, a.session.Workspace.TypeOrderMap)

	// Sort children recursively too.
	var sortChildren func(items []*core.WorkItem)
	sortChildren = func(items []*core.WorkItem) {
		for _, item := range items {
			if len(item.Children) > 0 {
				core.SortItems(item.Children, a.session.Workspace.StatusWeights, a.session.Workspace.TypeOrderMap)
				sortChildren(item.Children)
			}
		}
	}
	sortChildren(roots)

	return toView(roots), nil
}

// FieldDefinitions returns metadata describing the provider's fields.
func (a *App) FieldDefinitions() []core.FieldDef {
	return a.session.Provider.FieldDefinitions()
}

// ── Bulk operations ──

// ExportManifest exports the workspace as a YAML manifest string.
func (a *App) ExportManifest(filter string, full bool) (string, error) {
	items, err := a.session.Provider.Search(a.ctx, filter, true)
	if err != nil {
		return "", err
	}

	defs := a.session.Provider.FieldDefinitions()

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	roots := core.Roots(registry)

	manifest := core.Manifest{
		Metadata: core.Metadata{
			Workspace: a.session.Workspace.Slug,
		},
		Items: roots,
	}

	var buf bytes.Buffer
	if err := core.EncodeManifest(&buf, &manifest, defs, full, "yaml"); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ManifestSchema returns the JSON schema for validating manifest YAML.
func (a *App) ManifestSchema() (map[string]any, error) {
	defs := a.session.Provider.FieldDefinitions()
	schema := core.ManifestSchema(a.session.Workspace, defs)

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshaling schema: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("converting schema: %w", err)
	}
	return result, nil
}

// ── Misc ──

// OpenInBrowser opens a URL in the user's default browser.
func (a *App) OpenInBrowser(url string) {
	wailsRuntime.BrowserOpenURL(a.ctx, url)
}

// ── UI Bridge Resolve Methods ──
// These are called by the frontend to unblock pending commands.UI calls.

func (a *App) ResolveSelect(index int)                       { a.ui.ResolveSelect(index) }
func (a *App) ResolveConfirm(yes bool)                       { a.ui.ResolveConfirm(yes) }
func (a *App) ResolveEditText(text string, cancelled bool)   { a.ui.ResolveEditText(text, cancelled) }
func (a *App) ResolveEditDocument(metadata map[string]string, description string, cancelled bool) {
	a.ui.ResolveEditDocument(metadata, description, cancelled)
}
func (a *App) ResolvePromptText(text string, cancelled bool) { a.ui.ResolvePromptText(text, cancelled) }
func (a *App) ResolveReviewDiff(index int)                   { a.ui.ResolveReviewDiff(index) }

// ── Command-delegating methods ──
// All user interaction (popups, forms, toasts) is handled by the DesktopUI
// event bridge — the commands layer drives everything.

// RunAssign delegates to commands.Assign.
func (a *App) RunAssign(id string) error {
	return commands.Assign(a.session, id)
}

// RunTransition delegates to commands.Transition (uses UI.Select for status).
func (a *App) RunTransition(id string) error {
	return commands.Transition(a.session, id)
}

// RunCreate delegates to commands.Create. Type selection → form → submission
// all happen via DesktopUI events (Select → EditDocument → Notify).
func (a *App) RunCreate() error {
	return commands.Create(a.session, nil)
}

// RunEdit delegates to commands.Edit. Form popup appears via EditDocument event.
func (a *App) RunEdit(id string) error {
	return commands.Edit(a.session, id, nil)
}

// RunComment delegates to commands.Comment. Input popup appears via InputText event.
func (a *App) RunComment(id string) error {
	return commands.Comment(a.session, id)
}

// RunBranch delegates to commands.Branch.
func (a *App) RunBranch(id string) error {
	return commands.Branch(a.session, id)
}

// RunExtract delegates to commands.Extract. Scope selection and prompt input
// happen via Select and InputText events.
func (a *App) RunExtract(id string) error {
	return commands.Extract(a.session, id, commands.ExtractOptions{Copy: true})
}

// RunApplyManifest delegates to commands.ApplyContent for the full per-item
// review loop with diff display.
func (a *App) RunApplyManifest(yamlContent string) error {
	return commands.ApplyContent(a.rt, a.factory, []byte(yamlContent))
}
