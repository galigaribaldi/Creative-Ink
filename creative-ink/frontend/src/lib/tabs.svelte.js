// Tabs of the editor panes (docs/MAP.md §4.4, #9, C1; "Multiventana" mode).
// Purely frontend: the list of files open in each pane. Go still knows only
// the main pane's active file (OpenFile), and the content always comes from
// disk; Editor.svelte keeps each tab's CodeMirror state (cursor, undo) in
// memory while the tab is open.
//
// A file is never in both panes (their autosaves would overwrite each
// other): actions.js openFile() enforces it.

/** Above this, the oldest inactive tab of the pane closes on its own. */
export const MAX_TABS = 20

export const tabs = $state({
  /** @type {string[]} */
  primary: [],
  /** @type {string[]} */
  secondary: []
})

/** @typedef {import('./ui.svelte.js').EditorPane} EditorPane */

/**
 * Adds path to the pane's tabs (after `after`, or at the end) unless it is
 * there. replace: the minimalist mode shows one file: path takes the place
 * of `after` instead (the other tabs stay for the multi-window mode).
 * @param {EditorPane} pane @param {string} path @param {string} [after] @param {boolean} [replace]
 */
export function addTab(pane, path, after = '', replace = false) {
  const list = tabs[pane]
  if (!path || list.includes(path)) return
  const i = after ? list.indexOf(after) : -1
  if (replace && i >= 0) list[i] = path
  else if (i >= 0) list.splice(i + 1, 0, path)
  else list.push(path)
  while (list.length > MAX_TABS) {
    const oldest = list.findIndex((p) => p !== path && p !== after) // never the shown one
    list.splice(oldest, 1)
  }
}

/**
 * Removes path from the pane and returns the tab to show instead (its right
 * neighbour, or the left one; "" if none).
 * @param {EditorPane} pane @param {string} path
 */
export function removeTab(pane, path) {
  const list = tabs[pane]
  const i = list.indexOf(path)
  if (i < 0) return ''
  list.splice(i, 1)
  return list[i] ?? list[i - 1] ?? ''
}

/** Which pane has path open in a tab ('' = none). @returns {'' | EditorPane} */
export function paneOf(/** @type {string} */ path) {
  if (tabs.primary.includes(path)) return 'primary'
  if (tabs.secondary.includes(path)) return 'secondary'
  return ''
}

/** Another project: only its open file (Go's) remains. */
export function resetTabs(/** @type {string} */ primaryFile) {
  tabs.primary = primaryFile ? [primaryFile] : []
  tabs.secondary = []
}
