<script lang="ts">
  interface ActionDef {
    key: string;
    label: string;
    action: string;
  }

  const actions: ActionDef[] = [
    { key: "Alt-R", label: "Refresh", action: "refresh" },
    { key: "Alt-F", label: "Filter", action: "filter" },
    { key: "Alt-A", label: "Assign", action: "assign" },
    { key: "Alt-T", label: "Transition", action: "transition" },
    { key: "Alt-O", label: "Open", action: "open" },
    { key: "Alt-E", label: "Edit", action: "edit" },
    { key: "Alt-C", label: "Comment", action: "comment" },
    { key: "Alt-N", label: "Branch", action: "branch" },
    { key: "Alt-X", label: "Extract", action: "extract" },
    { key: "Ctrl-N", label: "New", action: "create" },
    { key: "Alt-B", label: "Bulk Edit", action: "bulkEdit" },
  ];

  interface Props {
    onAction: (action: string) => void;
  }
  let { onAction }: Props = $props();
</script>

<div class="action-bar">
  {#each actions as act, i}
    {#if i > 0}<span class="sep">&vert;</span>{/if}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="item" onclick={() => onAction(act.action)}>
      <span class="key">{act.key}</span>
      <span class="label">{act.label}</span>
    </div>
  {/each}
</div>

<style>
  .action-bar { display: flex; align-items: center; gap: 4px; padding: 6px 16px; background: var(--bg-secondary); border-top: 1px solid var(--border-default); flex-wrap: wrap; }
  .item { display: flex; align-items: center; gap: 3px; font-size: 11px; cursor: pointer; padding: 2px 6px; border-radius: var(--radius-sm); transition: background var(--transition-fast); white-space: nowrap; }
  .item:hover { background: var(--bg-hover); }
  .key { color: var(--accent-cyan); font-weight: 600; font-size: 10px; }
  .label { color: var(--text-hint); }
  .sep { color: var(--border-default); margin: 0 2px; user-select: none; }
</style>
