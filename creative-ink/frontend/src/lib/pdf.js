// PDF.js setup and document lifecycle (pdfjs-dist 6.x).
//
// Lifecycle rules (verified against pdfjs-dist 6.5 source):
//  - PDFDocumentProxy has NO destroy() in v6 (only cleanup()). The owner of a
//    document is its PDFDocumentLoadingTask: `doc.loadingTask.destroy()`.
//  - Without the `worker` option, every getDocument() spawns its own Web
//    Worker owned by the task. We pass ONE shared PDFWorker instead, so
//    destroying a task frees that document (pages, fonts, worker-side state)
//    while the worker thread is reused across compilations.
import { GlobalWorkerOptions, PDFWorker, getDocument } from 'pdfjs-dist'
// `?url` makes Vite emit the worker as a static asset and give us its URL,
// both in `wails dev` and inside the embedded production build (offline).
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

GlobalWorkerOptions.workerSrc = workerUrl

/** @type {PDFWorker | null} */
let sharedWorker = null

function getSharedWorker() {
  if (!sharedWorker || sharedWorker.destroyed) {
    sharedWorker = new PDFWorker({ name: 'creative-ink-pdf' })
  }
  return sharedWorker
}

// Dev only: when Vite hot-reloads this module, terminate the old worker thread
// instead of orphaning it (importers remount and release their documents).
if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    sharedWorker?.destroy()
    sharedWorker = null
  })
}

/**
 * Loads a PDF served by the Go AssetServer middleware.
 *
 * The bytes are fetched explicitly and handed to `getDocument({ data })`:
 * Wails serves the app from a custom scheme (wails://), where PDF.js' own
 * range/streaming network layer is unreliable.
 *
 * The caller owns the returned document and must pass it to releasePdf().
 *
 * @param {string} url
 * @returns {Promise<import('pdfjs-dist').PDFDocumentProxy>}
 */
export async function loadPdf(url) {
  const response = await fetch(url, { cache: 'no-store' })
  if (!response.ok) {
    throw new Error(`No se pudo leer el PDF (HTTP ${response.status})`)
  }
  const data = new Uint8Array(await response.arrayBuffer())

  const task = getDocument({
    data,
    worker: getSharedWorker(),
    // Explicit main-thread rendering: v6 would otherwise try a second
    // "renderer worker" per document when GlobalWorkerOptions.rendererSrc is set.
    disableWorkerRendering: true
  })
  try {
    return await task.promise
  } catch (err) {
    // A corrupt/partial PDF still allocated worker-side state: free it.
    await task.destroy().catch(() => {})
    throw err
  }
}

/**
 * Releases a document and all its resources. Safe to call with null, twice,
 * or on an already destroyed document; never throws.
 *
 * Do not call it on a document that is still being rendered unless that
 * render was cancelled first (see Preview.svelte).
 *
 * @param {import('pdfjs-dist').PDFDocumentProxy | null | undefined} doc
 * @returns {Promise<void>}
 */
export async function releasePdf(doc) {
  const task = doc?.loadingTask
  if (!task || task.destroyed) return
  try {
    await task.destroy()
  } catch (err) {
    console.warn('[pdf] No se pudo liberar el documento:', err)
  }
}
