// Integrated terminal state: the command log shown in TerminalPanel and the
// ↑/↓ history. It lives in a module so it survives tab switches.
//
// SECURITY: RunTerminalCommand executes arbitrary shell commands (app.go).
// Only ever pass it what the user typed in the terminal input.
import { CancelTerminalCommand, ReadTerminalOutput, RunTerminalCommand } from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'

const MAX_ENTRIES = 200 // older entries are dropped from the view
const MAX_HISTORY = 100

// Events emitted by Go (app.go).
export const TERMINAL_STARTED = 'terminal:started' // { runId, command }: output spooled to disk
export const TERMINAL_PROGRESS = 'terminal:progress' // { runId, pass, passes, tool, page, pages, elapsedMs, done, failed }

// "Salida completa" reads the spooled output from Go on demand: never more
// than this tail is kept in the DOM, polled every FULL_POLL_MS while the
// command runs and the view is open. Nothing crosses the bridge otherwise.
const FULL_TAIL_CHARS = 256 * 1024
const FULL_POLL_MS = 500

/**
 * @typedef {{
 *   id: number,
 *   command: string,
 *   output: string,
 *   status: 'running' | 'done' | 'error',
 *   exitCode: number | null,
 *   timedOut: boolean,
 *   canceled: boolean,
 *   durationMs: number,
 *   runId: number,
 *   progress: BuildProgress | null,
 *   full: FullOutput
 * }} TerminalEntry
 * @typedef {{ pass: number, passes: number, tool: string, page: number, pages: number, elapsedMs: number, done: boolean, failed: boolean }} BuildProgress
 * @typedef {{ open: boolean, text: string, next: number, size: number, done: boolean, truncated: boolean, loading: boolean, error: string }} FullOutput
 */

export const terminal = $state({
  /** @type {TerminalEntry[]} */
  entries: [],
  running: false
})

let nextId = 1
/** @type {string[]} newest last (not reactive: only read on ↑/↓) */
const history = []

/** Commands typed so far, oldest first, without consecutive duplicates. */
export function commandHistory() {
  return history.slice()
}

/**
 * Runs a command typed by the user. `clear`/`cls` only clears the view.
 * @param {string} input
 */
export async function runCommand(input) {
  const command = input.trim()
  if (!command || terminal.running) return
  if (history.at(-1) !== command) {
    history.push(command)
    if (history.length > MAX_HISTORY) history.shift()
  }
  if (command === 'clear' || command === 'cls') {
    terminal.entries = []
    return
  }

  terminal.entries.push({
    id: nextId++,
    command,
    output: '',
    status: 'running',
    exitCode: null,
    timedOut: false,
    canceled: false,
    durationMs: 0,
    runId: 0,
    progress: null,
    full: { open: false, text: '', next: 0, size: 0, done: false, truncated: false, loading: false, error: '' }
  })
  if (terminal.entries.length > MAX_ENTRIES) terminal.entries.shift()
  // Mutate the reactive proxy stored in the array, not the plain object.
  const entry = terminal.entries[terminal.entries.length - 1]

  terminal.running = true
  try {
    const result = await RunTerminalCommand(command)
    entry.output = result.output
    entry.exitCode = result.exitCode
    entry.timedOut = result.timedOut
    entry.canceled = result.canceled
    entry.durationMs = result.durationMs
    entry.runId = result.runId || entry.runId
    entry.status = 'done'
  } catch (err) {
    // Could not start (no folder open, another command running…).
    entry.output = String(err)
    entry.status = 'error'
  } finally {
    terminal.running = false
  }
}

/** Stops the running command (the Go side kills its whole process group). */
export function cancelCommand() {
  if (terminal.running) void CancelTerminalCommand()
}

/** The entry of the command in progress (at most one runs at a time). */
function runningEntry() {
  return terminal.entries.findLast((e) => e.status === 'running')
}

/**
 * Subscribes to terminal:started (the run's spool id) and terminal:progress
 * (passes and pages of `ci compile`). Call once during App init and call
 * the returned function on destroy.
 * @returns {() => void} unsubscribe
 */
export function listenTerminalEvents() {
  const offs = [
    EventsOn(TERMINAL_STARTED, (/** @type {{ runId: number }} */ event) => {
      const entry = runningEntry()
      if (entry) entry.runId = event.runId
    }),
    EventsOn(TERMINAL_PROGRESS, (/** @type {BuildProgress & { runId: number }} */ event) => {
      const entry = terminal.entries.findLast((e) => e.runId === event.runId)
      if (entry) entry.progress = { ...event }
    })
  ]
  return () => offs.forEach((off) => off())
}

/**
 * Shows or hides the full output of a run, read from Go's spool on disk in
 * chunks of up to 64 KB. While the run goes on, it polls every 500 ms.
 * @param {TerminalEntry} entry the reactive entry
 */
export function toggleFullOutput(entry) {
  if (!entry.runId) return
  entry.full.open = !entry.full.open
  if (entry.full.open) void pollFullOutput(entry)
}

/** @param {TerminalEntry} entry */
async function pollFullOutput(entry) {
  const full = entry.full
  if (full.loading) return
  full.loading = true
  full.error = ''
  try {
    while (full.open) {
      const chunk = await ReadTerminalOutput(entry.runId, full.next)
      // A huge output: start at its tail instead of downloading all of it.
      if (!full.text && full.next === 0 && chunk.size > FULL_TAIL_CHARS) {
        full.next = chunk.size - FULL_TAIL_CHARS
        full.truncated = true
        continue
      }
      if (chunk.data) {
        let text = full.text + chunk.data
        if (text.length > FULL_TAIL_CHARS) {
          text = text.slice(-FULL_TAIL_CHARS)
          full.truncated = true
        }
        full.text = text
      }
      full.next = chunk.next
      full.size = chunk.size
      full.done = chunk.done
      if (chunk.next < chunk.size) continue // more is already on disk
      if (chunk.done) break
      await new Promise((resolve) => setTimeout(resolve, FULL_POLL_MS))
    }
  } catch (err) {
    full.error = String(err) // e.g. an old run whose file was rotated out
  } finally {
    full.loading = false
  }
}
