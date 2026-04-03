# Configuration

## File Locations

```
~/.config/ihj/config.yaml       Config file
~/.config/ihj/credentials.json  Fallback token storage (when keychain unavailable)
~/.local/state/ihj/              Cache directory
```

## General Options

```yaml
theme: "default" # Glamour theme: auto, dark, light, pink, dracula, tokyo-night, ascii
editor: "nvim" # Falls back to $EDITOR, then vim
vim_mode: true # Enable vim-style modal key bindings
default_workspace: "eng" # Workspace to open on launch
cache_ttl: "10m" # Global cache TTL (default 15m). Workspaces can override.
```

## Layout

```yaml
layout:
  detail_height: 55 # Detail pane height as a percentage (20-80, default 55)
  show_help_bar: true # Show key binding help bar (default true)
```

See [TUI documentation](tui.md) for details on focus mode, pane focus, and vim mode indicator behaviour.

## Shortcuts

Default-mode action keys can be remapped. Ignored when `vim_mode` is enabled. Keys must include a modifier (`alt`, `ctrl`, `super`, or `hyper`) to avoid conflicting with search input.

Available actions: `refresh`, `filter`, `assign`, `transition`, `open`, `edit`, `comment`, `branch`, `extract`, `new`, `workspace`.

```yaml
shortcuts:
  extract: "ctrl+x"
  branch: "ctrl+b"
```

Navigation keys (`Up`, `Down`, `Enter`, `Tab`, `Esc`, `PgUp`, `PgDn`, `Home`, `End`) and `Help` (`Alt+/`) are reserved and cannot be remapped. See [TUI documentation](tui.md) for default key bindings.

## Caching

Issue data is cached per workspace and filter in `~/.local/state/ihj/`. Default TTL is 15 minutes. When switching filters:

- **Fresh cache** — loaded instantly, no network call.
- **Stale cache** — shown immediately while a background refresh runs.
- **No cache** — loading indicator shown until the API responds.

Use `Alt+R` (or `r` in vim mode) to force a refresh at any time.

Per-workspace TTL override:

```yaml
workspaces:
  eng:
    cache_ttl: "5m"
```

## Multi-Workspace

Multiple workspaces can share the same server (and token):

```yaml
servers:
  company-jira:
    provider: jira
    url: https://company.atlassian.net

workspaces:
  engineering:
    server: company-jira
    project_key: ENG
    # jql, filters, statuses, types...
  platform:
    server: company-jira
    project_key: PLAT
    # jql, filters, statuses, types...
```

See the full workspace examples: [`jira-scrum.yaml`](../examples/jira-scrum.yaml), [`jira-kanban.yaml`](../examples/jira-kanban.yaml)

Switch between workspaces in the TUI with `Alt+W` (or `w` in vim mode).

## JQL Variables

The `jql:` and `filters:` values in workspace config are templates. Placeholders written as `{name}` are expanded at query time, so you never need to hardcode project IDs or custom field numbers.

```yaml
workspaces:
  eng:
    project_key: "ENG"
    team_uuid: "abc-123-def"
    custom_fields:
      team: 15000
    jql: 'project = "{project_key}" AND {team} = "{team_uuid}"'
    filters:
      me: 'assignee = currentUser() AND statusCategory != Done'
```

The `jql:` above expands to: `project = "ENG" AND cf[15000] = "abc-123-def"`

Workspace metadata (`{project_key}`, `{team_uuid}`, `{id}`, `{name}`, `{slug}`) and custom field names (`{team}` → `cf[15000]`) are available as variables. Undefined placeholders produce a clear error at config load time.

`ihj jira bootstrap` auto-detects these values and generates templates with placeholders already wired up.

See [Jira provider docs](jira.md#jql-variables) for the full variable reference.

## Adding a Workspace

`ihj jira bootstrap` writes a full config to stdout. To add a second workspace to an existing config:

```bash
# Run bootstrap for the new project.
ihj jira bootstrap PROJ2

# Copy the workspace block from the output into your config.yaml
# under the workspaces: key. If it uses a different server,
# also copy the server entry.
```

## Provider Configuration

- [Jira](jira.md) — board types, sprints, filters, custom fields, templates

## LLM Guidance

See [Bulk operations](bulk-operations.md#llm-guidance) for configuring the `guidance` field.
