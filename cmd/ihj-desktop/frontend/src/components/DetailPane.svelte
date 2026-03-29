<script lang="ts">
  import { selectedItem } from "../lib/stores/ui";
  import { registry } from "../lib/stores/items";
  import type { WorkItem } from "../lib/types";
  import { Marked } from "marked";
  import hljs from "highlight.js";
  import {
    User, Calendar, Box, Tag, ArrowUp, ChevronRight,
    CircleCheck, CircleX, CircleDot, Play, Star, Circle,
    ChevronUp, ChevronsUp, Diamond, ChevronDown, ChevronsDown, Minus,
    GitBranch, MessageSquare, CornerDownRight,
  } from "lucide-svelte";

  const marked = new Marked({
    renderer: {
      code({ text, lang }) {
        const language = lang && hljs.getLanguage(lang) ? lang : "plaintext";
        const highlighted = hljs.highlight(text, { language }).value;
        return `<pre><code class="hljs language-${language}">${highlighted}</code></pre>`;
      },
    },
  });

  function renderMarkdown(md: string): string {
    return marked.parse(md) as string;
  }

  function typeColor(t: string): string {
    switch (t?.toLowerCase()) {
      case "epic": return "var(--type-epic)";
      case "story": return "var(--type-story)";
      case "bug": return "var(--type-bug)";
      case "sub-task": case "subtask": return "var(--type-subtask)";
      default: return "var(--type-task)";
    }
  }

  type StatusKey = "done" | "blocked" | "review" | "active" | "ready" | "default";
  type PrioKey = "critical" | "high" | "medium" | "low" | "trivial" | "none";

  function statusKey(s: string): StatusKey {
    const l = s?.toLowerCase() || "";
    if (l.includes("done") || l.includes("closed") || l.includes("resolved")) return "done";
    if (l.includes("block") || l.includes("hold") || l.includes("cancel")) return "blocked";
    if (l.includes("review") || l.includes("test") || l.includes("qa")) return "review";
    if (l.includes("progress") || l.includes("doing") || l.includes("active")) return "active";
    if (l.includes("ready") || l.includes("refined") || l.includes("approved")) return "ready";
    return "default";
  }

  const statusColors: Record<StatusKey, string> = {
    done: "var(--status-done)", blocked: "var(--status-blocked)", review: "var(--status-review)",
    active: "var(--status-active)", ready: "var(--status-ready)", default: "var(--status-default)",
  };

  function prioKey(p: string): PrioKey {
    const l = (p || "").toLowerCase();
    if (l.includes("crit") || l.includes("highest")) return "critical";
    if (l.includes("high")) return "high";
    if (l.includes("medium")) return "medium";
    if (l.includes("low")) return "low";
    if (l.includes("trivial") || l.includes("lowest")) return "trivial";
    return "none";
  }

  interface Props {
    onNavigate?: (id: string) => void;
  }
  let { onNavigate }: Props = $props();

  const item = $derived($selectedItem);
  const reg = $derived($registry);
</script>

