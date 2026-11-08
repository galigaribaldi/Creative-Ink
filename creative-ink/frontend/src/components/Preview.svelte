<script>
  import { onMount, untrack } from 'svelte'
  import { loadPdf, releasePdf } from '../lib/pdf.js'

  // Served by the Go AssetServer middleware (see preview.go: previewRoute).
  const PREVIEW_URL = '/__preview/document.pdf'
  // Horizontal padding of .viewer (keep in sync with the CSS below).
  const VIEWER_PADDING_PX = 16
  const RESIZE_DEBOUNCE_MS = 150

  /**
   * @type {{
   *   compileState: 'idle' | 'compiling' | 'success' | 'error',
   *   pdfTimestamp: number,
   *   documentKey: string
   * }}
   */
  let { compileState, pdfTimestamp, documentKey } = $props()

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

  // ── Ownership: exactly one live document at a time ───────────────────────
  // `pdfDoc` is the document currently on screen. A newly loaded document is
  // either promoted to `pdfDoc` (and the previous one released) or released
  // itself. Nothing else holds documents, so none can leak.
  /** @type {import('pdfjs-dist').PDFDocumentProxy | null} */
  let pdfDoc = null
  /** @type {import('pdfjs-dist').RenderTask | null} */
  let renderTask = null
  let loadSeq = 0 // identifies the latest reload; stale ones are discarded
  let renderSeq = 0 // identifies the latest render; stale ones are discarded
  let loading = false

  // Page the user is looking at; kept across recompilations (1-based).
  let currentPageNumber = $state(1)
  let pageCount = $state(0)
  let hasPage = $state(false)
  let renderError = $state('')

  // Another .tex was opened: its PDF is a different document, so drop the
  // current one and start again at page 1. Declared before the reload effect
  // so a reset never discards the new file's first PDF.
  let shownDocument = ''
  $effect(() => {
    const key = documentKey
    untrack(() => {
      if (key === shownDocument) return
      shownDocument = key
      loadSeq++ // in-flight loads become stale and release their own document
      loading = false
      renderTask?.cancel()
      void releasePdf(pdfDoc)
      pdfDoc = null
      currentPageNumber = 1
      pageCount = 0
      hasPage = false
      renderError = ''
    })
  })

  // Every successful compilation bumps pdfTimestamp → new URL → reload.
  $effect(() => {
    if (!pdfTimestamp) return
    const url = `${PREVIEW_URL}?t=${pdfTimestamp}`
    untrack(() => reload(url))
  })

  /** Clamps a page number to [1, numPages] (the new PDF may be shorter). */
  const clampPage = (n, numPages) => Math.min(Math.max(1, n), numPages)

  /**
   * Loads the new PDF, draws the remembered page and only then swaps it in.
   * Until that moment the canvas keeps showing the previous document.
   */
  async function reload(url) {
    const seq = ++loadSeq
    loading = true
    /** @type {import('pdfjs-dist').PDFDocumentProxy | null} */
    let doc = null
    try {
      doc = await loadPdf(url)
      if (seq !== loadSeq) return // a newer compilation won

      const target = clampPage(currentPageNumber, doc.numPages)
      const drawn = await renderPage(doc, target)
      if (seq !== loadSeq || !drawn) return

      // Promote: the new document is now the one on screen.
      const previous = pdfDoc
      pdfDoc = doc
      doc = null // ownership transferred; do not release below
      currentPageNumber = target
      pageCount = pdfDoc.numPages
      renderError = ''
      // Its render (if any) was cancelled by renderPage(), so it is safe to free.
      void releasePdf(previous)
    } catch (err) {
      // The canvas still holds the last valid PDF; only report the problem.
      if (seq === loadSeq) renderError = err?.message ?? String(err)
    } finally {
      // Any document that was not promoted is released here, on every path.
      if (doc) void releasePdf(doc)
      if (seq === loadSeq) loading = false
    }
  }

  /**
   * Renders one page into an offscreen canvas, then blits it onto the visible
   * one in a single step (no blank frame, no flicker).
   *
   * @param {import('pdfjs-dist').PDFDocumentProxy} doc
   * @param {number} pageNumber 1-based
   * @returns {Promise<boolean>} false if superseded or cancelled
   */
  async function renderPage(doc, pageNumber) {
    const seq = ++renderSeq
    // Never let two renders run: cancel the previous one before starting.
    renderTask?.cancel()

    const page = await doc.getPage(pageNumber)
    if (seq !== renderSeq) return false

    const dpr = window.devicePixelRatio || 1
    const cssWidth = Math.max(viewer.clientWidth - 2 * VIEWER_PADDING_PX, 100)
    const fitScale = cssWidth / page.getViewport({ scale: 1 }).width
    const viewport = page.getViewport({ scale: fitScale * dpr })

    const offscreen = document.createElement('canvas')
    offscreen.width = Math.floor(viewport.width)
    offscreen.height = Math.floor(viewport.height)

    const task = page.render({ canvas: offscreen, viewport })
    renderTask = task
    try {
      await task.promise
    } catch (err) {
      if (err?.name === 'RenderingCancelledException') return false
      throw err
    } finally {
      if (renderTask === task) renderTask = null
    }
    if (seq !== renderSeq) return false

    canvas.width = offscreen.width
    canvas.height = offscreen.height
    canvas.style.width = `${offscreen.width / dpr}px`
    canvas.style.height = `${offscreen.height / dpr}px`
    canvas.getContext('2d').drawImage(offscreen, 0, 0)
    // Free the offscreen backing store now instead of waiting for GC.
    offscreen.width = offscreen.height = 0
    hasPage = true
    return true
  }

  onMount(() => {
    // Re-render the current page at the new width when the pane resizes.
    let timer
    const observer = new ResizeObserver(() => {
      clearTimeout(timer)
      timer = setTimeout(() => {
        // An in-flight reload will render at the new width by itself.
        if (pdfDoc && !loading) {
          renderPage(pdfDoc, currentPageNumber).catch(
            (err) => (renderError = err?.message ?? String(err))
          )
        }
      }, RESIZE_DEBOUNCE_MS)
    })
    observer.observe(viewer)

    return () => {
      clearTimeout(timer)
      observer.disconnect()
      loadSeq++ // in-flight loads become stale and release their own document
      renderTask?.cancel() // cancel before destroying, as PDF.js requires
      void releasePdf(pdfDoc)
      pdfDoc = null
    }
  })
</script>

<div class="preview">
  <header class="bar">
    <span>
      Vista previa PDF
      {#if pageCount}<span class="pages">· página {currentPageNumber} de {pageCount}</span>{/if}
    </span>
    <span class="state-{compileState}">{LABELS[compileState]}</span>
  </header>

  <div class="viewer" bind:this={viewer}>
    <canvas bind:this={canvas} class:hidden={!hasPage} aria-label="Página {currentPageNumber} del PDF compilado"
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
