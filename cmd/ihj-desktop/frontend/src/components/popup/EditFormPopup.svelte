<script lang="ts">
  import { untrack } from "svelte";
  import type { FieldDef } from "../../lib/types";

  interface Props {
    metadata: Record<string, string>;
    description: string;
    fields: FieldDef[];
    statuses: string[];
    types: string[];
    onSave: (result: { metadata: Record<string, string>; description: string }) => void;
    onCancel: () => void;
  }
  let { metadata, description, fields, statuses, types, onSave, onCancel }: Props = $props();

  // One-time snapshot into local mutable state — untrack suppresses the
  // state_referenced_locally warning since we intentionally want only the initial value.
  let form = $state<Record<string, string>>(untrack(() => ({ ...metadata })));
  let desc = $state(untrack(() => description));

  // Built-in fields rendered at the top in fixed order.
  const builtinKeys = ["type", "summary", "status", "priority", "parent", "sprint"];

  // Field definitions keyed for lookup.
  let fieldMap = $derived(Object.fromEntries(fields.map((f: FieldDef) => [f.key, f])));

  // Extra provider fields (from fieldDefs, not built-in, visibility != readonly).
  let extraFields = $derived(
    fields.filter((f: FieldDef) => !builtinKeys.includes(f.key) && f.visibility !== "readonly"),
  );

  function enumOptions(key: string): string[] | null {
    if (key === "status") return statuses;
    if (key === "type") return types;
    const def = fieldMap[key];
    if (def?.type === "enum" && def.enum.length > 0) return def.enum;
    return null;
  }

  function handleSubmit() {
    onSave({ metadata: { ...form }, description: desc });
  }

  function onKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.code === "KeyS") {
      e.preventDefault();
      handleSubmit();
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<div class="form">
  <!-- Summary (full width) -->
  <div class="form__row form__row--full">
    <label class="form__label" for="field-summary">Summary</label>
    <!-- svelte-ignore a11y_autofocus -->
    <input
      id="field-summary"
      class="form__input"
      type="text"
      bind:value={form["summary"]}
      placeholder="Issue summary"
      autofocus
    />
  </div>

  <!-- Grid of built-in fields (except summary) -->
  <div class="form__grid">
    {#if types.length > 0}
      <div class="form__row">
        <label class="form__label" for="field-type">Type</label>
        <select id="field-type" class="form__select" bind:value={form["type"]}>
          {#each types as t}
            <option value={t}>{t}</option>
          {/each}
        </select>
      </div>
    {/if}

    <div class="form__row">
      <label class="form__label" for="field-status">Status</label>
      <select id="field-status" class="form__select" bind:value={form["status"]}>
        {#each statuses as s}
          <option value={s}>{s}</option>
        {/each}
      </select>
    </div>

    {#if enumOptions("priority")}
      <div class="form__row">
        <label class="form__label" for="field-priority">Priority</label>
        <select id="field-priority" class="form__select" bind:value={form["priority"]}>
          {#each enumOptions("priority")! as p}
            <option value={p}>{p}</option>
          {/each}
        </select>
      </div>
    {:else}
      <div class="form__row">
        <label class="form__label" for="field-priority">Priority</label>
        <input id="field-priority" class="form__input" type="text" bind:value={form["priority"]} />
      </div>
    {/if}

    {#if form["parent"] !== undefined}
      <div class="form__row">
        <label class="form__label" for="field-parent">Parent</label>
        <input id="field-parent" class="form__input" type="text" bind:value={form["parent"]} />
      </div>
    {/if}

    {#if form["sprint"] !== undefined}
      <div class="form__row">
        <label class="form__label" for="field-sprint">Sprint</label>
        <select id="field-sprint" class="form__select" bind:value={form["sprint"]}>
          <option value="">No</option>
          <option value="true">Yes</option>
        </select>
      </div>
    {/if}

    <!-- Extra provider fields -->
    {#each extraFields as field}
      <div class="form__row">
        <label class="form__label" for="field-{field.key}">{field.label}</label>
        {#if field.type === "enum" && field.enum.length > 0}
          <select id="field-{field.key}" class="form__select" bind:value={form[field.key]}>
            <option value="">--</option>
            {#each field.enum as opt}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        {:else}
          <input id="field-{field.key}" class="form__input" type="text" bind:value={form[field.key]} />
        {/if}
      </div>
    {/each}
  </div>

  <!-- Description (full width) -->
  <div class="form__row form__row--full">
    <label class="form__label" for="field-description">Description</label>
    <textarea id="field-description" class="form__textarea" bind:value={desc} placeholder="Markdown description"></textarea>
  </div>

  <!-- Actions -->
  <div class="form__actions">
    <button class="form__btn form__btn--secondary" onclick={onCancel}>Cancel</button>
    <button class="form__btn form__btn--primary" onclick={handleSubmit}>Save</button>
    <span class="form__hint">Ctrl/Cmd+S to save</span>
  </div>
</div>

<style>
  .form { display: flex; flex-direction: column; gap: 12px; }
  .form__row { display: flex; flex-direction: column; gap: 4px; }
  .form__row--full { width: 100%; }
  .form__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  .form__label { font-size: 11px; font-weight: 600; color: var(--text-hint); text-transform: uppercase; letter-spacing: 0.5px; }
  .form__input, .form__select { background: var(--bg-input); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 7px 10px; color: var(--text-primary); font-family: var(--font-mono); font-size: 12px; outline: none; }
  .form__input:focus, .form__select:focus, .form__textarea:focus { border-color: var(--accent-blue); }
  .form__select { cursor: pointer; }
  .form__textarea { background: var(--bg-input); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 8px 10px; color: var(--text-primary); font-family: var(--font-mono); font-size: 12px; outline: none; resize: vertical; min-height: 120px; }
  .form__actions { display: flex; align-items: center; gap: 8px; padding-top: 4px; }
  .form__btn { padding: 6px 16px; border-radius: var(--radius-sm); font-size: 12px; font-weight: 600; cursor: pointer; border: 1px solid var(--border-default); }
  .form__btn--primary { background: var(--accent-blue); color: var(--bg-primary); border-color: var(--accent-blue); }
  .form__btn--primary:hover { opacity: 0.9; }
  .form__btn--secondary { background: var(--bg-tertiary); color: var(--text-secondary); }
  .form__btn--secondary:hover { background: var(--bg-hover); }
  .form__hint { font-size: 11px; color: var(--text-hint); margin-left: auto; }
</style>
