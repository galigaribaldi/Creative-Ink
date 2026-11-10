<script>
  import { onMount } from 'svelte'
  import { GetRepositoryOptions } from '../../wailsjs/go/main/App.js'
  import { openFolder, setupRepository } from '../lib/actions.js'
  import { workspace } from '../lib/workspace.svelte.js'

  /** @type {import('../../wailsjs/go/models').system.RepositoryOptions | null} */
  let options = $state(null)
  let loadError = $state('')
  /** @type {'' | 'user' | 'shared'} */
  let busy = $state('')

  const CHOICES = [
    {
      scope: 'user',
      title: 'Solo para mí',
      detail: 'En tus Documentos. Solo tu cuenta de usuario lo ve.'
    },
    {
      scope: 'shared',
      title: 'Para todos los usuarios',
      detail: 'En la carpeta de documentos compartida del equipo. Todas las cuentas lo ven.'
    }
  ]

  onMount(async () => {
    try {
      options = await GetRepositoryOptions()
    } catch (err) {
      loadError = String(err)
    }
  })

  /** @param {'user' | 'shared'} scope */
  async function choose(scope) {
    busy = scope
    // On success Go returns needsSetup=false and this modal unmounts.
    await setupRepository(scope)
    busy = ''
  }
</script>

<div class="backdrop">
  <div class="panel" role="dialog" aria-modal="true" aria-labelledby="setup-title" aria-describedby="setup-desc">
    <h1 id="setup-title">Bienvenido a Creative-ink</h1>
    <p id="setup-desc" class="lead">
      Elige dónde vivirá tu repositorio central <strong>My-Creative-Ink-Repository</strong>.
      {#if options}Sistema detectado: <strong>{options.osName}</strong>.{/if}
    </p>

    {#if options}
      <div class="choices">
        {#each CHOICES as choice (choice.scope)}
          {@const option = options[choice.scope]}
          <button
            class="choice"
            onclick={() => choose(choice.scope)}
            disabled={!option.available || busy !== ''}
            aria-describedby="path-{choice.scope}"
          >
            <span class="title">{busy === choice.scope ? 'Creando…' : choice.title}</span>
            <span class="detail">{choice.detail}</span>
            <code id="path-{choice.scope}" class="path" class:unavailable={!option.available}>
              {option.available ? option.path : option.reason}
            </code>
          </button>
        {/each}
      </div>

      {#if options.warnings.length}
        <ul class="warnings">
          {#each options.warnings as warning}
            <li>⚠️ {warning}</li>
          {/each}
        </ul>
      {/if}
    {:else if loadError}
      <p class="error" role="alert">{loadError}</p>
    {:else}
      <p class="muted">Detectando el sistema…</p>
    {/if}

    {#if workspace.error}
      <p class="error" role="alert">{workspace.error}</p>
    {/if}

    <footer>
      <span class="muted">Si la carpeta ya existe, se usa tal cual: nunca se sobrescriben documentos.</span>
      <button class="link" onclick={openFolder} disabled={busy !== ''}>Abrir otra carpeta…</button>
    </footer>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: grid;
    place-items: center;
    padding: 1rem;
    background: var(--scrim);
  }

  .panel {
    width: min(640px, 100%);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 1.75rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
    font-size: var(--text-md);
    color: var(--text-primary);
  }

  h1 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .lead {
    margin: 0;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .choices {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  /* Two equivalent choices: neutral cards, neither is "the" primary action. */
  .choice {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.4rem;
    min-width: 0;
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-base);
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
    transition: background-color var(--transition), border-color var(--transition), opacity var(--transition);
  }

  .choice:hover:not(:disabled) {
    border-color: var(--border-strong);
    background: var(--surface-hover);
  }

  .choice:focus-visible,
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .choice:disabled {
    cursor: default;
    opacity: 0.55;
  }

  .title {
    font-weight: 500;
  }

  .detail {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    line-height: 1.4;
  }

  .path {
    width: 100%;
    margin-top: 0.25rem;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    overflow-wrap: anywhere;
  }

  .path.unavailable {
    color: var(--text-muted);
    font-family: inherit;
  }

  .warnings {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: var(--text-sm);
    line-height: 1.4;
    color: var(--warning);
  }

  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  .muted {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  /* Links are not accent-coloured (A2). */
  .link {
    flex-shrink: 0;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    font-size: var(--text-sm);
    text-decoration: underline transparent;
    text-underline-offset: 2px;
    cursor: pointer;
    transition: color var(--transition), text-decoration-color var(--transition);
  }

  .link:hover:not(:disabled) {
    color: var(--text-primary);
    text-decoration-color: currentColor;
  }

  .error {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--danger);
  }

  @media (max-width: 560px) {
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
