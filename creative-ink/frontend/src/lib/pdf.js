// PDF.js setup: offline worker bundled by Vite (no CDN).
import { GlobalWorkerOptions, getDocument } from 'pdfjs-dist'
// `?url` makes Vite emit the worker as a static asset and give us its URL,
// both in `wails dev` and inside the embedded production build.
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

GlobalWorkerOptions.workerSrc = workerUrl

/**
 * Loads a PDF served by the Go AssetServer middleware.
 *
 * The bytes are fetched explicitly and handed to `getDocument({ data })`
 * instead of `getDocument(url)`: Wails serves the app from a custom scheme
 * (wails://), where PDF.js' own range/streaming network layer is unreliable.
 * A single fetch with `no-store` is deterministic on macOS and Linux.
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
  return getDocument({ data }).promise
}
