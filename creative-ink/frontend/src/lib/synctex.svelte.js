// SyncTeX: bidirectional navigation between the source (CodeMirror) and the
// PDF (Preview). Go runs the synctex CLI; this module only carries the
// answers between the two components, without prop drilling.
//
// Coordinates are PDF points (1/72 in) from the page's top-left corner.
import { SyncTexBackward, SyncTexForward } from '../../wailsjs/go/main/App.js'
import { openFile } from './actions.js'

const NOTICE_MS = 4000

/**
 * @typedef {{ page: number, x: number, y: number, width: number, height: number, nonce: number }} SyncHighlight
 * @typedef {{ file: string, line: number, nonce: number }} SyncReveal
 */

export const sync = $state({
  /** Forward result for Preview: jump to the page and flash this box. @type {SyncHighlight | null} */
  highlight: null,
  /** Backward result for Editor: open the file and scroll to the line. @type {SyncReveal | null} */
  reveal: null,
  /** Subtle hint when SyncTeX has no answer (e.g. not compiled yet). */
  notice: ''
})

let nonce = 0
let noticeTimer

function notify(/** @type {string} */ message) {
  sync.notice = message
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => (sync.notice = ''), NOTICE_MS)
}

/**
 * Source → PDF: where `line` of `file` appears in the compiled PDF.
 * @param {string} file absolute .tex path
 * @param {number} line 1-based
 */
export async function syncForward(file, line) {
  if (!file) return
  try {
    const result = await SyncTexForward(line, file)
    if (!result.found) return notify(result.reason)
    // A new nonce re-triggers Preview even for the same box twice in a row.
    sync.highlight = { ...result, nonce: ++nonce }
  } catch (err) {
    notify(String(err))
  }
}

/**
 * PDF → source: open the .tex that produced (x, y) of `page`, at its line.
 * @param {number} page 1-based
 * @param {number} x points from the left edge
 * @param {number} y points from the top edge
 */
export async function syncBackward(page, x, y) {
  try {
    const result = await SyncTexBackward(page, x, y)
    if (!result.found) return notify(result.reason)
    sync.reveal = { file: result.file, line: result.line, nonce: ++nonce }
    // The pane already showing that file reveals the line; otherwise the
    // focused pane saves its file, loads this one and then reveals it.
    openFile(result.file)
  } catch (err) {
    notify(String(err))
  }
}
