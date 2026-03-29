<script lang="ts">
  import type { FieldDiff } from "../../lib/types";
  import SelectPopup from "./SelectPopup.svelte";

  interface Props {
    changes: FieldDiff[];
    options: string[];
    cursor: number;
    onSelect: (index: number) => void;
  }
  let { changes, options, cursor, onSelect }: Props = $props();
</script>

<div class="diff-table">
  {#each changes as change}
    <div class="diff-row">
      <span class="diff-field">{change.field}</span>
      <span class="diff-old">{change.old}</span>
      <span class="diff-arrow">&rarr;</span>
      <span class="diff-new">{change.new}</span>
    </div>
  {/each}
</div>
<SelectPopup {options} {cursor} {onSelect} />

<style>
  .diff-table { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; font-family: var(--font-mono); font-size: 12px; max-height: 60vh; overflow-y: auto; }
  .diff-row { display: grid; grid-template-columns: 100px 1fr auto 1fr; gap: 8px; align-items: baseline; padding: 6px 8px; border-radius: var(--radius-sm); background: var(--bg-tertiary); }
  .diff-field { font-weight: 600; color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .diff-old { color: #f7768e; white-space: pre-wrap; word-break: break-word; }
  .diff-arrow { color: var(--text-hint); flex-shrink: 0; }
  .diff-new { color: #9ece6a; white-space: pre-wrap; word-break: break-word; }
</style>
