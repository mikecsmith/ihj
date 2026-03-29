<script lang="ts">
  import { searchQuery, filtered, cursor } from "../lib/stores/ui";
  import { items } from "../lib/stores/items";

  let inputEl: HTMLInputElement | undefined = $state();

  function oninput(e: Event) {
    searchQuery.set((e.target as HTMLInputElement).value);
    cursor.set(0);
  }

  export function focus() {
    inputEl?.focus();
  }
</script>

<div class="search-bar">
  <span class="search-bar__count">{$filtered.length}/{$items.length}</span>
  <span class="search-bar__chevron">&rsaquo;</span>
  <input
    bind:this={inputEl}
    class="search-bar__input"
    type="text"
    placeholder="Type to filter..."
    value={$searchQuery}
    {oninput}
  />
</div>

<style>
  .search-bar { display: flex; align-items: center; padding: 3px 16px; background: var(--bg-secondary); border-bottom: 1px solid var(--border-default); gap: 8px; }
  .search-bar__count { font-size: 11px; color: var(--accent-cyan); font-variant-numeric: tabular-nums; }
  .search-bar__chevron { color: var(--text-muted); }
  .search-bar__input { flex: 1; background: transparent; border: none; outline: none; color: var(--text-primary); font-family: var(--font-mono); font-size: 13px; caret-color: var(--accent-blue); }
  .search-bar__input::placeholder { color: var(--text-muted); }
</style>
