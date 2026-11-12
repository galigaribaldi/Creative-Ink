<script>
  // A short message at the bottom of the window (ui.showToast): dropped
  // files that cannot be opened, drops while an import runs… Never modal.
  import { ui } from '../lib/ui.svelte.js'
</script>

{#if ui.toast}
  {#key ui.toast.id}
    <div class="toast toast-{ui.toast.level}" role="status" aria-live="polite">
      <span>{ui.toast.text}</span>
      <button onclick={() => (ui.toast = null)} aria-label="Cerrar el aviso">×</button>
    </div>
  {/key}
{/if}

<style>
  /* Above everything, raised surface and overlay shadow (A3). */
  .toast {
    position: fixed;
    left: 50%;
    bottom: 1.25rem;
    z-index: 30;
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    max-width: min(36rem, calc(100vw - 2rem));
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
    color: var(--text-primary);
    font-size: var(--text-sm);
    line-height: 1.45;
    transform: translateX(-50%);
  }

  .toast-warning {
    box-shadow: inset 3px 0 0 var(--warning), var(--shadow-overlay);
  }

  button {
    flex-shrink: 0;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    cursor: pointer;
    transition: color var(--transition);
  }

  button:hover {
    color: var(--text-primary);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
