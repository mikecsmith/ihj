<script lang="ts">
  import { workspace, filter, fetchedAt } from "../lib/stores/items";
  import { toggleTheme } from "../lib/stores/theme";

  let cacheAge = $state("0s");

  $effect(() => {
    const interval = setInterval(() => {
      const at = $fetchedAt;
      if (!at) { cacheAge = "0s"; return; }
      const s = Math.floor((Date.now() - at.getTime()) / 1000);
      const m = Math.floor(s / 60);
      cacheAge = m > 0 ? `${m}m${s % 60}s` : `${s}s`;
    }, 1000);
    return () => clearInterval(interval);
  });
</script>

<div class="title-bar">
  <div class="title-bar__left">
    <span class="title-bar__logo">ihj</span>
    <span class="title-bar__sep">&vert;</span>
    <span class="title-bar__workspace">{$workspace?.name ?? "..."}</span>
    <span class="title-bar__filter">{$filter.toUpperCase()}</span>
  </div>
  <div class="title-bar__right">
    <span class="title-bar__cache">{cacheAge}</span>
    <button class="theme-toggle" onclick={toggleTheme}>&#9684; Theme</button>
  </div>
</div>

<style>
  .title-bar { display: flex; align-items: center; justify-content: space-between; padding: 3px 16px; background: var(--bg-secondary); border-bottom: 1px solid var(--border-default); --wails-draggable: drag; user-select: none; }
  .title-bar__left { display: flex; align-items: center; gap: 10px; }
  .title-bar__logo { font-weight: 700; font-size: 14px; color: var(--accent-blue); letter-spacing: -0.5px; }
  .title-bar__sep { color: var(--text-muted); font-weight: 300; }
  .title-bar__workspace { font-weight: 500; color: var(--text-secondary); font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
  .title-bar__filter { font-size: 11px; color: var(--text-hint); background: var(--bg-tertiary); padding: 2px 8px; border-radius: 10px; border: 1px solid var(--border-muted); }
  .title-bar__right { display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-muted); --wails-draggable: no-drag; }
  .title-bar__cache { font-variant-numeric: tabular-nums; }
  .theme-toggle { background: var(--bg-tertiary); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 3px 8px; cursor: pointer; font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary); transition: all var(--transition-fast); }
  .theme-toggle:hover { background: var(--bg-hover); color: var(--text-primary); }
</style>
