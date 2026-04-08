# Bulk Operations

ihj supports a round-trip workflow for bulk-editing your backlog. This is designed for mass refinements, sprint planning, and LLM-assisted grooming.

## Workflow

1. **Extract:** `ihj extract` copies your workspace as structured XML to the clipboard. Depending on the extract mode, this includes a JSON schema, type templates, and LLM guidance.

2. **Refine:** Paste into your LLM of choice (Claude, Gemini, ChatGPT). The schema and guidance steer the LLM to produce valid YAML output. Alternatively, run `ihj export` and edit the YAML file by hand.

3. **Apply:** Run `ihj apply manifest.yaml`. The CLI validates the schema and presents an interactive diff for every changed issue.

## Informational Fields

Full exports (`ihj export --full`) include some fields with an underscore prefix (e.g. `_sprint`, `_created`). These are **informational** — they provide context but are silently ignored on import. This applies to:

- **Action fields** like `sprint`, where the export shows the current state (e.g. "Sprint 3") but the import expects an action (e.g. "active").
- **Immutable fields** like `created` and `updated`, which are set by the provider and cannot be changed.

To act on an action field, use the unprefixed key (e.g. `sprint: active`).

## Apply Options

During apply, each changed issue presents four choices:

- **Apply Changes** — push the changes for this issue to the provider.
- **Accept Remote** — discard local changes for the current issue and overwrite with the provider's current state.
- **Skip** — bypass this issue.
- **Abort Apply** — halt the entire process.

## Extract Modes

When you run `ihj extract`, you choose an extract mode (or pass `--preset` on the CLI):

| Mode | Guidance | Output format | Templates | Use case |
|------|----------|---------------|-----------|----------|
| **Refine** | Restructure and break down | Yes | Yes | Sprint planning, backlog grooming, breaking epics into stories |
| **Triage** | Assess and categorise | Yes | Yes | Prioritisation, completeness review, labelling |
| **Bare** | None | No | No | Feed raw issue context into any LLM prompt |

**Refine** and **triage** include a `<guidance>` section that steers the LLM toward the right kind of output. **Bare** includes only the instruction and issues — useful when you want full control over the prompt.

## Custom Guidance

Each guided preset (refine, triage) has built-in guidance that works well out of the box. You can override it per-workspace:

```yaml
workspaces:
  eng:
    extract:
      presets:
        refine:
          guidance: |
            Write stories in user-story format ("As a..., I want..., so that...").
            Preserve all existing issue keys exactly as provided.
            Do not invent new issue keys — if new issues are needed, omit the key field.
        triage:
          guidance: |
            Focus on acceptance criteria completeness.
            Flag stories missing edge cases or error handling.
```

Only `refine` and `triage` accept custom guidance — `bare` has no guidance by design.

Always include the key preservation rules in custom guidance — without them, LLMs tend to rename or fabricate issue keys, which breaks the apply round-trip.

## Markdown in Descriptions

Issue descriptions are converted between the provider's native format and Markdown. Most formatting round-trips cleanly, including empty list items used as placeholders (e.g. `- `). One edge case: a list item whose only content is a nested list (e.g. `- -`) may not survive a round-trip exactly. Use `- ` (dash-space) for empty placeholders instead.
