# ihj — Instant High-speed Jira 😉

Stop context-switching to a browser tab. `ihj` brings your Jira board into the terminal where you already live — navigate, triage, and update issues without leaving your workflow.

- **Fast.** Aggressive caching means your board loads instantly. Background refreshes keep it current.
- **Keyboard-driven.** Fuzzy search, single-key actions, and optional vim mode. No mouse required.
- **Multi-workspace.** Switch between projects (even across Jira instances) in a single keypress.
- **LLM-friendly.** Extract your backlog as structured context, feed it to an LLM, and apply the result back with a rich interactive diff.
- **Extensible.** Provider-agnostic core — built for Jira today, designed for GitHub Issues, Linear, and others tomorrow.

> **Early Stage Software** — Under active development but broadly stable. `ihj` makes
> **write calls** to your Jira instance (create, edit, transition, assign, comment).
> Use at your own risk — no warranties of any kind.

---

## Demo

### General Usage

<video src="https://github.com/user-attachments/assets/0b63889f-c75a-4670-939d-203f8ccbde94"></video>

### Bulk Apply

<video src="https://github.com/user-attachments/assets/652cb941-b43d-4bbf-893f-39a12ae5448a"></video>

## Quick Start

```bash
# 1. Bootstrap a workspace config from your Jira project.
#    You'll be prompted for your server URL, email, and API token.
#    Generate an API token at: https://id.atlassian.com/manage-profile/security/api-tokens
#    The credentials are stored securely (OS keychain when available).
ihj jira bootstrap PROJ > ~/.config/ihj/config.yaml

# 2. Launch the TUI.
ihj
```

To update or replace a stored token later: `ihj auth login <server-alias>`.

To try it without a Jira connection:

```bash
ihj jira demo
```

---

## Installation

```bash
go install github.com/mikecsmith/ihj/cmd/ihj@latest
```

Or build from source:

```bash
git clone https://github.com/mikecsmith/ihj.git
cd ihj
go build -o ihj ./cmd/ihj
```

---

## TUI Key Bindings

### Navigation

| Key                     | Action                                  |
| ----------------------- | --------------------------------------- |
| `Up` / `Ctrl+K`         | Move cursor up (or scroll detail pane)  |
| `Down` / `Ctrl+J`       | Move cursor down (or scroll detail pane)|
| `Home`                  | Jump to first issue                     |
| `End`                   | Jump to last issue                      |
| `PgUp` / `PgDown`       | Page up / down                          |
| `Shift+Up` / `Ctrl+U`   | Scroll detail up                        |
| `Shift+Down` / `Ctrl+D` | Scroll detail down                      |
| `Enter`                 | Focus mode (full-screen detail)         |
| `Tab`                   | Toggle focus between list / detail pane |
| `0`–`9`, `a`–`z`        | Navigate to child issue by hint key     |
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

### Pane Focus & Focus Mode

The TUI has two panes: a detail view (top) and a list view (bottom).

- **Tab** toggles keyboard focus between panes without changing the layout.
- **Enter** enters **focus mode** — the detail pane expands to fill the entire terminal.

When the detail pane is focused (via either Tab or Enter), the interaction model is the same: `Up`/`Down` scroll the detail content, child issue hints (`0`–`9`, then `a`–`z`) navigate the hierarchy, and `Backspace` goes back one level. All action keys continue to work regardless of focus state.

**Esc** exits focus mode (or clears search, or quits — in that priority order).

### Search

Type any character to start fuzzy filtering. The search matches across issue key, summary, assignee, status, and type. Press `Esc` to clear.

### Vim Mode

Enable vim-style key bindings with `vim_mode: true` in your config. This replaces the default alt-key bindings with a modal interface.

**Normal mode** — single-character keys for actions and navigation:

