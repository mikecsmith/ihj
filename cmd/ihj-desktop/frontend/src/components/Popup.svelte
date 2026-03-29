<script lang="ts">
  import { popup } from "../lib/stores/ui";
  import BulkEditor from "./BulkEditor.svelte";
  import SelectPopup from "./popup/SelectPopup.svelte";
  import InputPopup from "./popup/InputPopup.svelte";
  import ReviewDiffPopup from "./popup/ReviewDiffPopup.svelte";
  import EditFormPopup from "./popup/EditFormPopup.svelte";

  let selectCursor = $state(0);

  function close(result: unknown = null) {
    const p = $popup;
    if (!p) return;
    popup.set(null);
    if (p.kind === "select") p.onResult(result as number | null);
    else if (p.kind === "input") p.onResult(result as string | null);
    else if (p.kind === "editor") p.onResult(result as { action: string; content: string } | null);
    else if (p.kind === "reviewdiff") p.onResult(result as number | null);
    else if (p.kind === "form") p.onResult(result as { metadata: Record<string, string>; description: string } | null);
  }

  function onOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains("popup-overlay")) close(null);
  }

  function onKeydown(e: KeyboardEvent) {
    const p = $popup;
    if (!p) return;

    if (p.kind === "editor" || p.kind === "form") return;

    if (e.key === "Escape") { e.preventDefault(); close(null); return; }

    if (p.kind === "select" || p.kind === "reviewdiff") {
      e.preventDefault();
      e.stopPropagation();
      if (e.key === "ArrowUp") selectCursor = Math.max(0, selectCursor - 1);
      else if (e.key === "ArrowDown") selectCursor = Math.min(p.options.length - 1, selectCursor + 1);
      else if (e.key === "Enter") close(selectCursor);
      else if (e.key >= "1" && e.key <= "9") {
        const idx = parseInt(e.key) - 1;
        if (idx < p.options.length) close(idx);
      }
    } else if (p.kind === "input") {
      const isSubmit = ((e.ctrlKey || e.metaKey) && e.code === "KeyS") || (e.altKey && e.key === "Enter");
      if (isSubmit) {
        e.preventDefault();
        e.stopPropagation();
        const v = (document.querySelector(".input") as HTMLTextAreaElement)?.value?.trim();
        close(v || null);
      }
    }
  }

  $effect(() => {
    if ($popup?.kind === "select" || $popup?.kind === "reviewdiff") selectCursor = 0;
  });
</script>

<svelte:window on:keydown={onKeydown} />

{#if $popup}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="popup-overlay" onclick={onOverlayClick}>
    <div class="popup" class:popup--wide={$popup.kind === "editor"} class:popup--form={$popup.kind === "form"}>
      <div class="popup__title">{$popup.title}</div>

      {#if $popup.kind === "select"}
        <SelectPopup options={$popup.options} cursor={selectCursor} onSelect={(i) => close(i)} />

      {:else if $popup.kind === "input"}
        <InputPopup placeholder={$popup.placeholder} />

      {:else if $popup.kind === "editor"}
        <BulkEditor
          content={$popup.content}
          schema={$popup.schema}
          onSave={(yaml) => close({ action: "save", content: yaml })}
          onCancel={() => close(null)}
        />

      {:else if $popup.kind === "reviewdiff"}
        <ReviewDiffPopup changes={$popup.changes} options={$popup.options} cursor={selectCursor} onSelect={(i) => close(i)} />

      {:else if $popup.kind === "form"}
        <EditFormPopup
          metadata={$popup.metadata}
          description={$popup.description}
          fields={$popup.fields}
          statuses={$popup.statuses}
          types={$popup.types}
          onSave={(result) => close(result)}
          onCancel={() => close(null)}
        />
      {/if}
    </div>
  </div>
{/if}

<style>
  .popup-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(3px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: fadeIn 100ms ease; }
  @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
  @keyframes slideUp { from { transform: translateY(8px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
  .popup { background: var(--bg-secondary); border: 1px solid var(--border-default); border-radius: var(--radius-lg); padding: 16px 20px; min-width: 480px; max-width: 680px; max-height: 80vh; overflow-y: auto; box-shadow: var(--shadow-popup); animation: slideUp 120ms ease; }
  .popup--wide { width: calc(100vw - 80px); max-width: calc(100vw - 80px); height: calc(100vh - 80px); max-height: calc(100vh - 80px); padding: 0; overflow: hidden; }
  .popup--form { min-width: 560px; max-width: 640px; }
  .popup__title { font-weight: 700; font-size: 13px; color: var(--accent-blue); margin-bottom: 12px; }
</style>
