<script lang="ts">
  interface Props {
    placeholder: string;
  }
  let { placeholder }: Props = $props();
</script>

<!-- svelte-ignore a11y_autofocus -->
<textarea class="input" {placeholder} autofocus></textarea>
<div class="hint">Ctrl/Cmd+S Submit &middot; Esc Cancel</div>

<style>
  .input { width: 100%; background: var(--bg-input); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 10px 12px; color: var(--text-primary); font-family: var(--font-mono); font-size: 12px; outline: none; resize: vertical; min-height: 160px; caret-color: var(--accent-blue); }
  .input:focus { border-color: var(--accent-blue); }
  .hint { font-size: 11px; color: var(--text-hint); margin-top: 10px; }
</style>