| Key   | Action                             |
| ----- | ---------------------------------- |
| `j`   | Move cursor down                   |
| `k`   | Move cursor up                     |
| `g`   | Jump to first issue                |
| `G`   | Jump to last issue                 |
| `e`   | Edit selected issue (opens editor) |
| `n`   | Create new issue                   |
| `t`   | Transition (change status)         |
| `a`   | Assign to yourself                 |
| `c`   | Add comment                        |
| `o`   | Open in browser                    |
| `b`   | Copy git branch name to clipboard  |
| `x`   | Extract issue context for LLM      |
| `f`   | Switch filter                      |
| `w`   | Switch workspace                   |
| `r`   | Refresh data                       |
| `/`   | Enter search mode                  |
| `:`         | Enter command mode                     |
| `Enter`     | Focus mode (full-screen detail)        |
| `Tab`       | Toggle focus between list / detail     |
| `Backspace` | Go back (pop child history)            |
| `Esc`       | Exit focus / clear search              |
| `?`         | Show help overlay                      |

**Search mode** (`/`) — type to fuzzy filter, `Enter` or `Esc` to return to normal mode. The filter is preserved.

**Command mode** (`:`) — type a command, `Enter` to execute. Supported commands: `:q`, `:quit`, `:h`, `:help`.

---

## Bulk Operations & Two-Way Sync

`ihj` includes a workflow for bulk-editing your backlog, ideal for mass refinements or LLM-assisted grooming:

1. **Extract:** `ihj extract` extracts your workspace as LLM friendly XML, dynamically injecting a JSON schema for the LLM to use in a YAML response and custom instructions directly to your clipboard.
2. **Edit:** Pipe the prompt into your LLM of choice (Gemini, Claude, ChatGPT) to refine your backlog, or run `ihj export` and edit the YAML file by hand instead.
3. **Apply:** Run `ihj apply manifest.yaml`. The CLI validates the schema and presents an interactive, rich diff for every changed issue.

During the apply process, you can interactively resolve conflicts:

- **Apply Changes:** Pushes your local YAML changes up to the provider.
- **Accept Remote:** Discards your local changes and overwrites your local file with the provider's current state.
- **Skip:** Bypasses the issue.
- **Abort Apply:** Safely halts the process.

---

## CLI Commands

All commands that operate on a single issue accept an `<id>` argument (e.g. `PROJ-123`).

```
ihj                          Launch TUI (default)
ihj tui [-w workspace] [-f filter]
                             Launch TUI for a specific workspace/filter
ihj jira demo                Launch TUI with synthetic data (no credentials needed)
ihj jira bootstrap <project> Scaffold config from a Jira project
ihj auth login <server>      Store an access token for a server
ihj auth logout <server>     Remove a stored token
ihj auth status              Show token status for all configured servers
ihj create                   Create a new issue (opens editor)
ihj edit <id>                Edit an issue (opens editor)
ihj comment <id>             Add a comment (opens editor)
ihj assign <id>              Assign issue to yourself
ihj transition <id>          Change issue status
ihj open <id>                Open in browser
ihj branch <id>              Copy git branch name to clipboard
ihj extract <id>             Extract issue context for LLM prompts
ihj export [-w workspace] [-f filter]
                             Export issue hierarchy as a YAML manifest
ihj apply <file> [-w workspace]
                             Review and apply YAML manifest changes
```

---

## Configuration

### File Locations

```
~/.config/ihj/config.yaml       Config file
~/.config/ihj/credentials.json  Fallback token storage (when keychain unavailable)
~/.local/state/ihj/              Cache directory
```

### Authentication

Tokens are resolved through a chain of backends, tried in order:

1. **OS Keychain** (macOS Keychain, Linux libsecret, Windows Credential Manager) — preferred
2. **Environment variables** — `IHJ_TOKEN_<ALIAS>` (alias uppercased, hyphens become underscores)
3. **Credentials file** — `~/.config/ihj/credentials.json` (0600 permissions)

Use `ihj auth login <server-alias>` to store credentials. You'll be prompted for your email and API token — the CLI handles encoding internally. The keychain is used when available; otherwise tokens fall back to the credentials file.

| Variable            | Description                                          |
| ------------------- | ---------------------------------------------------- |
| `IHJ_TOKEN_<ALIAS>` | Token for a server alias (e.g., `IHJ_TOKEN_MY_JIRA`) |
| `EDITOR`            | Fallback editor if not set in config                 |

