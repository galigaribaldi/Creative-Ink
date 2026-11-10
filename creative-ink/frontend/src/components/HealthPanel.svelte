<script>
  import { onMount } from 'svelte'
  import { SystemHealthCheck } from '../../wailsjs/go/main/App.js'

  /** @type {{ onclose: () => void }} */
  let { onclose } = $props()

  // Maps each HealthStatus field (json tag in Go) to its display label.
  const TOOLS = [
    { key: 'hasXeLaTeX', label: 'XeLaTeX' },
    { key: 'hasLatexmk', label: 'Latexmk' },
    { key: 'hasBiber', label: 'Biber' }
  ]

  /** @type {import('../../wailsjs/go/models').system.HealthStatus | null} */
  let status = $state(null)
  let loading = $state(false)
  let error = $state('')

  const SCOPE_LABELS = { user: 'Solo para mí', shared: 'Para todos los usuarios' }

  // A failed Go probe reports 0; show "No disponible" instead of a fake 0.
  const fmt = (value, unit) => (value > 0 ? `${value} ${unit}` : 'No disponible')

  let hardware = $derived(
    status
      ? [
          { label: 'Memoria RAM', value: fmt(status.totalRAMGB, 'GB') },
          { label: 'Núcleos de CPU', value: fmt(status.cpuCores, 'núcleos') },
          { label: 'Disco libre (/)', value: fmt(status.freeDiskSpaceGB, 'GB') }
        ]
      : []
  )

  async function checkHealth() {
    loading = true
    error = ''
    try {
      // The binding returns a Promise that resolves with the Go struct
      // already deserialised into a plain JS object.
      status = await SystemHealthCheck()
    } catch (err) {
      status = null
      error = String(err)
    } finally {
      loading = false
    }
  }

  function onkeydown(event) {
    if (event.key === 'Escape') onclose()
  }

  onMount(checkHealth)
</script>

<svelte:window {onkeydown} />

<!-- Escape is handled on window; the backdrop click is a mouse convenience. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="panel" role="dialog" aria-modal="true" aria-labelledby="health-title">
    <header>
      <h1 id="health-title">Estado del sistema</h1>
      <div class="actions">
        <button onclick={checkHealth} disabled={loading}>
          {loading ? 'Verificando…' : 'Volver a verificar'}
        </button>
        <button class="close" onclick={onclose} aria-label="Cerrar">✕</button>
      </div>
    </header>

    {#if status}
      <div class="panels">
        <section>
          <h2>Herramientas de LaTeX</h2>
          <ul class="list">
            {#each TOOLS as tool (tool.key)}
              {@const installed = status[tool.key]}
              <li class="card" class:ok={installed} class:missing={!installed}>
                <span>{tool.label}</span>
                <span class="state">{installed ? '✅ Instalado' : '❌ No encontrado'}</span>
              </li>
            {/each}
          </ul>
        </section>

        <section>
          <h2>Recursos de Hardware</h2>
          <ul class="list">
            {#each hardware as item (item.label)}
              <li class="card">
                <span>{item.label}</span>
                <span class="value">{item.value}</span>
              </li>
            {/each}
          </ul>
          <span class="badge tier-{status.hardwareTierId}">{status.hardwareTier}</span>
        </section>
      </div>

      <section class="repository">
        <h2>Sistema y repositorio</h2>
        <ul class="list">
          <li class="card">
            <span>Sistema operativo</span>
            <span class="value">{status.repository.osName}</span>
          </li>
          <li class="card">
            <span>Repositorio actual</span>
            <span class="value path" title={status.repository.currentPath}>
              {status.repository.currentPath
                ? `${SCOPE_LABELS[status.repository.currentScope] ?? ''} · ${status.repository.currentPath}`
                : 'Sin configurar'}
            </span>
          </li>
          {#each [status.repository.user, status.repository.shared] as option (option.scope)}
            <li class="card" class:missing={!option.available}>
              <span>Propuesta: {SCOPE_LABELS[option.scope]}</span>
              <span class="value path" title={option.path || option.reason}>
                {option.available ? option.path : option.reason}
              </span>
            </li>
          {/each}
        </ul>
        {#if status.repository.warnings.length}
          <ul class="warnings">
            {#each status.repository.warnings as warning}
              <li>⚠️ {warning}</li>
            {/each}
          </ul>
        {/if}
      </section>

      {#if status.warnings.length}
        <ul class="warnings">
          {#each status.warnings as warning}
            <li>⚠️ {warning}</li>
          {/each}
        </ul>
      {/if}
    {:else if loading}
      <p class="muted">Verificando…</p>
    {/if}

    {#if error}
      <p class="err">{error}</p>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--scrim);
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    max-width: calc(100vw - 2rem);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    padding: 1.5rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
    font-size: var(--text-md);
    color: var(--text-primary);
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 2rem;
  }

  h1 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  h2 {
    margin: 0 0 0.75rem;
    font-size: var(--text-xs);
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }

  .actions {
    display: flex;
    gap: 0.5rem;
  }

  /* An informational dialog: its actions are secondary (neutral, outlined). */
  button {
    padding: 0.4rem 0.9rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-md);
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
    opacity: 0.6;
    cursor: default;
  }

  .close {
    border-color: transparent;
    color: var(--text-secondary);
  }

  .close:hover:not(:disabled) {
    color: var(--text-primary);
  }

  .panels {
    display: flex;
    flex-wrap: wrap;
    gap: 1.5rem;
  }

  section {
    display: flex;
    flex-direction: column;
    width: 300px;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .card {
    display: flex;
    justify-content: space-between;
    gap: 1.5rem;
    padding: 0.6rem 1rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-base);
  }

  /* Quiet success, visible problems (A1). */
  .card.ok .state {
    color: var(--text-secondary);
  }

  .card.missing .state {
    color: var(--danger);
  }

  .value {
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }

  .badge {
    align-self: flex-start;
    margin-top: 0.75rem;
    padding: 0.25rem 0.6rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .tier-advanced {
    color: var(--text-primary);
  }

  .tier-basic {
    color: var(--warning);
  }

  .tier-unknown {
    color: var(--text-muted);
  }

  .repository {
    width: auto;
    max-width: 624px;
  }

  .path {
    max-width: 380px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl; /* keep the end of long paths visible */
    text-align: left;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .card.missing .path {
    color: var(--text-muted);
  }

  .warnings {
    list-style: none;
    margin: 0;
    padding: 0;
    color: var(--warning);
    font-size: var(--text-sm);
  }

  .muted {
    margin: 0;
    color: var(--text-secondary);
  }

  .err {
    margin: 0;
    color: var(--danger);
  }
</style>
