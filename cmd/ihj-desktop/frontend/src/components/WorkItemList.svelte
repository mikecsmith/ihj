<script lang="ts">
  import { filtered, cursor, setCursor } from "../lib/stores/ui";
  import WorkItemRow from "./WorkItemRow.svelte";

  let listBody: HTMLDivElement | undefined = $state();

  $effect(() => {
    const _ = $cursor;
    const el = listBody?.querySelector(".row--selected") as HTMLElement | null;
    el?.scrollIntoView({ block: "nearest" });
  });
</script>

<div class="list-pane">
  <div class="list__header">
    <span>ID</span>
    <span>P</span>
    <span>TYPE</span>
    <span>STATUS</span>
    <span>ASSIGNEE</span>
    <span>SUMMARY</span>
  </div>
  <div class="list-body" bind:this={listBody}>
    {#each $filtered as item, idx (item.id)}
      <WorkItemRow {item} selected={idx === $cursor} onclick={() => setCursor(idx)} />
    {/each}
  </div>
</div>

<style>
  .list-pane { flex: 1 1 35%; overflow-y: auto; scrollbar-width: thin; scrollbar-color: var(--border-default) transparent; min-height: 0; }
  .list__header { display: grid; grid-template-columns: 110px 18px 90px 140px 130px 1fr; gap: 0 8px; padding: 2px 16px; font-size: 11px; font-weight: 700; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid var(--border-muted); position: sticky; top: 0; background: var(--bg-primary); z-index: 1; }
</style>
