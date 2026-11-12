<script>
  // Continuous PDF view with zoom (docs/MAP.md §4.4, #9, block B2): pdf.js'
  // own PDFViewer (pdfjs-dist/web, already installed: no new dependency).
  // Pages are virtualized (only the visible ones are drawn, in the renderer
  // worker), there is a text layer (select and copy text) and internal
  // links work; external links are disabled (they would navigate the
  // WebView away).
  //
  // Reloads without flicker: two viewers are stacked. A new PDF loads in
  // the hidden one, at the same page, offset and zoom; when its visible
  // page is drawn, they swap and the old document is released. Exactly one
  // document per viewer; ownership as in Preview.svelte (releasePdf).
  import { onMount, tick, untrack } from 'svelte'
  import 'pdfjs-dist/web/pdf_viewer.css'
  import { AnnotationMode } from 'pdfjs-dist'
  import { loadPdf, releasePdf } from '../lib/pdf.js'
  import { sync, syncBackward } from '../lib/synctex.svelte.js'

  /**
   * url: the PDF to show ('' = none). syncEnabled: SyncTeX maps this PDF
   * (not for a foreign one such as the ci guide).
   * @type {{
   *   url: string,
   *   documentKey: string,
   *   syncEnabled: boolean,
   *   pageCount?: number,
   *   currentPage?: number,
   *   zoomLabel?: string,
   *   error?: string
   * }}
   */
  let {
    url,
    documentKey,
    syncEnabled,
    pageCount = $bindable(0),
    currentPage = $bindable(1),
    zoomLabel = $bindable(''),
    error = $bindable('')
  } = $props()

  const RESIZE_DEBOUNCE_MS = 150
  const SWAP_TIMEOUT_MS = 2000 // swap anyway if the first page takes longer
  const HIGHLIGHT_MS = 1800
  const HIGHLIGHT_PAD_PX = 2

  /** The zoom the user chose: 'page-width', 'page-fit' or a number (1 = 100 %). */
  let scaleValue = 'page-width'

  /** @type {HTMLDivElement} */
  let hostA
  /** @type {HTMLDivElement} */
  let hostB
  /** @type {HTMLDivElement} */
  let innerA
  /** @type {HTMLDivElement} */
  let innerB

  /**
   * @typedef {{ host: HTMLDivElement, viewer: any, eventBus: any, linkService: any,
   *   doc: import('pdfjs-dist').PDFDocumentProxy | null }} Buffer
   */
  /** @type {Buffer[]} */
  let buffers = []
  let front = $state(0) // index of the visible buffer
  let ready = $state(false) // pdf_viewer loaded
  let loadSeq = 0

  /** @type {{ left: number, top: number, width: number, height: number, key: number, buffer: number } | null} */
  let highlight = $state(null)
  let highlightTimer

  onMount(() => {
    let disposed = false
    /** @type {ResizeObserver | undefined} */
    let observer
    let resizeTimer
    ;(async () => {
      // Loaded after pdfjs-dist (lib/pdf.js), which defines globalThis.pdfjsLib
      // that pdf_viewer.mjs reads when it is evaluated.
      const { EventBus, PDFLinkService, PDFViewer } = await import('pdfjs-dist/web/pdf_viewer.mjs')
      if (disposed) return
      buffers = [
        [hostA, innerA],
        [hostB, innerB]
      ].map(([host, inner], index) => {
        const eventBus = new EventBus()
        const linkService = new PDFLinkService({ eventBus })
        linkService.externalLinkEnabled = false // never navigate the WebView
        const viewer = new PDFViewer({
          container: host,
          viewer: inner,
          eventBus,
          linkService,
          textLayerMode: 1, // select and copy text
          annotationMode: AnnotationMode.ENABLE // links, no form editing
        })
        linkService.setViewer(viewer)
        // Only the visible buffer drives the header (page, zoom).
        eventBus.on('pagechanging', (/** @type {{ pageNumber: number }} */ e) => {
          if (index === front) currentPage = e.pageNumber
        })
        eventBus.on('scalechanging', (/** @type {{ scale: number }} */ e) => {
          if (index === front) zoomLabel = `${Math.round(e.scale * 100)} %`
        })
        return { host, viewer, eventBus, linkService, doc: null }
      })
      ready = true

      // 'page-width'/'page-fit' follow the pane's size (pdf.js does not on its own).
      observer = new ResizeObserver(() => {
        clearTimeout(resizeTimer)
        resizeTimer = setTimeout(() => {
          const buf = buffers[front]
          if (buf?.doc && buf.host.clientWidth > 0 && typeof scaleValue === 'string') {
            buf.viewer.currentScaleValue = scaleValue
          }
        }, RESIZE_DEBOUNCE_MS)
      })
      observer.observe(hostA)
    })().catch((err) => (error = err?.message ?? String(err)))

    return () => {
      disposed = true
      clearTimeout(resizeTimer)
      observer?.disconnect()
      loadSeq++
      for (const buf of buffers) clear(buf)
    }
  })

  /** Detaches and frees a buffer's document. @param {Buffer} buf */
  function clear(buf) {
    if (!buf.doc) return
    buf.viewer.setDocument(null) // cancels its renders first, as pdf.js requires
    void releasePdf(buf.doc)
    buf.doc = null
  }

  // Another document (another .tex): start again at page 1, original zoom kept.
  let shownDocument = ''
  $effect(() => {
    const key = documentKey
    untrack(() => {
      if (key === shownDocument) return
      shownDocument = key
      loadSeq++
      for (const buf of buffers) clear(buf)
      pageCount = 0
      currentPage = 1
      highlight = null
    })
  })

  $effect(() => {
    const target = url
    if (ready && target) untrack(() => void reload(target))
  })

  /** Waits for one event of a buffer (or the timeout). */
  function once(/** @type {any} */ eventBus, /** @type {string} */ name, timeoutMs = SWAP_TIMEOUT_MS) {
    return new Promise((resolve) => {
      const done = () => {
        clearTimeout(timer)
        eventBus.off(name, done)
        resolve(undefined)
      }
      const timer = setTimeout(done, timeoutMs)
      eventBus.on(name, done)
    })
  }

  /** Where the visible buffer is: page and the fraction of it scrolled past. */
  function position() {
    const buf = buffers[front]
    if (!buf?.doc) return { page: currentPage, offset: 0, left: 0 }
    const page = buf.viewer.currentPageNumber
    const div = buf.viewer.getPageView(page - 1)?.div
    const offset = div ? (buf.host.scrollTop - div.offsetTop) / Math.max(div.clientHeight, 1) : 0
    return { page, offset: Math.max(0, offset), left: buf.host.scrollLeft }
  }

  async function reload(/** @type {string} */ target) {
    const seq = ++loadSeq
    /** @type {import('pdfjs-dist').PDFDocumentProxy | null} */
    let doc = null
    try {
      doc = await loadPdf(target)
      if (seq !== loadSeq) return
      const pos = position()
      const back = buffers[1 - front]
      clear(back)
      back.doc = doc
      doc = null // owned by the buffer now
      back.linkService.setDocument(back.doc)
      const inited = once(back.eventBus, 'pagesinit')
      back.viewer.setDocument(back.doc)
      await inited
      if (seq !== loadSeq) return
      // Hidden preview (no width): the ResizeObserver applies the zoom when shown.
      if (back.host.clientWidth > 0) back.viewer.currentScaleValue = String(scaleValue)
      const page = Math.min(Math.max(1, pos.page), back.doc.numPages)
      const rendered = once(back.eventBus, 'pagerendered')
      await tick()
      const div = back.viewer.getPageView(page - 1)?.div
      if (div) back.host.scrollTop = div.offsetTop + pos.offset * div.clientHeight
      back.host.scrollLeft = pos.left
      await rendered
      if (seq !== loadSeq) return

      // Swap: the new document is on screen; the old one is released.
      const old = buffers[front]
      front = 1 - front
      pageCount = back.doc.numPages
      currentPage = back.viewer.currentPageNumber
      zoomLabel = `${Math.round(back.viewer.currentScale * 100)} %`
      error = ''
      clear(old)
    } catch (err) {
      // The visible buffer still shows the last valid PDF.
      if (seq === loadSeq) error = err?.message ?? String(err)
    } finally {
      if (doc) void releasePdf(doc)
    }
  }

  // ── Header controls (Preview.svelte calls these) ─────────────────────────
  export function goToPage(/** @type {number} */ n) {
    const buf = buffers[front]
    if (buf?.doc) buf.viewer.currentPageNumber = Math.min(Math.max(1, n), buf.doc.numPages)
  }

  /** 'page-width', 'page-fit', or a factor (1 = 100 %). @param {string | number} value */
  export function setZoom(value) {
    scaleValue = value
    const buf = buffers[front]
    if (buf?.doc) buf.viewer.currentScaleValue = String(value)
  }

  export function zoomStep(/** @type {1 | -1} */ dir) {
    const buf = buffers[front]
    if (!buf?.doc) return
    if (dir > 0) buf.viewer.increaseScale()
    else buf.viewer.decreaseScale()
    scaleValue = buf.viewer.currentScale // a number now: no longer follows the width
  }

  // ── SyncTeX ──────────────────────────────────────────────────────────────
  // Forward (Editor → PDF): scroll to the box and flash it.
  $effect(() => {
    const target = sync.highlight
    if (target && syncEnabled) untrack(() => showSyncHighlight(target))
  })

  /** @param {import('../lib/synctex.svelte.js').SyncHighlight} target */
  async function showSyncHighlight(target) {
    sync.highlight = null
    const buf = buffers[front]
    if (!buf?.doc) return
    const page = Math.min(Math.max(1, target.page), buf.doc.numPages)
    buf.viewer.scrollPageIntoView({ pageNumber: page })
    await tick()
    const view = buf.viewer.getPageView(page - 1)
    if (!view?.div) return
    // SyncTeX gives PDF points from the top-left corner; pdf.js wants them
    // from the bottom-left one (viewBox[3] is the page's top).
    const top = view.viewport.viewBox[3]
    const [x1, y1, x2, y2] = view.viewport.convertToViewportRectangle([
      target.x,
      top - (target.y + target.height),
      target.x + target.width,
      top - target.y
    ])
    const left = view.div.offsetLeft + Math.min(x1, x2)
    const boxTop = view.div.offsetTop + Math.min(y1, y2)
    highlight = {
      left: left - HIGHLIGHT_PAD_PX,
      top: boxTop - HIGHLIGHT_PAD_PX,
      width: Math.max(Math.abs(x2 - x1), 8) + 2 * HIGHLIGHT_PAD_PX,
      height: Math.max(Math.abs(y2 - y1), 8) + 2 * HIGHLIGHT_PAD_PX,
      key: target.nonce, // restarts the CSS animation
      buffer: front
    }
    buf.host.scrollTo({ top: Math.max(0, boxTop - buf.host.clientHeight / 2), behavior: 'smooth' })
    clearTimeout(highlightTimer)
    highlightTimer = setTimeout(() => (highlight = null), HIGHLIGHT_MS)
  }

  // Backward (PDF → Editor): double click on a page.
  /** @param {MouseEvent} event @param {number} index */
  function ondblclick(event, index) {
    const buf = buffers[index]
    if (!syncEnabled || index !== front || !buf?.doc) return
    const pageEl = /** @type {HTMLElement | null} */ (/** @type {HTMLElement} */ (event.target).closest('.page'))
    const pageNumber = Number(pageEl?.dataset.pageNumber)
    if (!pageEl || !pageNumber) return
    const view = buf.viewer.getPageView(pageNumber - 1)
    const rect = pageEl.getBoundingClientRect()
    const [x, y] = view.viewport.convertToPdfPoint(event.clientX - rect.left, event.clientY - rect.top)
    void syncBackward(pageNumber, x, view.viewport.viewBox[3] - y)
  }
