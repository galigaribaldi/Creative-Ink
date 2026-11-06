<script>
  import Editor from './components/Editor.svelte'
  import HealthPanel from './components/HealthPanel.svelte'
  import Preview from './components/Preview.svelte'

  let showHealth = $state(false)

  /** @type {'idle' | 'compiling' | 'success' | 'error'} */
  let compileState = $state('idle')
  let compileLogs = $state('')
  // Bumped after every successful compilation to reload the PDF iframe.
  let pdfTimestamp = $state(0)

  function onsavestart() {
    compileState = 'compiling'
  }

  /** @param {import('../wailsjs/go/models').main.SaveResult} result */
  function onsaved(result) {
    compileLogs = result.compileLogs
    if (result.saved && result.compileSuccess) {
      compileState = 'success'
      pdfTimestamp = Date.now()
    } else {
      // Keep showing the last good PDF (pdfTimestamp unchanged) under the error.
      compileState = 'error'
    }
  }
</script>

<div class="shell">
  <header class="topbar">
    <span class="brand">Creative-ink</span>
    <button onclick={() => (showHealth = true)}>Ver estado del sistema</button>
  </header>

  <div class="workspace">
    <section class="pane editor-pane" aria-label="Editor">
      <Editor {onsavestart} {onsaved} />
    </section>
    <section class="pane" aria-label="Vista previa">
      <Preview state={compileState} {pdfTimestamp} logs={compileLogs} />
    </section>
  </div>

  {#if showHealth}
    <HealthPanel onclose={() => (showHealth = false)} />
  {/if}
</div>

<style>
  .shell {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.75rem;
    border-bottom: 1px solid #333;
    background: #181818;
  }

  .brand {
    font-size: 0.85rem;
    font-weight: 500;
    letter-spacing: 0.05em;
  }

  .topbar button {
    padding: 0.25rem 0.75rem;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    background: #2d2d2d;
    color: inherit;
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .topbar button:hover {
    border-color: var(--accent);
  }

  .workspace {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 1fr 1fr;
  }

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .editor-pane {
    border-right: 1px solid #333;
  }
</style>