### Config File

The easiest way to generate a config is `ihj jira bootstrap <PROJECT>`, which queries your Jira instance and outputs a ready-to-use YAML file. You can then hand-edit it.

#### Adding Additional Workspaces

Bootstrap always writes a full config to stdout, so you can't append it directly. To add a second workspace:

```bash
# 1. Run bootstrap for the new project (outputs to stdout, not your config file).
ihj jira bootstrap PROJ2

# 2. Copy the workspace block from the output into your existing config.yaml
#    under the `workspaces:` key. If the new workspace uses a different server,
#    also copy its entry under `servers:`.
```

If both workspaces share the same Jira instance, you only need the new workspace block — they'll reference the same server alias and token.

```yaml
theme: "default" # Glamour theme: auto, dark, light, pink, dracula, tokyo-night, ascii.
editor: "nvim" # Optional. Falls back to $EDITOR, then vim.
vim_mode: true # Optional. Enable vim-style modal key bindings.
default_workspace: "my-board"
cache_ttl: "10m" # Global cache TTL (default: 15m). Workspaces can override.

# Optional. Custom LLM guidance for the extract command.
# Overrides the built-in defaults. Can also be set per-workspace.
guidance: |
  Focus on acceptance criteria and edge cases.
  Preserve all existing issue keys exactly as provided.
  Do not invent new issue keys — if new issues are needed, omit the key field.

# Optional. Override default-mode key bindings by action name.
# Ignored when vim_mode is enabled. See "Custom Shortcuts" below.
shortcuts:
  extract: "ctrl+x"
  branch: "ctrl+b"

# Optional. Control the TUI layout.
layout:
  detail_height: 55    # Detail pane height as a percentage (20–80, default 55)
  show_help_bar: true  # Show key binding help bar (default true). Vim mode
                       # indicator is always visible regardless of this setting.

servers: # Server definitions with provider type + URL.
  my-jira:
    provider: "jira"
    url: "https://company.atlassian.net"
workspaces:
  my-board:
    server: "my-jira" # References a server alias above.
    name: "My Board"
    cache_ttl: "5m" # Optional. Overrides global cache_ttl for this workspace.

    # Provider-specific fields (Jira):
    board_id: 42
    board_type: "scrum" # "scrum", "kanban", or "simple"
    project_key: "PROJ"
    jql: <BASE BOARD JQL> # JQL query which is applied to all filters
    custom_fields:
      team: 15000
      epic_name: 10009
    filters: # Named JQL filter clauses (AND-ed with base jql).
      active: "sprint IN openSprints() AND sprint NOT IN futureSprints() AND (statusCategory != Done OR resolved >= -2w)"
      backlog: "sprint NOT IN openSprints() OR sprint IS EMPTY"
      me: "assignee = currentUser() AND statusCategory != Done"
      all: ""
    statuses: # Status workflow with sort order and display colors.
      - name: Backlog
        order: 10
        color: default
      - name: To Do
        order: 20
        color: cyan
      - name: In Progress
        order: 30
        color: blue
      - name: In Review
        order: 40
        color: magenta
      - name: Done
        order: 50
        color: green
    types: # Issue types with display metadata.
      - id: 1
        name: Epic
        order: 20
        color: magenta
        has_children: true
      - id: 3
        name: Task
        order: 30
        color: default
        has_children: true
        template: | # Optional Markdown template for new issues.
          ## Acceptance Criteria

          -
      - id: 5
        name: Sub-task
        order: 40
        color: white
        has_children: false
```

Multiple workspaces can share the same server (and therefore the same token):

```yaml
servers:
  company-jira:
    provider: jira
    url: https://company.atlassian.net
workspaces:
  engineering:
    server: company-jira
    name: Engineering
    project_key: ENG
    # ...
  platform:
    server: company-jira # Same server, same token.
    name: Platform
    project_key: PLAT
    # ...
```

### Jira: Board Types and Sprints

Bootstrap auto-detects your board type (scrum, kanban, or simple) and stores
it as `board_type` in the workspace config. This controls which filters are
generated and whether the `sprint` field is available.

#### Board-specific filters

