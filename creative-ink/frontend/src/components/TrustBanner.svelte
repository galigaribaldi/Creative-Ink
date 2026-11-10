<script>
  import { trustProject, untrustProject } from '../lib/actions.js'
  import { workspace } from '../lib/workspace.svelte.js'

  // Only for a .latexmkrc the user never decided about: a restricted folder
  // (the user said no) compiles with -norc without asking again.
  let visible = $derived(workspace.trustStatus === 'untrusted')
  let files = $derived(workspace.trustFiles.join(', ') || '.latexmkrc')
  let busy = $state(false)
  let error = $state('')

  /** @param {() => Promise<string>} action */
  async function decide(action) {
    busy = true
    error = await action()
    busy = false
  }
</script>

{#if visible}
  <div class="banner" role="status">
    <span class="icon" aria-hidden="true">⚠</span>
    <p class="text">
      Este proyecto contiene un archivo de configuración <code>{files}</code> desconocido. Se está compilando
      en modo restringido por seguridad.
      <span class="detail">Es código Perl que latexmk ejecutaría: confía solo si conoces su origen.</span>
    </p>
    <div class="actions">
      <button onclick={() => decide(trustProject)} disabled={busy} title="Guardar su huella y recompilar con su configuración">
        Confiar y recompilar
      </button>
      <button class="quiet" onclick={() => decide(untrustProject)} disabled={busy} title="Compilar siempre sin él (latexmk -norc) y no volver a preguntar">
        Mantener restringido
      </button>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </div>
{/if}

<style>
  /* Discreet but visible (A1: no modal): a warning edge on the chrome surface. */
  .banner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.75rem;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid var(--border);
    box-shadow: inset 2px 0 0 var(--warning);
    background: var(--surface-base);
    font-size: var(--text-sm);
    color: var(--text-primary);
  }

  .icon {
    color: var(--warning);
  }

  .text {
    flex: 1;
    min-width: 16rem;
    margin: 0;
    line-height: 1.45;
  }

  .text code {
    font-family: var(--font-mono);
    color: var(--warning);
  }

  .detail {
    color: var(--text-muted);
  }

  .actions {
    display: flex;
    gap: 0.4rem;
  }

  /* A security decision: neutral buttons, nothing nudges towards trusting. */
  button {
    padding: 0.25rem 0.6rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition), opacity var(--transition);
  }

  button:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .quiet {
    border-color: transparent;
    color: var(--text-secondary);
  }

  .error {
    flex-basis: 100%;
    margin: 0;
    color: var(--danger);
  }
</style>
