<script lang="ts">
  import type { WorkItem } from "../lib/types";

  interface Props {
    item: WorkItem;
    selected: boolean;
    onclick: () => void;
  }

  let { item, selected, onclick }: Props = $props();

  function typeColor(t: string): string {
    switch (t?.toLowerCase()) {
      case "epic": return "var(--type-epic)";
      case "story": return "var(--type-story)";
      case "bug": return "var(--type-bug)";
      case "sub-task": case "subtask": return "var(--type-subtask)";
      default: return "var(--type-task)";
    }
  }

  function statusIcon(s: string): { icon: string; color: string } {
    const l = s?.toLowerCase() || "";
    if (l.includes("done") || l.includes("closed") || l.includes("resolved")) return { icon: "\u2714", color: "var(--status-done)" };
    if (l.includes("block") || l.includes("hold") || l.includes("cancel")) return { icon: "\u2718", color: "var(--status-blocked)" };
    if (l.includes("review") || l.includes("test") || l.includes("qa")) return { icon: "\u25C9", color: "var(--status-review)" };
    if (l.includes("progress") || l.includes("doing") || l.includes("active")) return { icon: "\u25B6", color: "var(--status-active)" };
    if (l.includes("ready") || l.includes("refined") || l.includes("approved")) return { icon: "\u2605", color: "var(--status-ready)" };
    return { icon: "\u25CB", color: "var(--status-default)" };
  }

  function prioIcon(p: string): { icon: string; color: string } {
    const l = (p || "").toLowerCase();
    if (l.includes("crit") || l.includes("highest")) return { icon: "\u25B2", color: "var(--prio-critical)" };
    if (l.includes("high")) return { icon: "\u25B4", color: "var(--prio-high)" };
    if (l.includes("medium")) return { icon: "\u25C6", color: "var(--prio-medium)" };
    if (l.includes("low")) return { icon: "\u25BE", color: "var(--prio-low)" };
    if (l.includes("trivial") || l.includes("lowest")) return { icon: "\u25BC", color: "var(--prio-trivial)" };
    return { icon: "\u2212", color: "var(--text-muted)" };
  }

  const tc = $derived(typeColor(item.type));
  const si = $derived(statusIcon(item.status));
  const pi = $derived(prioIcon(item.fields?.priority as string ?? ""));
  const depth = $derived(item.depth ?? 0);
  const childCount = $derived(item.children?.length ?? 0);
  const indent = $derived(depth > 0 ? "\u00A0".repeat((depth - 1) * 3) + "\u21B3 " : "");
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="row" class:row--selected={selected} {onclick}>
  <span class="cell cell--key" style:color={tc}>{item.id}</span>
  <span class="cell cell--prio" style:color={pi.color}>{pi.icon}</span>
  <span class="cell cell--type" style:color={tc}>{item.type}</span>
  <span class="cell cell--status" style:color={si.color}>{si.icon} {item.status}</span>
  <span class="cell cell--assignee">{item.displayFields?.assignee || item.fields?.assignee || "\u2014"}</span>
  <span class="cell cell--summary" style:color={tc}>
    {#if depth > 0}<span class="tree">{indent}</span>{/if}
    {#if selected}<b>{item.summary}</b>{:else}{item.summary}{/if}
    {#if childCount > 0}<span class="child-count">({childCount})</span>{/if}
  </span>
</div>

<style>
  .row { display: grid; grid-template-columns: 110px 18px 90px 140px 130px 1fr; gap: 0 8px; padding: 3px 16px; font-size: 12px; cursor: pointer; transition: background var(--transition-fast); border-left: 2px solid transparent; align-items: center; scroll-margin-top: 24px; }
  .row--selected { background: var(--bg-active); border-left-color: var(--accent-blue); }
  .cell { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cell--key { font-weight: 600; }
  .cell--prio { text-align: center; }
  .cell--type { font-size: 11px; text-transform: uppercase; letter-spacing: 0.3px; }
  .cell--status { font-size: 11px; }
  .cell--assignee { color: var(--text-muted); font-size: 11px; }
  .tree { color: var(--text-muted); }
  .child-count { color: var(--text-muted); font-size: 10px; margin-left: 4px; }
</style>
