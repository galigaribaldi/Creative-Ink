// The sidebar's "Buscar" (docs/MAP.md §4.4, #9): Go searches the project's
// text files (SearchProject); this module keeps the query and the last
// results while the user switches tools, and opens a match in the editor.
import { SearchProject } from '../../wailsjs/go/main/App.js'
import { openFile } from './actions.js'
import { sync } from './synctex.svelte.js'
import { toggleTool, ui } from './ui.svelte.js'

const SEARCH_DELAY_MS = 250 // after the last keystroke

/**
 * @typedef {{ line: number, column: number, length: number, preview: string, start: number, end: number }} SearchMatch
 * @typedef {{ path: string, rel: string, matches: SearchMatch[] }} SearchFile
 */

export const search = $state({
  text: '',
  caseSensitive: false,
  wholeWord: false,
  regex: false,
  /** @type {SearchFile[]} */
  files: [],
  total: 0,
  truncated: false,
  error: '',
  busy: false,
  /** Bumped by ⇧⌘F so the panel focuses its input. */
  focusNonce: 0
})

let timer = 0
let seq = 0 // only the latest answer is shown

/** Re-runs the search shortly after the last change (typing, toggles). */
export function scheduleSearch() {
  clearTimeout(timer)
  timer = setTimeout(runSearch, SEARCH_DELAY_MS)
}

export async function runSearch() {
  clearTimeout(timer)
  const mine = ++seq
  if (!search.text.trim()) {
    search.files = []
    search.total = 0
    search.truncated = false
    search.error = ''
    return
  }
  search.busy = true
  try {
    const res = await SearchProject({
      text: search.text,
      caseSensitive: search.caseSensitive,
      wholeWord: search.wholeWord,
      regex: search.regex
    })
    if (mine !== seq) return
    search.files = res.files ?? []
    search.total = res.total
    search.truncated = res.truncated
    search.error = ''
  } catch (err) {
    if (mine !== seq) return
    search.files = []
    search.total = 0
    search.error = String(err)
  } finally {
    if (mine === seq) search.busy = false
  }
}

/** Opens the file of a match and selects it (like SyncTeX backward). */
export function openMatch(/** @type {SearchFile} */ file, /** @type {SearchMatch} */ match) {
  sync.reveal = { file: file.path, line: match.line, column: match.column, length: match.length, nonce: Date.now() }
  openFile(file.path)
}

/** ⇧⌘F / Ctrl+Shift+F: show the search tool and focus its input. */
export function focusSearch() {
  if (!(ui.sidebarOpen && ui.activeTool === 'search')) toggleTool('search')
  search.focusNonce++
}
