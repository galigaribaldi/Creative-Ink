// Compilation state mirrored from Go events (docs/MAP.md §3). Go is the
// source of truth; this module only reflects the last event received so that
// any component (ActionBar, Workspace, Preview) can read it without props.
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { compileTarget } from './workspace.svelte.js'

// Event names emitted by Go (app.go: EventCompileStarted / EventCompileFinished).
export const COMPILE_STARTED = 'compile:started'
export const COMPILE_FINISHED = 'compile:finished'

/** @typedef {'idle' | 'compiling' | 'success' | 'error'} CompileStatus */

export const compilation = $state({
  /** @type {CompileStatus} */
  status: 'idle',
  logs: '',
  durationMs: 0,
  // Immutable PDF snapshot to show (Go's pdfVersion). A new version reloads
  // the canvas; 0 = nothing compiled yet for this target.
  pdfVersion: 0
})

/**
 * Subscribes to the Go compilation events. Call it once, during App init
 * (before the Editor triggers its first save), and call the returned
 * function on destroy so hot reloads do not stack listeners.
 *
 * @returns {() => void} unsubscribe
 */
export function listenCompileEvents() {
  const offs = [
    /** @param {{ file: string }} event */
    EventsOn(COMPILE_STARTED, (event) => {
      if (event.file !== compileTarget()) return
      compilation.status = 'compiling'
    }),
    /** @param {{ file: string, success: boolean, logs: string, durationMs: number, pdfVersion: number }} result */
    EventsOn(COMPILE_FINISHED, (result) => {
      // A result for another document (e.g. the file that was active before
      // a switch) must not touch the preview: it is not the PDF shown.
      if (result.file !== compileTarget()) return
      compilation.logs = result.logs
      compilation.durationMs = result.durationMs
      if (result.success) {
        compilation.status = 'success'
        // 0 means Go could not snapshot the PDF: keep showing the last one.
        if (result.pdfVersion) compilation.pdfVersion = result.pdfVersion
      } else {
        // pdfVersion is left untouched, so the canvas keeps the last valid PDF.
        compilation.status = 'error'
      }
    })
  ]
  return () => offs.forEach((off) => off())
}

/** Clears the state shown for the previous document (on file switch). */
export function resetCompilation() {
  compilation.status = 'idle'
  compilation.logs = ''
  compilation.durationMs = 0
}
