<script>
  /** @type {{ logs: string }} */
  let { logs } = $props()

  let collapsed = $state(false)

  // A new failure always opens the panel again, even if it was collapsed.
  $effect(() => {
    logs
    collapsed = false
  })
</script>

<section class="log" class:collapsed role="alert" aria-label="Errores de compilación de LaTeX">
  <header>
    <span class="title">❌ Error de compilación de LaTeX</span>
    <button onclick={() => (collapsed = !collapsed)} aria-expanded={!collapsed}>
      {collapsed ? 'Mostrar' : 'Ocultar'}
    </button>
  </header>
  {#if !collapsed}
    <pre>{logs}</pre>
  {/if}
</section>

<style>
  .log {
    max-height: 40%;
    display: flex;
    flex-direction: column;
    border-top: 2px solid #f87171;
    background: #1a1414;
  }

  .log.collapsed {
    max-height: none;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.3rem 0.75rem;
  }

  .title {
    font-size: 0.8rem;
    font-weight: 600;
    color: #f87171;
  }

  button {
    padding: 0.1rem 0.6rem;
    border: 1px solid #5c2b2b;
    border-radius: 4px;
    background: transparent;
    color: #e5c0c0;
    font: inherit;
    font-size: 0.75rem;
    cursor: pointer;
  }

  button:hover {
    border-color: #f87171;
  }

  pre {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 0 0.75rem 0.75rem;
    overflow: auto;
    font-family: ui-monospace, 'SF Mono', Menlo, monospace;
    font-size: 0.75rem;
    line-height: 1.45;
    color: #e5c0c0;
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>