**Scrum boards** get sprint-aware filters:

| Filter    | Description                                                                          |
| --------- | ------------------------------------------------------------------------------------ |
| `active`  | Items in the current active sprint (excludes future sprints), plus recently resolved |
| `backlog` | Items in future sprints or with no sprint assigned                                   |
| `all`     | No additional filtering                                                              |
| `me`      | Assigned to you, not done                                                            |

**Kanban / simple boards** get status-based filters (no sprint concepts):

| Filter   | Description                                                        |
| -------- | ------------------------------------------------------------------ |
| `active` | Items in visible board statuses, plus resolved in the last 2 weeks |
| `all`    | No additional filtering                                            |
| `me`     | Assigned to you, not done                                          |

You can customise these filters by editing the `filters:` section in your
config. Filter values are JQL fragments that get AND-ed with the base `jql:`
query.

#### Sprint assignment

On scrum boards, you can assign items to a sprint when creating, editing, or
applying a manifest. The `sprint` field accepts three values:

| Value    | Behaviour                                |
| -------- | ---------------------------------------- |
| `active` | Assign to the current active sprint      |
| `future` | Assign to the next upcoming sprint       |
| `none`   | Remove from any sprint (move to backlog) |

Omitting the field means "don't change the sprint" — this is different from
`none`, which explicitly removes the issue from its current sprint.

In the editor frontmatter:

```yaml
---
type: Story
priority: High
status: To Do
sprint: active
summary: "Implement login flow"
---
```

In a manifest (for `ihj apply`):

```yaml
items:
  - type: Epic
    summary: Authentication
    status: In Progress
    sprint: active
    children:
      - type: Story
        summary: OAuth integration
        status: To Do
        sprint: future
```

Sprint is an _action_ field, not a state field. Exported manifests never
include `sprint:` because the current sprint is context-dependent — re-applying
an export simply leaves sprint assignment unchanged. Use `sprint:` explicitly
when you want to move items between sprints.

If no matching sprint exists (e.g., no active sprint between sprints, or no
future sprints planned), you'll see a warning — the item is still created or
updated, only the sprint assignment fails.

Kanban and simple boards do not support the `sprint` field. Including it in a
manifest targeting a kanban workspace will fail schema validation.

### Editor Integration

When creating or editing issues, ihj opens your editor with a Markdown file containing YAML frontmatter:

```yaml
---
# yaml-language-server: $schema=/automatically/generated/schema.json
type: Task
priority: Medium
status: Backlog
summary: "Your issue title here"
---
Description in Markdown goes here.
```

If you use a vim-like editor, ihj automatically:

