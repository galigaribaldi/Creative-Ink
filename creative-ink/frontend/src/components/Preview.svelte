<script>
  import { onMount, tick, untrack } from 'svelte'
  import { compilation, showDocumentPreview } from '../lib/compilation.svelte.js'
  import { config } from '../lib/config.svelte.js'
  import { loadPdf, releasePdf } from '../lib/pdf.js'
  import { sync, syncBackward } from '../lib/synctex.svelte.js'

  // Served by the Go AssetServer middleware (see preview.go: previewRoute).
  const PREVIEW_URL = '/__preview/document.pdf'
  // Horizontal padding of .viewer (keep in sync with the CSS below).
  const VIEWER_PADDING_PX = 16
  const RESIZE_DEBOUNCE_MS = 150
  const SAVE_SHORTCUT = navigator.platform.toUpperCase().includes('MAC') ? '⌘S' : 'Ctrl+S'

  /**
   * @type {{
   *   compileState: 'idle' | 'compiling' | 'success' | 'error',
   *   pdfVersion: number,
   *   documentKey: string
   * }}
   */
  let { compileState, pdfVersion, documentKey } = $props()

  // Which PDF is shown (the main file's when the project has one, or a
  // foreign one such as the `ci guide` guide).
  let pdfName = $derived(
    compilation.foreign
      ? compilation.foreign.title
      : (documentKey.split(/[\\/]/).pop() ?? '').replace(/\.tex$/i, '.pdf')
  )

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
      highlight = null
      rendered = { pageNumber: 0, cssScale: 1 }
    })
  })

  // Every successful compilation that rewrote the PDF publishes a new
  // immutable snapshot in Go (pdfVersion) → new URL → reload. An unchanged PDF
  // (same ModTime) keeps the same pdfVersion, so this effect does not re-run
  // and the canvas is not reloaded. PDF.js then fetches only the byte ranges
  // it needs from that snapshot, even while latexmk rewrites the live PDF.
  $effect(() => {
    if (!pdfVersion) return
    const url = `${PREVIEW_URL}?v=${pdfVersion}`
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
    // CSS pixels per PDF point: converts SyncTeX coordinates both ways.
    rendered = { pageNumber, cssScale: fitScale }
    hasPage = true
    return true
  }

  // ── SyncTeX ─────────────────────────────────────────────────────────────
  const HIGHLIGHT_MS = 1800
  const HIGHLIGHT_PAD_PX = 2
  /** Page and scale of what the canvas shows now (non-reactive). */
  let rendered = { pageNumber: 0, cssScale: 1 }
  /** @type {{ left: number, top: number, width: number, height: number, key: number } | null} */
  let highlight = $state(null)
  /** @type {HTMLDivElement | undefined} */
  let highlightEl = $state()
  let highlightTimer

  // Forward search (Editor → PDF): jump to the page and flash the box.
  $effect(() => {
    const target = sync.highlight
    if (target) untrack(() => void showSyncHighlight(target))
  })

  /** @param {import('../lib/synctex.svelte.js').SyncHighlight} target */
  async function showSyncHighlight(target) {
    sync.highlight = null
    if (!pdfDoc) return
    const page = clampPage(target.page, pdfDoc.numPages)
    if (page !== rendered.pageNumber) {
      currentPageNumber = page
      if (!(await renderPage(pdfDoc, page))) return
    }
    const s = rendered.cssScale
    highlight = {
      left: target.x * s - HIGHLIGHT_PAD_PX,
      top: target.y * s - HIGHLIGHT_PAD_PX,
      width: Math.max(target.width * s, 8) + 2 * HIGHLIGHT_PAD_PX,
      height: Math.max(target.height * s, 8) + 2 * HIGHLIGHT_PAD_PX,
      key: target.nonce // restarts the CSS animation
    }
    clearTimeout(highlightTimer)
    highlightTimer = setTimeout(() => (highlight = null), HIGHLIGHT_MS)
    await tick()
    highlightEl?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }

  // ── Page navigation (header controls) ──────────────────────────────────
  let canGoBack = $derived(hasPage && currentPageNumber > 1)
  let canGoForward = $derived(hasPage && currentPageNumber < pageCount)

  /**
   * Shows page n (clamped). While a reload is in flight, it only moves
   * currentPageNumber: the reload then draws that page itself.
   * @param {number} n 1-based
   */
  async function goToPage(n) {
    if (!pdfDoc) return
    const page = clampPage(n, pdfDoc.numPages)
    if (page === currentPageNumber && rendered.pageNumber === page) return
    currentPageNumber = page
    if (loading) return
    try {
      if (await renderPage(pdfDoc, page)) viewer.scrollTop = 0
    } catch (err) {
      renderError = err?.message ?? String(err)
    }
  }

  // Backward search (PDF → Editor): double-click converts the CSS position to
  // PDF points with the scale the page was drawn at.
  /** @param {MouseEvent} event */
  function ondblclick(event) {
    if (!hasPage || !rendered.pageNumber || compilation.foreign) return // SyncTeX only maps the document
    const rect = canvas.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rendered.cssScale
    const y = (event.clientY - rect.top) / rendered.cssScale
    void syncBackward(rendered.pageNumber, x, y)
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
    <span class="title">
      Vista previa{#if pdfName}<span class="pdf-name" title={compilation.foreign?.file ?? documentKey}> · {pdfName}</span>{/if}
      {#if compilation.foreign}
        <button class="back" onclick={showDocumentPreview} title="Volver al PDF del documento">Volver al documento</button>
      {/if}
    </span>
    {#if pageCount}
      <!-- Chevron paths in the Lucide style (ISC licence, lucide.dev). -->
      <nav class="pager" aria-label="Páginas del PDF">
        <button
          onclick={() => goToPage(currentPageNumber - 1)}
          disabled={!canGoBack}
          aria-label="Página anterior"
          title="Página anterior"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>
        </button>
        <span class="pages" aria-live="polite">Página {currentPageNumber} de {pageCount}</span>
        <button
          onclick={() => goToPage(currentPageNumber + 1)}
          disabled={!canGoForward}
          aria-label="Página siguiente"
          title="Página siguiente"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6" /></svg>
        </button>
      </nav>
    {/if}
    {#if sync.notice}<span class="sync-notice" role="status">{sync.notice}</span>{/if}
    {#if compileState === 'success' && compilation.cached}
      <span class="state-cached" title={compilation.logs}>Último PDF guardado · sin compilar</span>
    {:else}
      <span class="state-{compileState}">{LABELS[compileState]}</span>
    {/if}
  </header>

  <div class="viewer" bind:this={viewer}>
    <div class="page" class:hidden={!hasPage}>
      <!-- Double-click: SyncTeX backward (keyboard alternative: ⌘⌥J/Ctrl+Alt+J
           works the other way, from the editor). -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <canvas
        bind:this={canvas}
        {ondblclick}
        title="Doble clic para ir al código fuente"
        aria-label="Página {currentPageNumber} del PDF compilado"
      ></canvas>
      {#if highlight}
        {#key highlight.key}
          <div
            bind:this={highlightEl}
            class="sync-highlight"
            style:left="{highlight.left}px"
            style:top="{highlight.top}px"
            style:width="{highlight.width}px"
            style:height="{highlight.height}px"
            aria-hidden="true"
          ></div>
        {/key}
      {/if}
    </div>
    {#if !hasPage}
      <p class="placeholder">
        {#if compileState === 'compiling'}
          Compilando el documento…
        {:else if !config.autoCompile}
          La compilación automática está desactivada: pulsa {SAVE_SHORTCUT} para compilar y ver el PDF.
        {:else}
          Escribe un documento LaTeX válido para ver el PDF.
        {/if}
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
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.25rem 0.75rem;
    border-bottom: 1px solid var(--border);
    background: var(--surface-base);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .pdf-name {
    color: var(--text-secondary);
  }

  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .back {
    margin-left: 0.5rem;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    text-decoration: underline transparent;
    text-underline-offset: 2px;
    cursor: pointer;
    transition: color var(--transition), text-decoration-color var(--transition);
  }

  .back:hover {
    color: var(--text-primary);
    text-decoration-color: currentColor;
  }

  .back:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .pager {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 0.15rem;
  }

  /* Fixed width: changing pages never shifts the bar (A1). */
  .pages {
    min-width: 7.5rem;
    text-align: center;
    font-variant-numeric: tabular-nums;
    color: var(--text-secondary);
  }

  .pager button {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  .pager button:hover:not(:disabled) {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .pager button:active:not(:disabled) {
    background: var(--surface-active);
  }

  .pager button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .pager button:disabled {
    color: var(--text-disabled);
    cursor: default;
  }

  .pager svg {
    width: var(--icon-size);
    height: var(--icon-size);
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  /* Quiet success, visible problems (A1). */
  .state-success {
    color: var(--text-muted);
  }

  .state-compiling {
    color: var(--text-secondary);
  }

  .state-error {
    color: var(--danger);
  }

  .state-cached {
    color: var(--text-muted);
    cursor: help;
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
    background: var(--surface-editor);
  }

  /* The sheet: soft page shadow and the medium radius (A3). */
  .page {
    position: relative;
    flex-shrink: 0;
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-page);
    overflow: hidden;
  }

  .page.hidden {
    display: none;
  }

  canvas {
    display: block;
    background: var(--paper);
    cursor: text;
  }

  /* SyncTeX highlight (A2, accent rule 3): accent border, soft accent fill,
     fade-out. 1.8 s is the time to find the box, not an interaction. */
  .sync-highlight {
    position: absolute;
    border-radius: var(--radius-sm);
    background: var(--accent-soft);
    outline: 2px solid var(--accent);
    pointer-events: none;
    animation: sync-fade 1.8s ease-out forwards;
  }

  @keyframes sync-fade {
    0%,
    60% {
      opacity: 1;
    }
    100% {
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .sync-highlight {
      animation: none;
    }
  }

  .sync-notice {
    margin-left: auto;
    margin-right: 0.75rem;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .placeholder {
    margin: auto 0;
    padding: 1rem;
    text-align: center;
    font-size: var(--text-md);
    color: var(--text-muted);
  }

  .render-error {
    margin: 0;
    padding: 0.35rem 0.75rem;
    border-top: 1px solid var(--border);
    background: var(--surface-base);
    font-size: var(--text-sm);
    color: var(--warning);
  }
</style>
