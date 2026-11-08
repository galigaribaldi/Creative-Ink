// Integrated terminal state: the command log shown in TerminalPanel and the
// ↑/↓ history. It lives in a module so it survives tab switches.
//
// SECURITY: RunTerminalCommand executes arbitrary shell commands (app.go).
// Only ever pass it what the user typed in the terminal input.
import { CancelTerminalCommand, RunTerminalCommand } from '../../wailsjs/go/main/App.js'

const MAX_ENTRIES = 200 // older entries are dropped from the view
const MAX_HISTORY = 100

/**
 * @typedef {{
 *   id: number,
 *   command: string,
 *   output: string,
 *   status: 'running' | 'done' | 'error',
 *   exitCode: number | null,
 *   timedOut: boolean,
 *   canceled: boolean,
 *   durationMs: number
 * }} TerminalEntry
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
    durationMs: 0
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
