# ihj — an fzf-inspired issue tracker

An fzf-inspired issue tracker with pluggable providers, vim mode, and LLM-assisted backlog refinement.

- **Instant load.** Aggressive caching with background refresh — your board is always ready.
- **Keyboard-driven.** Fuzzy search, single-key actions, and optional vim mode.
- **Multi-workspace.** Switch between projects and Jira instances in a single keypress.
- **LLM-assisted backlog refinement.** Extract your backlog as structured context, refine it with an LLM, and review every change through an interactive diff before anything is written.
- **Provider-agnostic.** Built for Jira today, designed for GitHub Issues, Linear, and others.

> **Early stage software.** Under active development. `ihj` makes write calls to your Jira instance (create, edit, transition, assign, comment). Use at your own risk.

---

## Demo

### General Usage

<video src="https://github.com/user-attachments/assets/0b63889f-c75a-4670-939d-203f8ccbde94" width="720"></video>

### Bulk Apply

<video src="https://github.com/user-attachments/assets/652cb941-b43d-4bbf-893f-39a12ae5448a" width="720"></video>

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

## Quick Start

```bash
# Bootstrap a workspace config from your Jira project.
# You'll be prompted for server URL, email, and API token.
ihj jira bootstrap PROJ > ~/.config/ihj/config.yaml

# Launch the TUI.
ihj
```

To try it without a Jira connection: `ihj jira demo`

## Authentication

Tokens are resolved through a chain of backends, tried in order:

1. **OS Keychain** (macOS Keychain, Linux libsecret, Windows Credential Manager) — preferred
2. **Environment variables** — `IHJ_TOKEN_<ALIAS>` (alias uppercased, hyphens become underscores)
3. **Credentials file** — `~/.config/ihj/credentials.json` (0600 permissions)

Use `ihj auth login <server-alias>` to store credentials. You'll be prompted for your email and API token. The keychain is used when available; otherwise tokens fall back to the credentials file.

---

## What It Does

### Terminal UI

A split-pane interface with fuzzy search, issue detail with child navigation, and full keyboard control. Enter expands the detail pane to full screen. Tab toggles focus between panes. All actions are available from any view state.

Default mode uses modifier keys (`Alt+E` to edit, `Alt+T` to transition). Vim mode (`vim_mode: true`) uses single characters in a modal interface (`e`, `t`, `/` for search, `:` for commands).

See [TUI documentation](docs/tui.md) for key bindings, vim mode, and layout options.

### Bulk Operations

Extract your backlog as structured XML context with `ihj extract`, feed it to an LLM for refinement, and apply the resulting YAML with `ihj apply`. Every field change on every issue gets its own interactive diff — accept, reject, take the remote version, or abort. Nothing is written without your approval.

See [Bulk operations guide](docs/bulk-operations.md) for the full workflow.

### CLI Commands

Every TUI action is also available as a standalone command. All issue commands accept an `<id>` argument (e.g. `PROJ-123`).

```
ihj                          Launch TUI (default)
ihj tui [-w workspace] [-f filter]
                             Launch TUI for a specific workspace/filter
ihj jira demo                Launch TUI with synthetic data (no credentials needed)
ihj jira bootstrap <project> Scaffold config from a Jira project
ihj auth login <server>      Store an access token for a server
ihj auth logout <server>     Remove a stored token
ihj auth status              Show token status for all configured servers
ihj create [-s key=value]    Create a new issue (opens editor)
ihj edit <id> [-s key=value] Edit an issue (opens editor)
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

Commands that open your editor use Markdown with YAML frontmatter and JSON Schema for autocompletion. See [Editor integration](docs/editor.md) for details and Neovim setup.

---

## Configuration

Config lives at `~/.config/ihj/config.yaml`. The easiest way to generate one is `ihj jira bootstrap <PROJECT>`.

```yaml
editor: "nvim"
vim_mode: true
default_workspace: "my-board"

servers:
  my-jira:
    provider: "jira"
    url: "https://company.atlassian.net"

workspaces:
  my-board:
    server: "my-jira"
    project_key: "PROJ"
    board_id: 42
    jql: 'project = "{project_key}"'
    # filters, statuses, types, custom_fields...
```

See [Configuration reference](docs/config.md) for shortcuts, caching, layout, and multi-workspace setup.

Full working examples: [`jira-scrum.yaml`](examples/jira-scrum.yaml), [`jira-kanban.yaml`](examples/jira-kanban.yaml)

### Provider Setup

- [Jira](docs/jira.md) — board types, sprints, filters, JQL variables, custom fields, templates

---

## Project

- [Architecture](ARCHITECTURE.md) — package structure, dependency graph, design patterns
- [Contributing](CONTRIBUTING.md) — setup, testing conventions, how to submit changes
- [Changelog](CHANGELOG.md)

## Acknowledgements

- [Charm](https://charm.sh/) — ihj is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Glamour](https://github.com/charmbracelet/glamour), [Huh](https://github.com/charmbracelet/huh), and [VHS](https://github.com/charmbracelet/vhs).
- [jira-cli](https://github.com/ankitpokhrel/jira-cli) — the original inspiration. Early versions of ihj were bash scripts wrapping jira-cli with fzf.
- [fzf](https://github.com/junegunn/fzf) — inspired the search-driven navigation model.
