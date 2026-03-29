<script lang="ts">
  import { toasts } from "../lib/stores/ui";
</script>

<div class="toast-container">
  {#each $toasts as toast (toast.id)}
    <div class="toast">
      <span class="toast__icon toast__icon--{toast.type}">&bull;</span>
      {toast.message}
    </div>
  {/each}
</div>

<style>
  .toast-container { position: fixed; bottom: 48px; right: 24px; z-index: 200; display: flex; flex-direction: column; gap: 8px; }
  .toast { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); padding: 6px 12px; font-size: 12px; color: var(--text-secondary); display: flex; align-items: center; gap: 6px; box-shadow: 0 4px 16px rgba(0,0,0,0.3); animation: slideUp 120ms ease; }
  @keyframes slideUp { from { transform: translateY(8px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
  .toast__icon--success { color: var(--accent-green); }
  .toast__icon--error { color: var(--accent-red); }
  .toast__icon--info { color: var(--accent-blue); }
</style>
