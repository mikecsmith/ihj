<script lang="ts">
  interface Props {
    options: string[];
    cursor: number;
    onSelect: (index: number) => void;
  }
  let { options, cursor, onSelect }: Props = $props();
</script>

{#each options as opt, i}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="option" class:option--selected={i === cursor} onclick={() => onSelect(i)}>
    <span class="option__num">{i < 9 ? i + 1 : ""}</span>
    <span class="option__marker">{i === cursor ? "\u25B8" : " "}</span>
    <span>{opt}</span>
  </div>
{/each}
<div class="hint">&uarr;&darr; Navigate &middot; Enter Confirm &middot; Esc Cancel</div>

<style>
  .option { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: var(--radius-sm); cursor: pointer; font-size: 12px; transition: background var(--transition-fast); }
  .option:hover, .option--selected { background: var(--bg-active); }
  .option--selected { font-weight: 600; }
  .option__num { color: var(--text-hint); font-size: 10px; width: 14px; text-align: right; }
  .option__marker { color: var(--accent-cyan); font-size: 11px; width: 12px; }
  .hint { font-size: 11px; color: var(--text-hint); margin-top: 10px; }
</style>