- Positions the cursor on the summary field (or description body)
- Enters insert mode
- Points at the JSON schema for YAML autocompletion (works with yaml-language-server in Neovim via [otter.nvim](https://github.com/jmbuhr/otter.nvim) - you'll need a custom autocmd)

Save and quit to submit. If validation fails or the API rejects the request, you'll be offered the choice to re-edit, copy to clipboard, or abort.

### Issue Templates

You can set a default description template per issue type in your config.
When creating a new issue of that type, the template pre-populates the
description body in the editor. When editing an existing issue that has no
description, the template is also used as a starting point. Templates are
written in Markdown.

```yaml
types:
  - id: 3
    name: Task
    order: 30
    color: default
    has_children: true
    template: |
      ## Acceptance Criteria

      -

      ## Notes

  - id: 7
    name: Bug
    order: 30
    color: red
    has_children: true
    template: |
      ## Steps to Reproduce

      1.

      ## Expected Behaviour

      ## Actual Behaviour
```

Templates are also included in the context output of `ihj extract`, so LLMs
are aware of your team's conventions when generating issue content.

### LLM Guidance

The `ihj extract` command wraps your issues in XML context that includes a
`<guidance>` section — instructions that steer the LLM's behaviour. The
built-in default guidance is:

```
- This is an interactive conversation. Ask clarifying questions before producing output.
- Ask the user if they have supporting materials to share — meeting transcripts,
  discovery documents, proposals, specs, or design docs can dramatically improve
  output quality.
- Once you understand the scope, produce a brief plan and wait for confirmation
  before generating the structured YAML output.
- Preserve all existing issue keys exactly as provided.
- Do not invent new issue keys — if new issues are needed, omit the key field.
```

You can override this globally or per-workspace using the `guidance` field (YAML
`|` multiline syntax works well here). Your custom guidance **replaces** the
default entirely, so include any default rules you still want to keep.

**Recommendations:** Always include the last two rules (`Preserve all existing
issue keys…` and `Do not invent new issue keys…`) in any custom guidance — they
prevent the LLM from silently renaming or fabricating issue keys, which would
break the `ihj apply` round-trip.

```yaml
# Global — applies to all workspaces unless overridden.
guidance: |
  Focus on acceptance criteria and edge cases.
  Preserve all existing issue keys exactly as provided.
  Do not invent new issue keys — if new issues are needed, omit the key field.

workspaces:
  eng:
    # Per-workspace override — replaces the global guidance for this workspace.
    guidance: |
      Write stories in user-story format ("As a…, I want…, so that…").
      Preserve all existing issue keys exactly as provided.
      Do not invent new issue keys — if new issues are needed, omit the key field.
```

### Custom Shortcuts

You can remap default-mode action keys using the `shortcuts` section in your
config. Shortcuts are ignored when `vim_mode` is enabled — vim mode key
bindings are opinionated and not user-configurable.

```yaml
shortcuts:
  extract: "ctrl+x"
  branch: "ctrl+b"
```

Available action names:

| Action       | Default key | Description                        |
| ------------ | ----------- | ---------------------------------- |
| `refresh`    | `alt+r`     | Refresh data                       |
| `filter`     | `alt+f`     | Switch filter                      |
| `workspace`  | `alt+w`     | Switch workspace                   |
| `edit`       | `alt+e`     | Edit selected issue                |
| `new`        | `ctrl+n`    | Create new issue                   |
| `transition` | `alt+t`     | Transition (change status)         |
| `assign`     | `alt+a`     | Assign to yourself                 |
| `comment`    | `alt+c`     | Add comment                        |
| `open`       | `alt+o`     | Open in browser                    |
| `branch`     | `alt+n`     | Copy git branch name to clipboard  |
| `extract`    | `alt+x`     | Extract issue context for LLM      |

Shortcut values must include a modifier prefix — bare characters would conflict
with search input. Supported modifiers: `alt+`, `ctrl+`, `super+`, `hyper+`
(e.g., `alt+x`, `ctrl+b`). `shift+` alone is not accepted. Key names follow
[Bubble Tea conventions](https://github.com/charmbracelet/ultraviolet/blob/main/key_table.go).

Collisions with reserved bindings (navigation, quit, help) or other shortcuts
are rejected at config load.

### Caching

Issue data is cached per workspace and filter in `~/.local/state/ihj/`. Cache TTL is 15 minutes. When switching filters:

- **Fresh cache** -- loaded instantly, no network call.
- **Stale cache** -- shown immediately while a background refresh runs.
- **No cache** -- loading indicator shown until the API responds.

Use `Alt+R` (or `r` in vim mode) to force a refresh at any time.

---

## Architecture

The codebase follows a layered, provider-agnostic design. Core business logic
speaks only in terms of `WorkItem` and `Provider` interfaces -- the Jira
provider is a self-contained vertical slice that implements those interfaces.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full breakdown of packages,
dependency graph, design patterns, and how to add a new provider.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, testing
conventions, and how to submit changes.

## Acknowledgements

- [Charm](https://charm.sh/) — ihj is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Glamour](https://github.com/charmbracelet/glamour), [Huh](https://github.com/charmbracelet/huh), and [VHS](https://github.com/charmbracelet/vhs). The Charm team's work on terminal tooling made this project possible.
- [jira-cli](https://github.com/ankitpokhrel/jira-cli) — the original inspiration. Early versions of ihj were bash scripts wrapping jira-cli with fzf for fuzzy filtering.
- [fzf](https://github.com/junegunn/fzf) — the fuzzy finder that inspired the TUI's search-driven navigation.
