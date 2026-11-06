// Compilation state mirrored from Go events (docs/MAP.md §3). Go is the
// source of truth; this module only reflects the last event received so that
// any component (ActionBar, Workspace, Preview) can read it without props.
import { EventsOn } from '../../wailsjs/runtime/runtime.js'

// Event names emitted by Go (app.go: EventCompileStarted / EventCompileFinished).
export const COMPILE_STARTED = 'compile:started'
export const COMPILE_FINISHED = 'compile:finished'

/** @typedef {'idle' | 'compiling' | 'success' | 'error'} CompileStatus */

export const compilation = $state({
  /** @type {CompileStatus} */
  status: 'idle',
  logs: '',
  durationMs: 0,
  // Bumped after every successful compilation to reload the PDF canvas.
  pdfTimestamp: 0
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
    EventsOn(COMPILE_STARTED, () => {
      compilation.status = 'compiling'
    }),
    /** @param {{ success: boolean, logs: string, durationMs: number }} result */
    EventsOn(COMPILE_FINISHED, (result) => {
      compilation.logs = result.logs
      compilation.durationMs = result.durationMs
      if (result.success) {
        compilation.status = 'success'
        compilation.pdfTimestamp = Date.now()
      } else {
        // pdfTimestamp is left untouched, so the canvas keeps the last valid PDF.
        compilation.status = 'error'
      }
    })
  ]
  return () => offs.forEach((off) => off())
}
