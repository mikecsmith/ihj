# Desktop App (POC)

Experimental Wails v2 desktop frontend for ihj. Lives on the `poc/desktop` branch while under evaluation.

## Prerequisites

All tools are managed by mise:

```bash
mise install
```

You also need [Wails v2](https://wails.io) installed:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

A valid `JIRA_BASIC_TOKEN` environment variable is required for Jira workspaces (same as the CLI/TUI).

## Development

```bash
mise run desktop:dev
```

This runs `wails dev` which starts the Go backend and a Vite dev server for the Svelte frontend with hot-reload.

## Production Build

```bash
mise run desktop:build                    # current platform
mise run desktop:build:darwin-universal   # universal macOS binary
```

Production builds use the `desktop` build tag to embed the compiled frontend assets. The icon PNG is regenerated from SVG automatically before each build.

## Icon

The app icon source is `cmd/ihj-desktop/build/appicon.svg`. To regenerate the PNG manually:

```bash
mise run desktop:icon
```

Uses `resvg` (installed via mise) for high-fidelity SVG rendering.

## Architecture

The desktop app reuses the same `commands.*` business logic as the CLI/TUI. The difference is how user interaction is handled:

```
cmd/ihj-desktop/main.go     Wails app setup, config loading (mirrors cmd/ihj)
internal/desktop/app.go     Wails-bound backend — exports methods to frontend
internal/desktop/ui.go      DesktopUI — implements commands.UI via channel bridge
internal/desktop/dto.go     Data transfer objects for Go <-> frontend
cmd/ihj-desktop/frontend/   Svelte + TypeScript frontend
```

**Channel bridge pattern**: When a `commands.*` function needs user input (e.g. `UI.Select()`), `DesktopUI` emits a Wails event to the frontend and blocks on a Go channel. The Svelte `uiBridge.ts` listens for these events, opens the appropriate popup component, and calls the corresponding `Resolve*` binding when the user responds — unblocking the Go side.

This means the full command orchestration (including recovery loops, validation, and post-actions) runs identically to the CLI — no duplicated flow logic in the frontend.

## Frontend Structure

```
src/App.svelte                  Root layout — list + detail + popups
src/lib/uiBridge.ts             Subscribes to Wails events, bridges to popup stores
src/lib/stores/                 Svelte stores (items, UI state, theme)
src/lib/bindings.ts             Typed wrappers around Wails-generated Go bindings
src/components/                 UI components (list, detail, popups, action bar)
```
