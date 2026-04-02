# Contributing to ihj

## Prerequisites

- **Go 1.26+**
- **[mise](https://mise.jdx.dev/)** — manages Go, golangci-lint, and lefthook versions
- A terminal with a **Nerd Font** installed (the TUI uses Nerd Font icons)

## Getting Started

```bash
git clone https://github.com/mikecsmith/ihj.git
cd ihj
mise install
mise exec -- go build ./cmd/ihj
mise exec -- go run ./cmd/ihj demo   # verify it works — no Jira needed
```

## Running Tests

```bash
mise exec -- go test ./...
```

During development, run only the package you changed for faster feedback:

```bash
mise exec -- go test ./internal/tui/...
```

## Project Structure

See [ARCHITECTURE.md](ARCHITECTURE.md) for a detailed breakdown of packages,
dependencies, and design patterns.

## Code Conventions

### Testing

- **Black-box tests** (`package foo_test`) by default.
- Use the `_internal_test.go` suffix for tests that need access to unexported
  functions.
- Use `httptest` servers for provider integration tests.
- Prefer table-driven tests.

#### TUI tests

The TUI has three test layers with distinct roles:

1. **Journey tests** (`journey_test.go`) — end-to-end behaviour tests using
   `teatest.TestModel`. Assert on **UIEvents** emitted by `BubbleTeaUI.Emit()`,
   not on terminal output text. Use `waitForEvent(t, ui, "kind")` and check
   `evt.Data` for specific values.
2. **Golden tests** (`golden_test.go`) — snapshot tests of `View()` output.
   Run with `-update-golden` to regenerate after intentional visual changes.
3. **Vim unit tests** (`vim_test.go`) — white-box state machine tests for
   vim modal input.

Use `testutil.NewTestHarness(t, ui)` to construct test models — it wires up
workspace, provider, runtime, session, and factory consistently. Extend the
harness if it doesn't cover your case; don't inline the construction.

### Style

- Follow standard Go conventions and `gofmt`.
- Comments must follow
  [GoDoc conventions](https://go.dev/blog/godoc) — start with the name of the
  thing being documented.
- No section-separator banners in comments (e.g., `// --- Section ---`).
- Nerd Font icons appear in TUI string literals. Take care not to strip them
  during edits — they look like blank spaces but are multi-byte Unicode
  codepoints.

### Dependencies

| Area | Library |
|------|---------|
| TUI framework | `charm.land/bubbletea/v2` |
| TUI components | `charm.land/bubbles/v2` |
| Terminal styling | `charm.land/lipgloss/v2` |
| Markdown rendering | `charm.land/glamour/v2` |
| CLI framework | `github.com/spf13/cobra` |
| YAML | `github.com/goccy/go-yaml` |
| OS keychain | `github.com/zalando/go-keyring` |

All tools are managed via mise. Use `mise exec -- <command>` to run them.

## Submitting Changes

1. Fork the repository and create a feature branch.
2. Make your changes.
3. Ensure the build compiles cleanly:
   ```bash
   mise exec -- go build ./...
   ```
4. Ensure all tests pass:
   ```bash
   mise exec -- go test ./...
   ```
5. Keep commits focused — one logical change per commit.
6. Open a pull request with a clear description of what changed and why.
