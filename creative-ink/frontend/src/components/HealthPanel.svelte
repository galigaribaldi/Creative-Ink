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
    background: rgba(0, 0, 0, 0.55);
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    max-width: calc(100vw - 2rem);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    padding: 1.5rem;
    border: 1px solid #3c3c3c;
    border-radius: 8px;
    background: #1e1e1e;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 2rem;
  }

  h1 {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 500;
  }

  h2 {
    margin: 0 0 0.75rem;
    font-size: 0.8rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #9d9d9d;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
  }

  button {
    padding: 0.4rem 0.9rem;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    background: #2d2d2d;
    color: inherit;
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
  }

  button:hover:not(:disabled) {
    border-color: var(--accent);
  }

  button:disabled {
    opacity: 0.6;
    cursor: default;
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
    border: 1px solid #3c3c3c;
    border-left-width: 3px;
    border-radius: 4px;
    background: #252526;
  }

  .card.ok {
    border-left-color: #4ade80;
  }

  .card.ok .state {
    color: #4ade80;
  }

  .card.missing {
    border-left-color: #f87171;
  }

  .card.missing .state {
    color: #f87171;
  }

  .value {
    font-variant-numeric: tabular-nums;
    color: #e5e5e5;
  }

  .badge {
    align-self: flex-start;
    margin-top: 0.75rem;
    padding: 0.3rem 0.75rem;
    border: 1px solid;
    border-radius: 999px;
    font-size: 0.85rem;
    font-weight: 600;
  }

  .tier-advanced {
    color: #4ade80;
    border-color: #4ade80;
    background: rgba(74, 222, 128, 0.1);
  }

  .tier-basic {
    color: #fbbf24;
    border-color: #fbbf24;
    background: rgba(251, 191, 36, 0.1);
  }

  .tier-unknown {
    color: #9d9d9d;
    border-color: #6b6b6b;
    background: rgba(157, 157, 157, 0.1);
  }

  .warnings {
    list-style: none;
    margin: 0;
    padding: 0;
    color: #fbbf24;
    font-size: 0.85rem;
  }

  .muted {
    margin: 0;
    color: #9d9d9d;
  }

  .err {
    margin: 0;
    color: #f87171;
  }
</style>