{#snippet statusIcon(status: string)}
  {@const sk = statusKey(status)}
  <span class="icon-inline" style:color={statusColors[sk]}>
    {#if sk === "done"}<CircleCheck size={12} />
    {:else if sk === "blocked"}<CircleX size={12} />
    {:else if sk === "review"}<CircleDot size={12} />
    {:else if sk === "active"}<Play size={12} />
    {:else if sk === "ready"}<Star size={12} />
    {:else}<Circle size={12} />
    {/if}
  </span>
{/snippet}

{#snippet prioIcon(priority: string)}
  {@const pk = prioKey(priority)}
  <span class="icon-inline" class:prio-crit={pk === "critical"} class:prio-high={pk === "high"} class:prio-med={pk === "medium"} class:prio-low={pk === "low"} class:prio-triv={pk === "trivial"} class:prio-none={pk === "none"}>
    {#if pk === "critical"}<ChevronsUp size={12} />
    {:else if pk === "high"}<ChevronUp size={12} />
    {:else if pk === "medium"}<Diamond size={12} />
    {:else if pk === "low"}<ChevronDown size={12} />
    {:else if pk === "trivial"}<ChevronsDown size={12} />
    {:else}<Minus size={12} />
    {/if}
  </span>
{/snippet}

<div class="detail-pane">
  {#if !item}
    <div class="detail__empty">Select a work item to view details</div>
  {:else}
    <div class="detail__identity">
      <span class="detail__key">{item.id}</span>
      <span class="chevron"><ChevronRight size={10} /></span>
      <span class="detail__type" style:color={typeColor(item.type)}>{item.type}</span>
      <span class="chevron"><ChevronRight size={10} /></span>
      <span class="detail__status" style:color={statusColors[statusKey(item.status)]}>{@render statusIcon(item.status)} {item.status.toUpperCase()}</span>
      {#if item.fields?.priority}
        <span class="chevron"><ChevronRight size={10} /></span>
        <span class="detail__prio">{@render prioIcon(item.fields.priority as string)} {item.fields.priority}</span>
      {/if}
    </div>

    <div class="detail__meta-row">
      <span class="detail__meta-label label--cyan"><User size={11} /> Assignee:</span>
      <span class="detail__meta-value detail__meta-value--name">{item.displayFields?.assignee || item.fields?.assignee || "\u2014"}</span>
      <span class="detail__meta-label"><Calendar size={11} /> Created:</span>
      <span class="detail__meta-value">{item.fields?.created ?? "\u2014"}</span>
    </div>
    <div class="detail__meta-row">
      <span class="detail__meta-label"><User size={11} /> Reporter:</span>
      <span class="detail__meta-value detail__meta-value--name">{item.displayFields?.reporter || item.fields?.reporter || "\u2014"}</span>
      <span class="detail__meta-label"><Calendar size={11} /> Updated:</span>
      <span class="detail__meta-value">{item.fields?.updated ?? "\u2014"}</span>
    </div>
    {#if item.fields?.components}
      <div class="detail__meta-row">
        <span class="detail__meta-label" style:color="var(--accent-blue)"><Box size={11} /> Components:</span>
        <span class="detail__meta-value">{item.fields.components}</span>
      </div>
    {/if}
    {#if item.fields?.labels}
      <div class="detail__meta-row">
        <span class="detail__meta-label" style:color="var(--type-epic)"><Tag size={11} /> Labels:</span>
        <span class="detail__meta-value">{item.fields.labels}</span>
      </div>
    {/if}
    {#if item.parentId}
      <div class="detail__meta-row">
        <span class="detail__meta-label" style:color="var(--text-muted)"><ArrowUp size={11} /> Parent:</span>
        <span class="detail__meta-value"><b>{item.parentId}</b></span>
      </div>
    {/if}

    <hr class="detail__divider" />
    <div class="detail__summary">{item.summary}</div>
    {#if item.description}
      <div class="detail__description">{@html renderMarkdown(item.description)}</div>
    {:else}
      <div class="detail__description detail__description--empty">No description.</div>
    {/if}

    {#if item.children?.length > 0}
      <hr class="detail__divider" />
      <div class="section-header section-header--children"><GitBranch size={12} /> CHILDREN</div>
      {#each item.children as child, i}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="detail__child" onclick={() => onNavigate?.(child.id)}>
          <span class="detail__child-tree"><CornerDownRight size={11} /></span>
          <span class="detail__child-key" style:color={typeColor(child.type)}>{child.id}</span>
          <span class="detail__child-type" style:color={typeColor(child.type)}>{child.type.toUpperCase()}</span>
          <span class="detail__child-status" style:color={statusColors[statusKey(child.status)]}>{@render statusIcon(child.status)} {child.status}</span>
          <span class="detail__child-summary">{child.summary}</span>
          <span class="detail__child-hint">[{i + 1}]</span>
        </div>
      {/each}
    {/if}

    {#if item.comments?.length > 0}
      <hr class="detail__divider" />
      <div class="section-header section-header--comments"><MessageSquare size={12} /> LATEST COMMENTS</div>
      {#each item.comments as comment}
        <div class="detail__comment">
          <div>
            <span class="detail__comment-author">{comment.author}</span>
            <span class="detail__comment-date">&bull; {comment.created}</span>
          </div>
          <div class="detail__comment-body">{@html renderMarkdown(comment.body)}</div>
        </div>
      {/each}
    {/if}
  {/if}
</div>

<style>
  .detail-pane { flex: 1 1 50%; overflow-y: auto; border-bottom: 1px solid var(--border-default); padding: 12px 16px; scrollbar-width: thin; scrollbar-color: var(--border-default) transparent; }
  .detail__empty { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-muted); font-style: italic; }
  .detail__identity { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; margin-bottom: 6px; }
  .chevron { color: var(--text-muted); display: inline-flex; align-items: center; }
  .detail__key { font-weight: 700; font-size: 13px; }
  .detail__type { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; padding: 1px 6px; border-radius: 3px; background: var(--bg-tertiary); }
  .detail__status { font-size: 11px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.3px; display: inline-flex; align-items: center; gap: 3px; }
  .detail__prio { font-size: 11px; display: inline-flex; align-items: center; gap: 3px; }
  .detail__meta-row { display: flex; align-items: center; font-size: 12px; line-height: 1.6; }
  .detail__meta-label { color: var(--text-muted); white-space: nowrap; display: inline-flex; align-items: center; gap: 3px; width: 120px; flex-shrink: 0; }
  .label--cyan { color: var(--accent-cyan); }
  .detail__meta-value { color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail__meta-value--name { width: 200px; flex-shrink: 0; }
  .icon-inline { display: inline-flex; align-items: center; vertical-align: middle; }
  .prio-crit { color: var(--prio-critical); }
  .prio-high { color: var(--prio-high); }
  .prio-med { color: var(--prio-medium); }
  .prio-low { color: var(--prio-low); }
  .prio-triv { color: var(--prio-trivial); }
  .prio-none { color: var(--text-muted); }
  .detail__divider { border: none; border-top: 1px solid var(--border-muted); margin: 10px 0; }
  .detail__summary { font-family: var(--font-sans); font-size: 15px; font-weight: 700; text-transform: uppercase; letter-spacing: -0.3px; margin-bottom: 8px; line-height: 1.3; }
  .detail__description { font-family: var(--font-sans); font-size: 13px; color: var(--text-secondary); line-height: 1.7; max-width: 680px; }
  .detail__description--empty { font-style: italic; color: var(--text-muted); }
  .detail__description :global(p) { margin: 0 0 8px; }
  .detail__description :global(ul), .detail__description :global(ol) { margin: 0 0 8px; padding-left: 20px; }
  .detail__description :global(li) { margin-bottom: 2px; }
  .detail__description :global(h1), .detail__description :global(h2), .detail__description :global(h3) { font-size: 14px; font-weight: 700; margin: 12px 0 4px; color: var(--text-primary); }
  .detail__description :global(pre) { background: var(--bg-tertiary); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 10px 12px; overflow-x: auto; margin: 8px 0; font-size: 12px; }
  .detail__description :global(code) { font-family: var(--font-mono); font-size: 12px; }
  .detail__description :global(:not(pre) > code) { background: var(--bg-tertiary); padding: 1px 4px; border-radius: 3px; }
  .detail__description :global(a) { color: var(--accent-blue); text-decoration: none; }
  .detail__description :global(a:hover) { text-decoration: underline; }
  .detail__description :global(blockquote) { border-left: 3px solid var(--border-default); padding-left: 12px; color: var(--text-muted); margin: 8px 0; }
  /* Tokyo Night syntax colors for highlight.js */
  .detail__description :global(.hljs) { color: var(--text-primary); background: transparent; }
  .detail__description :global(.hljs-keyword) { color: #bb9af7; }
  .detail__description :global(.hljs-string) { color: #9ece6a; }
  .detail__description :global(.hljs-number) { color: #ff9e64; }
  .detail__description :global(.hljs-literal) { color: #ff9e64; }
  .detail__description :global(.hljs-comment) { color: #565f89; font-style: italic; }
  .detail__description :global(.hljs-function) { color: #7aa2f7; }
  .detail__description :global(.hljs-title) { color: #7dcfff; }
  .detail__description :global(.hljs-built_in) { color: #2ac3de; }
  .detail__description :global(.hljs-attr) { color: #7dcfff; }
  .detail__description :global(.hljs-type) { color: #2ac3de; }
  .detail__description :global(.hljs-params) { color: #e0af68; }
  .detail__description :global(.hljs-meta) { color: #565f89; }
  .detail__description :global(.hljs-variable) { color: #c0caf5; }
  .detail__description :global(.hljs-punctuation) { color: #a9b1d6; }
  .section-header { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 1px; margin: 4px 0 8px; display: flex; align-items: center; gap: 5px; }
  .section-header--children { color: var(--accent-blue); }
  .section-header--comments { color: var(--accent-yellow); }
  .detail__child { display: flex; align-items: center; gap: 6px; padding: 2px 6px 2px 16px; margin: 0 -6px; font-size: 12px; cursor: pointer; border-radius: var(--radius-sm); transition: background var(--transition-fast); }
  .detail__child:hover { background: var(--bg-hover); }
  .detail__child-tree { color: var(--text-muted); display: inline-flex; align-items: center; flex-shrink: 0; }
  .detail__child-key { font-weight: 600; width: 100px; flex-shrink: 0; }
  .detail__child-type { font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.3px; width: 60px; flex-shrink: 0; }
  .detail__child-status { font-size: 11px; width: 140px; flex-shrink: 0; display: inline-flex; align-items: center; gap: 3px; }
  .detail__child-summary { color: var(--text-secondary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail__child-hint { color: var(--text-hint); font-size: 10px; flex-shrink: 0; }
  .detail__comment { margin-bottom: 10px; }
  .detail__comment-author { font-weight: 600; font-size: 12px; }
  .detail__comment-date { color: var(--text-muted); margin-left: 6px; font-size: 12px; }
  .detail__comment-body { font-family: var(--font-sans); font-size: 12px; color: var(--text-secondary); line-height: 1.6; }
</style>
