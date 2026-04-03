# Bulk Operations

ihj supports a round-trip workflow for bulk-editing your backlog. This is designed for mass refinements, sprint planning, and LLM-assisted grooming.

## Workflow

1. **Extract:** `ihj extract` copies your workspace as structured XML to the clipboard, including a JSON schema and custom guidance for the LLM.

2. **Refine:** Paste into your LLM of choice (Claude, Gemini, ChatGPT). The schema and guidance steer the LLM to produce valid YAML output. Alternatively, run `ihj export` and edit the YAML file by hand.

3. **Apply:** Run `ihj apply manifest.yaml`. The CLI validates the schema and presents an interactive diff for every changed issue.

## Apply Options

During apply, each changed issue presents four choices:

- **Apply Changes** — push the changes for this issue to the provider.
- **Accept Remote** — discard local changes for the current issue and overwrite with the provider's current state.
- **Skip** — bypass this issue.
- **Abort Apply** — halt the entire process.

## LLM Guidance

The `ihj extract` command includes a `<guidance>` section in its output. The built-in defaults instruct the LLM to:

- Ask clarifying questions before producing output
- Request supporting materials (meeting notes, specs, design docs)
- Produce a brief plan and wait for confirmation before generating YAML
- Preserve existing issue keys and not invent new ones

You can override this globally or per-workspace:

```yaml
# Global guidance — applies to all workspaces unless overridden.
guidance: |
  Focus on acceptance criteria and edge cases.
  Preserve all existing issue keys exactly as provided.
  Do not invent new issue keys — if new issues are needed, omit the key field.

workspaces:
  eng:
    # Per-workspace override — replaces global guidance for this workspace.
    guidance: |
      Write stories in user-story format ("As a..., I want..., so that...").
      Preserve all existing issue keys exactly as provided.
      Do not invent new issue keys — if new issues are needed, omit the key field.
```

Always include the key preservation rules in custom guidance — without them, LLMs tend to rename or fabricate issue keys, which breaks the apply round-trip.
