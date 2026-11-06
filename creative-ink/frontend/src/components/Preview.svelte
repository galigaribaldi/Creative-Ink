<script>
  import { onMount, untrack } from 'svelte'
  import { loadPdf } from '../lib/pdf.js'

  // Served by the Go AssetServer middleware (see preview.go: previewRoute).
  const PREVIEW_URL = '/__preview/document.pdf'
  // Horizontal padding of .viewer (keep in sync with the CSS below).
  const VIEWER_PADDING_PX = 16
  const RESIZE_DEBOUNCE_MS = 150

  /**
   * @type {{
   *   compileState: 'idle' | 'compiling' | 'success' | 'error',
   *   pdfTimestamp: number
   * }}
   */
  let { compileState, pdfTimestamp } = $props()

  const LABELS = {
    idle: 'Sin compilar',
    compiling: 'Compilando…',
    success: '✅ Compilado',
    error: '❌ Error de compilación'
  }

  /** @type {HTMLDivElement} */
  let viewer
  /** @type {HTMLCanvasElement} */
  let canvas

  // Non-reactive rendering bookkeeping.
  /** @type {import('pdfjs-dist').PDFDocumentProxy | null} */
  let pdfDoc = null // last successfully rendered document
  /** @type {import('pdfjs-dist').RenderTask | null} */
  let renderTask = null
  let loadSeq = 0 // identifies the latest reload; stale ones are discarded
  let renderSeq = 0 // identifies the latest render; stale ones are discarded
  let loading = false

  let hasPage = $state(false)
  let pageCount = $state(0)
  let renderError = $state('')

  // Every successful compilation bumps pdfTimestamp in App → new URL → reload.
  $effect(() => {
    if (!pdfTimestamp) return
    const url = `${PREVIEW_URL}?t=${pdfTimestamp}`
    untrack(() => reload(url))
  })

  /** Loads the new PDF and swaps it in only once its first page is drawn. */
  async function reload(url) {
    const seq = ++loadSeq
    loading = true
    try {
      const doc = await loadPdf(url)
      if (seq !== loadSeq) return void doc.destroy() // a newer compile won

      const drawn = await renderFirstPage(doc)
      if (seq !== loadSeq || !drawn) return void doc.destroy()

      const previous = pdfDoc
      pdfDoc = doc
      pageCount = doc.numPages
      renderError = ''
      previous?.destroy()
    } catch (err) {
      // The canvas still holds the last valid PDF; only report the problem.
      if (seq === loadSeq) renderError = err?.message ?? String(err)
    } finally {
      if (seq === loadSeq) loading = false
    }
  }

  /**
   * Renders page 1 into an offscreen canvas, then blits it onto the visible
   * one in a single step: the previous page stays on screen until the new
   * one is complete (no blank frame, no flicker).
   *
   * @param {import('pdfjs-dist').PDFDocumentProxy} doc
   * @returns {Promise<boolean>} false if superseded or cancelled
   */
  async function renderFirstPage(doc) {
    const seq = ++renderSeq
    renderTask?.cancel()

    const page = await doc.getPage(1)
    if (seq !== renderSeq) return false

    const dpr = window.devicePixelRatio || 1
    const cssWidth = Math.max(viewer.clientWidth - 2 * VIEWER_PADDING_PX, 100)
    const fitScale = cssWidth / page.getViewport({ scale: 1 }).width
    const viewport = page.getViewport({ scale: fitScale * dpr })

    const offscreen = document.createElement('canvas')
    offscreen.width = Math.floor(viewport.width)
    offscreen.height = Math.floor(viewport.height)

    renderTask = page.render({ canvas: offscreen, viewport })
    try {
      await renderTask.promise
    } catch (err) {
      if (err?.name === 'RenderingCancelledException') return false
      throw err
    } finally {
      if (seq === renderSeq) renderTask = null
    }
    if (seq !== renderSeq) return false

    canvas.width = offscreen.width
    canvas.height = offscreen.height
    canvas.style.width = `${offscreen.width / dpr}px`
    canvas.style.height = `${offscreen.height / dpr}px`
    canvas.getContext('2d').drawImage(offscreen, 0, 0)
    hasPage = true
    return true
  }

  onMount(() => {
    // Re-render the current document at the new width when the pane resizes.
    let timer
    const observer = new ResizeObserver(() => {
      clearTimeout(timer)
      timer = setTimeout(() => {
        // An in-flight reload will render at the new width by itself.
        if (pdfDoc && !loading) {
          renderFirstPage(pdfDoc).catch((err) => (renderError = err?.message ?? String(err)))
        }
      }, RESIZE_DEBOUNCE_MS)
    })
    observer.observe(viewer)

    return () => {
      clearTimeout(timer)
      observer.disconnect()
      loadSeq++ // invalidate in-flight loads
      renderTask?.cancel()
      pdfDoc?.destroy()
      pdfDoc = null
    }
  })
</script>

<div class="preview">
  <header class="bar">
    <span>
      Vista previa PDF
      {#if pageCount}<span class="pages">· página 1 de {pageCount}</span>{/if}
    </span>
    <span class="state-{compileState}">{LABELS[compileState]}</span>
  </header>

  <div class="viewer" bind:this={viewer}>
    <canvas bind:this={canvas} class:hidden={!hasPage} aria-label="Primera página del PDF compilado"
    ></canvas>
    {#if !hasPage}
      <p class="placeholder">
        {compileState === 'compiling' ? 'Compilando el documento…' : 'Escribe un documento LaTeX válido para ver el PDF.'}
      </p>
    {/if}
  </div>

  {#if renderError}
    <p class="render-error" role="alert">⚠️ No se pudo mostrar el PDF: {renderError}</p>
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

  .pages {
    color: #6b6b6b;
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
    overflow: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    /* Keep in sync with VIEWER_PADDING_PX. */
    padding: 16px;
    background: #2a2a2a;
  }

  canvas {
    display: block;
    flex-shrink: 0;
    background: #fff;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.5);
  }

  canvas.hidden {
    display: none;
  }

  .placeholder {
    margin: auto 0;
    padding: 1rem;
    text-align: center;
    color: #6b6b6b;
  }

  .render-error {
    margin: 0;
    padding: 0.35rem 0.75rem;
    border-top: 1px solid #5c4a1f;
    background: #1f1a10;
    font-size: 0.75rem;
    color: #fbbf24;
  }
</style>
