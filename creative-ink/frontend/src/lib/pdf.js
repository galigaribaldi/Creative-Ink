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

// Bytes per HTTP Range request (PDF.js' default, made explicit).
const RANGE_CHUNK_SIZE = 65536

/**
 * Opens a PDF served by the Go AssetServer middleware, loading it lazily
 * with HTTP Range requests: PDF.js downloads only the chunks needed for the
 * page being rendered, so a 200 MB thesis does not land in RAM at once.
 *
 * - `disableAutoFetch` stops background pre-fetching of the rest of the
 *   file. Per the pdfjs-dist 6 docs it only works together with
 *   `disableStream: true` (streaming would download the whole file anyway).
 * - Transport: on macOS/Linux the app runs on wails://, which is not
 *   http(s), so PDF.js uses XMLHttpRequest (PDFNetworkStream); on Windows
 *   (http://wails.localhost) it uses fetch. Wails forwards the Range header
 *   and the 206 answer on macOS, Windows and Linux with WebKitGTK ≥ 2.36
 *   (webkit2_41 tag). On older WebKitGTK no headers cross the bridge: PDF.js
 *   sees no Accept-Ranges and falls back to one full download.
 * - The URL carries `?v=<pdfVersion>`: an immutable snapshot, so ranges read
 *   later always belong to the same PDF.
 *
 * The caller owns the returned document and must pass it to releasePdf().
 *
 * @param {string} url
 * @returns {Promise<import('pdfjs-dist').PDFDocumentProxy>}
 */
export async function loadPdf(url) {
  const task = getDocument({
    url,
    rangeChunkSize: RANGE_CHUNK_SIZE,
    disableAutoFetch: true,
    disableStream: true,
    worker: getSharedWorker(),
    // Explicit main-thread rendering: v6 would otherwise try a second
    // "renderer worker" per document when GlobalWorkerOptions.rendererSrc is set.
    disableWorkerRendering: true
  })
  try {
    return await task.promise
  } catch (err) {
    // Aborts pending range requests and frees worker-side state.
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
