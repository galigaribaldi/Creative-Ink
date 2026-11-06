<script>
  // Served by the Go AssetServer middleware (see preview.go: previewRoute).
  const PREVIEW_URL = '/__preview/document.pdf'

  /**
   * @type {{
   *   state: 'idle' | 'compiling' | 'success' | 'error',
   *   pdfTimestamp: number,
   *   logs: string
   * }}
   */
  let { state, pdfTimestamp, logs } = $props()

  const LABELS = {
    idle: 'Sin compilar',
    compiling: 'Compilando…',
    success: '✅ Compilado',
    error: '❌ Error de compilación'
  }

  // A new timestamp gives a new URL, which forces the iframe to reload the
  // freshly compiled PDF (the Go side also sends Cache-Control: no-store).
  let src = $derived(pdfTimestamp ? `${PREVIEW_URL}?t=${pdfTimestamp}` : '')
</script>

<div class="preview">
  <header class="bar">
    <span>Vista previa PDF</span>
    <span class="state state-{state}">{LABELS[state]}</span>
  </header>

  <div class="viewer">
    {#if src}
      <iframe {src} title="Vista previa del PDF compilado"></iframe>
    {:else}
      <p class="placeholder">
        {state === 'compiling' ? 'Compilando el documento…' : 'Escribe un documento LaTeX válido para ver el PDF.'}
      </p>
    {/if}
  </div>

  {#if state === 'error' && logs}
    <details class="logs" open>
      <summary>Log de compilación</summary>
      <pre>{logs}</pre>
    </details>
  {/if}
</div>

<style>
  .preview {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .bar {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.25rem 0.75rem;
    border-bottom: 1px solid #333;
    background: #1b1d23;
    font-size: 0.75rem;
    color: #9d9d9d;
  }

  .state-success {
    color: #4ade80;
  }

  .state-compiling {
    color: #fbbf24;
  }

  .state-error {
    color: #f87171;
  }

  .viewer {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #2a2a2a;
  }

  iframe {
    width: 100%;
    height: 100%;
    border: 0;
    background: #525659;
  }

  .placeholder {
    margin: 0;
    padding: 1rem;
    text-align: center;
    color: #6b6b6b;
  }

  .logs {
    max-height: 40%;
    display: flex;
    flex-direction: column;
    border-top: 1px solid #5c2b2b;
    background: #1a1414;
  }

  .logs summary {
    padding: 0.35rem 0.75rem;
    font-size: 0.75rem;
    color: #f87171;
    cursor: pointer;
  }

  .logs pre {
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
