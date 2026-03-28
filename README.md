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
# 1. Set your Jira API credentials.
#    Generate a token at: https://id.atlassian.com/manage-profile/security/api-tokens
#    Encode as base64(email:token):
export JIRA_BASIC_TOKEN=$(echo -n "you@company.com:your-api-token" | base64)

# 2. Bootstrap a workspace config from your Jira project.
ihj jira bootstrap PROJ > ~/.config/ihj/config.yaml

# 3. Launch the TUI.
ihj
```

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

### File Location

```
~/.config/ihj/config.yaml    Config file
~/.local/state/ihj/          Cache directory
```

### Environment Variables

| Variable           | Required          | Description                              |
| ------------------ | ----------------- | ---------------------------------------- |
| `JIRA_BASIC_TOKEN` | Yes (except demo) | `base64(email:api_token)` for Jira Cloud |
| `EDITOR`           | No                | Fallback editor if not set in config     |

### Config File

The easiest way to generate a config is `ihj jira bootstrap <PROJECT>`, which queries your Jira instance and outputs a ready-to-use YAML file. You can then hand-edit it.

```yaml
theme: "default"             # Glamour theme for content rendering.
editor: "nvim"               # Optional. Falls back to $EDITOR, then vim.
default_workspace: "my-board"

workspaces:
  my-board:
    provider: "jira"         # Provider discriminator.
    name: "My Board"

    # Provider-specific fields (Jira):
    server: "https://company.atlassian.net"
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
