// Compilation state mirrored from Go events (docs/MAP.md §3). Go is the
// source of truth; this module only reflects the last event received so that
// any component (ActionBar, Workspace, Preview) can read it without props.
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { compileTarget } from './workspace.svelte.js'

// Event names emitted by Go (app.go: EventCompileStarted / EventCompileFinished).
export const COMPILE_STARTED = 'compile:started'
export const COMPILE_FINISHED = 'compile:finished'

/** @typedef {'idle' | 'compiling' | 'success' | 'error'} CompileStatus */

// Adaptive auto-compile (docs/MAP.md §4.5). The idle delay before an
// automatic compilation follows the last measured duration, so a slow
// document is not recompiled back to back while the user types.
export const BASE_COMPILE_DELAY_MS = 3000
// Above this, auto-compile pauses for the current target: only Cmd/Ctrl+S
// (or the toolbar) compiles, until a compilation finishes under it again.
export const HEAVY_COMPILE_MS = 10000

/** newDelay = max(3000, durationMs × 2) */
export function adaptiveDelay(/** @type {number} */ durationMs) {
  return Math.max(BASE_COMPILE_DELAY_MS, durationMs * 2)
}

export const compilation = $state({
  /** @type {CompileStatus} */
  status: 'idle',
  logs: '',
  durationMs: 0,
  // Immutable PDF snapshot to show (Go's pdfVersion). A new version reloads
  // the canvas; 0 = nothing compiled yet for this target.
  pdfVersion: 0,
  // ModTime (Unix ms) of the PDF behind pdfVersion: an identical ModTime
  // means latexmk did not rewrite the PDF, so the canvas is not reloaded.
  pdfModTime: 0,
  // Read by Editor.svelte each time it schedules an automatic compilation.
  autoCompileDelayMs: BASE_COMPILE_DELAY_MS,
  autoCompilePaused: false,
  // The PDF shown was not compiled now: it is the one already on disk
  // (project opened with compile.auto off).
  cached: false
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
      compilation.cached = false
    }),
    /** @param {{ file: string, success: boolean, logs: string, durationMs: number, pdfVersion: number, pdfModTime: number, canceled: boolean, cached: boolean }} result */
    EventsOn(COMPILE_FINISHED, (result) => {
      // A result for another document (e.g. the file that was active before
      // a switch) must not touch the preview: it is not the PDF shown.
      if (result.file !== compileTarget()) return
      // An obsolete automatic run was cancelled and the next one already
      // started (Go's bounded cancellation): stay in "compiling".
      if (result.canceled) return
      compilation.logs = result.logs
      compilation.cached = result.cached === true

      // Adaptive trigger: a failed run costs CPU too, so every real result
      // counts. A cached preview compiled nothing: it measures nothing.
      if (!result.cached) {
        compilation.durationMs = result.durationMs
        compilation.autoCompileDelayMs = adaptiveDelay(result.durationMs)
        compilation.autoCompilePaused = result.durationMs > HEAVY_COMPILE_MS
      }

      if (result.success) {
        compilation.status = 'success'
        // Reload only for a different PDF. Go already reuses pdfVersion when
        // the PDF was not rewritten; the ModTime check is a second guard.
        // 0 means Go could not snapshot the PDF: keep showing the last one.
        const unchanged = result.pdfModTime !== 0 && result.pdfModTime === compilation.pdfModTime
        if (result.pdfVersion && !unchanged) {
          compilation.pdfVersion = result.pdfVersion
          compilation.pdfModTime = result.pdfModTime
        }
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
  // A different target (another project or file): forget its PDF stamp and
  // start the adaptive trigger over ("paused" was for the heavy document).
  compilation.pdfModTime = 0
  compilation.autoCompileDelayMs = BASE_COMPILE_DELAY_MS
  compilation.autoCompilePaused = false
  compilation.cached = false
}
