# TUI Guide

The `ihj` TUI is a split-pane terminal interface: a detail pane (top) showing the selected issue, and a list pane (bottom) with fuzzy search.

## Detail Pane

The detail pane displays the selected issue's metadata in sections:

- **Header** — issue key, type, status, priority, and summary.
- **Ownership** — assignee, reporter (paired with temporal fields).
- **Temporal** — created and updated dates.
- **Iteration** — sprint name (scrum boards only, shown when populated).
- **Categorisation** — labels, components (shown when populated).
- **Parent** — parent issue link (shown when set).
- **FIELDS** — custom and dynamic fields discovered from the provider. Auto-discovered fields only appear when they have a value. Pinned fields (configured via `fields` on the issue type) always appear, with an em dash if empty.
- **Description** — rendered Markdown from the issue body.
- **Comments** — the three most recent comments.

## Layout

- **Enter** expands the detail pane to fill the entire terminal (focus mode).
- **Tab** toggles keyboard focus between panes without changing the layout.
- **Esc** exits focus mode, then clears search, then quits (in that priority order).

When the detail pane is focused (via Tab or Enter), `Up`/`Down` scroll the detail content, hint keys (`0`-`9`, then `a`-`z`) navigate child issues, and `Backspace` pops back one level. All action keys work regardless of focus state.

## Key Bindings (Default Mode)

### Navigation

| Key                     | Action                                  |
| ----------------------- | --------------------------------------- |
| `Up` / `Down`           | Move cursor (or scroll detail when focused) |
| `Home` / `End`          | Jump to first / last issue              |
| `PgUp` / `PgDown`       | Page up / down                          |
| `Shift+Up` / `Ctrl+U`   | Scroll detail up (from list focus)      |
| `Shift+Down` / `Ctrl+D` | Scroll detail down (from list focus)    |
| `Enter`                 | Focus mode (full-screen detail)         |
| `Tab`                   | Toggle focus between list / detail pane |
| `0`-`9`, `a`-`z`        | Navigate to child issue by hint key     |
| `Backspace`             | Go back (pop child history)             |
| `Esc`                   | Exit focus / clear search / quit        |
| `Ctrl+C`                | Quit                                    |

### Actions

| Key      | Action                             |
| -------- | ---------------------------------- |
| `Alt+E`  | Edit selected issue (opens editor) |
| `Ctrl+N` | Create new issue                   |
| `Alt+T`  | Transition (change status)         |
| `Alt+A`  | Assign to yourself                 |
| `Alt+C`  | Add comment                        |
| `Alt+O`  | Open in browser                    |
| `Alt+N`  | Copy git branch name to clipboard  |
| `Alt+X`  | Extract issue context for LLM      |
| `Alt+F`  | Switch filter                      |
| `Alt+W`  | Switch workspace                   |
| `Alt+R`  | Refresh data                       |
| `Alt+/`  | Show help overlay                  |

### Search

Type any character to start fuzzy filtering. Matches across issue key, summary, assignee, status, and type. Press `Esc` to clear the filter.

## Vim Mode

Enable with `vim_mode: true` in your config. Replaces modifier-key bindings with a modal interface.

### Normal Mode

Single-character keys for actions and navigation:

| Key   | Action                             |
| ----- | ---------------------------------- |
| `j`/`k` | Move cursor down / up            |
| `g`/`G` | Jump to first / last issue       |
| `e`   | Edit selected issue                |
| `n`   | Create new issue                   |
| `t`   | Transition (change status)         |
| `a`   | Assign to yourself                 |
| `c`   | Add comment                        |
| `o`   | Open in browser                    |
| `b`   | Copy git branch name               |
| `x`   | Extract issue context for LLM      |
| `f`   | Switch filter                      |
| `w`   | Switch workspace                   |
| `r`   | Refresh data                       |
| `/`   | Enter search mode                  |
| `:`   | Enter command mode                 |
| `Enter` | Focus mode (full-screen detail)  |
| `Tab` | Toggle focus between panes         |
| `Backspace` | Go back (pop child history)  |
| `Esc` | Exit focus / clear search          |
| `?`   | Show help overlay                  |

### Search Mode

Press `/` to enter. Type to fuzzy filter. `Enter` or `Esc` returns to normal mode. The filter is preserved.

### Command Mode

Press `:` to enter. Supported commands: `:q`, `:quit`, `:h`, `:help`.

## Layout Configuration

The detail pane height and help bar visibility are configurable:

```yaml
layout:
  detail_height: 55    # Percentage of available height (20-80, default 55)
  show_help_bar: true  # Show key binding bar (default true)
```

When `show_help_bar` is `false`, the space is reclaimed for content. In vim mode, a minimal mode indicator (NORMAL / `/` / `:`) is always shown regardless of this setting. The `?` help overlay remains accessible either way.

## Custom Shortcuts

Default-mode action keys can be remapped. Ignored when `vim_mode` is enabled.

```yaml
shortcuts:
  extract: "ctrl+x"
  branch: "ctrl+b"
```

Available actions: `refresh`, `filter`, `workspace`, `edit`, `new`, `transition`, `assign`, `comment`, `open`, `branch`, `extract`.

Shortcuts must include a modifier prefix (`alt+`, `ctrl+`, `super+`, `hyper+`). Collisions with reserved bindings are rejected at config load.
