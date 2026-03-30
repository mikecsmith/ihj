# ihj — Instant High-speed Jira 😉

A terminal-native Jira client with a fuzzy-filterable TUI and headless CLI commands.
Built on a provider-agnostic architecture that can be extended to other backends
(GitHub Issues, Linear, Trello, etc.).

> **Alpha Software** — This tool is under active development. It can and will make
> **write calls** to your Jira instance (create issues, update fields, post comments,
> change statuses, assign users). Use at your own risk. There are **no warranties**
> of any kind. Always review what you're doing before confirming an action.

---

## Quick Start

```bash
# 1. Bootstrap a workspace config from your Jira project.
#    You'll be prompted for your server URL and API token.
#    The token is stored securely (OS keychain when available).
#    Generate a token at: https://id.atlassian.com/manage-profile/security/api-tokens
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

| Key                     | Action                           |
| ----------------------- | -------------------------------- |
| `Up` / `Ctrl+K`         | Move cursor up                   |
| `Down` / `Ctrl+J`       | Move cursor down                 |
| `Home`                  | Jump to first issue              |
| `End`                   | Jump to last issue               |
| `PgUp` / `PgDown`       | Page up / down                   |
| `Shift+Up` / `Ctrl+U`   | Scroll preview up                |
| `Shift+Down` / `Ctrl+D` | Scroll preview down              |
| `Enter`                 | Navigate into first child issue  |
| `1`–`9`                 | Navigate to nth child in preview |
| `Esc`                   | Go back / clear search / quit    |
| `Ctrl+C`                | Quit                             |

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
| `Alt+R`  | Refresh data                       |

### Search

Type any character to start fuzzy filtering. The search matches across issue key, summary, assignee, status, and type. Press `Esc` to clear.

---

## Bulk Operations & Two-Way Sync

`ihj` includes a workflow for bulk-editing your backlog, ideal for mass refinements or LLM-assisted grooming:

1. **Export:** `ihj export` extracts your workspace into a clean YAML manifest, dynamically injecting a JSON schema and LLM instructions directly to your clipboard.
2. **Edit:** Paste the prompt into your LLM of choice (Gemini, Claude, ChatGPT) to generate sweeping backlog changes, or edit the YAML file by hand.
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
ihj apply <file>             Review and apply YAML manifest changes
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

Use `ihj auth login <server-alias>` to store tokens. The keychain is used when available; otherwise tokens fall back to the credentials file.

| Variable              | Description                                             |
| --------------------- | ------------------------------------------------------- |
| `IHJ_TOKEN_<ALIAS>`   | Token for a server alias (e.g., `IHJ_TOKEN_MY_JIRA`)   |
| `EDITOR`              | Fallback editor if not set in config                    |

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
theme: "default"             # Glamour theme for content rendering.
editor: "nvim"               # Optional. Falls back to $EDITOR, then vim.
default_workspace: "my-board"

servers:                     # Server definitions with provider type + URL.
  my-jira:
    provider: "jira"
    url: "https://company.atlassian.net"

workspaces:
  my-board:
    server: "my-jira"        # References a server alias above.
    name: "My Board"

    # Provider-specific fields (Jira):
    board_id: 42
    project_key: "PROJ"
    custom_fields:
      team: 15000
      epic_name: 10009

    statuses:                # Ordered status workflow.
      - Backlog
      - To Do
      - In Progress
      - In Review
      - Done

    filters:                 # Named filter clauses.
      active: "statusCategory != Done OR status CHANGED AFTER -2w"
      mine: "assignee = currentUser()"
      all: ""

    types:                   # Issue types with display metadata.
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
        template: |          # Optional Markdown template for new issues.
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
    server: company-jira     # Same server, same token.
    name: Platform
    project_key: PLAT
    # ...
```

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
- Points at the JSON schema for YAML autocompletion (works with yaml-language-server in neovim)

Save and quit to submit. If validation fails or the API rejects the request, you'll be offered the choice to re-edit, copy to clipboard, or abort.

### Caching

Issue data is cached per workspace and filter in `~/.local/state/ihj/`. Cache TTL is 15 minutes. When switching filters:

- **Fresh cache** -- loaded instantly, no network call.
- **Stale cache** -- shown immediately while a background refresh runs.
- **No cache** -- loading indicator shown until the API responds.

Use `Alt+R` to force a refresh at any time.

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
