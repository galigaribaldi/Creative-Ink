<script>
  import { onDestroy } from 'svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import CompileLog from './components/CompileLog.svelte'
  import Editor from './components/Editor.svelte'
  import HealthPanel from './components/HealthPanel.svelte'
  import Preview from './components/Preview.svelte'

  // Event names emitted by Go (app.go: EventCompileStarted / EventCompileFinished).
  const COMPILE_STARTED = 'compile:started'
  const COMPILE_FINISHED = 'compile:finished'

  let showHealth = $state(false)

  /** @type {'idle' | 'compiling' | 'success' | 'error'} */
  let compileState = $state('idle')
  let compileLogs = $state('')
  // Bumped after every successful compilation to reload the PDF canvas.
  let pdfTimestamp = $state(0)

  // Subscribed during component init, i.e. before the Editor mounts and
  // triggers its first save, so no event can be missed.
  const unsubscribers = [
    EventsOn(COMPILE_STARTED, () => {
      compileState = 'compiling'
    }),
    /** @param {{ success: boolean, logs: string, durationMs: number }} result */
    EventsOn(COMPILE_FINISHED, (result) => {
      compileLogs = result.logs
      if (result.success) {
        compileState = 'success'
        pdfTimestamp = Date.now()
      } else {
        // pdfTimestamp is left untouched, so the canvas keeps the last valid PDF.
        compileState = 'error'
      }
    })
  ]

  // Without this, every hot reload would stack another set of listeners.
  onDestroy(() => unsubscribers.forEach((off) => off()))
</script>

<div class="shell">
  <header class="topbar">
    <span class="brand">Creative-ink</span>
    <button onclick={() => (showHealth = true)}>Ver estado del sistema</button>
  </header>

  <div class="workspace">
    <section class="pane editor-pane" aria-label="Editor">
      <div class="editor-host">
        <Editor />
      </div>
      {#if compileState === 'error' && compileLogs}
        <CompileLog logs={compileLogs} />
      {/if}
    </section>
    <section class="pane" aria-label="Vista previa">
      <Preview {compileState} {pdfTimestamp} />
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
    display: flex;
    flex-direction: column;
    border-right: 1px solid #333;
  }

  .editor-host {
    flex: 1;
    min-height: 0;
  }
</style>