</script>

<div class="continuous">
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="host" class:front={front === 0} bind:this={hostA} ondblclick={(e) => ondblclick(e, 0)} aria-hidden={front !== 0}>
    <div class="pdfViewer" bind:this={innerA}></div>
    {@render box(0)}
  </div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="host" class:front={front === 1} bind:this={hostB} ondblclick={(e) => ondblclick(e, 1)} aria-hidden={front !== 1}>
    <div class="pdfViewer" bind:this={innerB}></div>
    {@render box(1)}
  </div>
</div>

{#snippet box(/** @type {number} */ index)}
  {#if highlight && highlight.buffer === index}
    {#key highlight.key}
      <div
        class="sync-highlight"
        style:left="{highlight.left}px"
        style:top="{highlight.top}px"
        style:width="{highlight.width}px"
        style:height="{highlight.height}px"
        aria-hidden="true"
      ></div>
    {/key}
  {/if}
{/snippet}

<style>
  .continuous {
    position: relative;
    flex: 1;
    min-height: 0;
  }

  /* pdf.js needs absolutely positioned containers. The hidden one keeps its
     size (visibility, not display) so the next PDF lays out at the same
     width while it loads behind. */
  .host {
    position: absolute;
    inset: 0;
    overflow: auto;
    visibility: hidden;
    background: var(--surface-editor);
  }

  .host.front {
    visibility: visible;
  }

  /* The sheet is the document, not chrome: white in both themes (B6). */
  /* Same sheet as the single-page view: soft shadow, medium radius (A3). */
  .host :global(.pdfViewer .page) {
    margin: 16px auto;
    border: 0;
    border-radius: var(--radius-md);
    background-color: var(--paper);
    box-shadow: var(--shadow-page);
    overflow: hidden;
  }

  /* SyncTeX highlight (A2, accent rule 3), as in Preview.svelte. */
  .sync-highlight {
    position: absolute;
    z-index: 5;
    border-radius: var(--radius-sm);
    background: var(--accent-soft);
    outline: 2px solid var(--accent);
    pointer-events: none;
    animation: fade 1.8s ease-out forwards;
  }

  @media (prefers-reduced-motion: reduce) {
    .sync-highlight {
      animation: none;
    }
  }

  @keyframes fade {
    0%,
    60% {
      opacity: 1;
    }
    100% {
      opacity: 0;
    }
  }
</style>
