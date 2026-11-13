// Editor panes (up to 4) and their arrangement (docs/MAP.md §4.4, #9).
// Purely frontend state: which file each pane shows, its tabs, the focused
// pane, and the layout tree (lib/layout.js), remembered globally in
// localStorage. Go knows only the focused pane's file (SetActiveDocument /
// OpenFile); the content always comes from disk.
//
// A file is never in two panes (their autosaves would overwrite each
// other): actions.js openFile() enforces it.
import { MAX_PANES, PANE_IDS, defaultLayout, itemsOf, panesOf, placeItem, removeItem, validLayout } from './layout.js'

/** @typedef {import('./layout.js').PaneId} PaneId */
/** @typedef {import('./layout.js').LayoutNode} LayoutNode */

/** Above this, the oldest tab of a pane that is not shown closes on its own. */
export const MAX_TABS = 20
const LAYOUT_KEY = 'creative-ink.layout'

function readLayout() {
  try {
    const saved = validLayout(JSON.parse(localStorage.getItem(LAYOUT_KEY) ?? 'null'))
    if (saved) return saved
  } catch {
    // Storage unavailable or not JSON: the default arrangement.
  }
  return defaultLayout()
}

/** @returns {Record<PaneId, string>} */
const emptyFiles = () => ({ p1: '', p2: '', p3: '', p4: '' })
/** @returns {Record<PaneId, string[]>} */
const emptyTabs = () => ({ p1: [], p2: [], p3: [], p4: [] })

export const panes = $state({
  /** The arrangement of "Multiventana" (the editor panes and the preview). @type {LayoutNode} */
  layout: readLayout(),
  /** The pane that last had the focus: files open there; its file is Go's active one. @type {PaneId} */
  focused: /** @type {PaneId} */ ('p1'),
  /** The file each pane shows ("" = none). */
  files: emptyFiles(),
  /** The tabs of each pane (Multiventana). */
  tabs: emptyTabs(),
  /** The pane showing the untitled document of ⌘N ("" = none). @type {'' | PaneId} */
  untitled: /** @type {'' | PaneId} */ ('')
})

// The focused pane must exist (the stored layout may not have p1).
if (!panesOf(panes.layout).includes(panes.focused)) panes.focused = panesOf(panes.layout)[0]

/** Editor panes that exist now, in layout order. */
export function openPanes() {
  return panesOf(panes.layout)
}

/**
 * Replaces the arrangement; with persist (the default), it is remembered.
 * While a divider is dragged it is not (one write when the drag ends).
 * @param {LayoutNode} layout
 */
export function setLayout(layout, persist = true) {
  panes.layout = layout
  if (!persist) return
  try {
    localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout))
  } catch {
    // Not persisted; it still applies for this session.
  }
}

/** Back to one editor and the preview (the panes' files are kept in p1 order). */
export function resetLayout() {
  setLayout(defaultLayout())
  if (!openPanes().includes(panes.focused)) panes.focused = 'p1'
}

/**
 * A new, empty pane next to `next` (right of it); null when there are
 * already 4. @param {PaneId} next @returns {PaneId | null}
 */
export function addPane(next) {
  const open = openPanes()
  if (open.length >= MAX_PANES) return null
  const id = /** @type {PaneId} */ (PANE_IDS.find((p) => !open.includes(p)))
  panes.files[id] = ''
  panes.tabs[id] = []
  setLayout(placeItem(panes.layout, id, next, 'right'))
  return id
}

/** Removes a pane (its edits must be saved first). @param {PaneId} id */
export function dropPane(id) {
  if (openPanes().length <= 1) return
  setLayout(removeItem(panes.layout, id))
  panes.files[id] = ''
  panes.tabs[id] = []
  if (panes.untitled === id) panes.untitled = ''
  if (panes.focused === id) panes.focused = openPanes()[0]
}

/**
 * Adds path to the pane's tabs (after `after`, or at the end) unless it is
 * there. replace: the minimalist mode shows one file: path takes the place
 * of `after` instead (the other tabs stay for the multi-window mode).
 * @param {PaneId} pane @param {string} path @param {string} [after] @param {boolean} [replace]
 */
export function addTab(pane, path, after = '', replace = false) {
  const list = panes.tabs[pane]
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
 * @param {PaneId} pane @param {string} path
 */
export function removeTab(pane, path) {
  const list = panes.tabs[pane]
  const i = list.indexOf(path)
  if (i < 0) return ''
  list.splice(i, 1)
  return list[i] ?? list[i - 1] ?? ''
}

/** Which open pane has path (shown or as a tab); '' = none. @returns {'' | PaneId} */
export function paneOf(/** @type {string} */ path) {
  if (!path) return ''
  return openPanes().find((p) => panes.files[p] === path || panes.tabs[p].includes(path)) ?? ''
}

/** The file shown in some pane (the explorer marks it). */
export function isShown(/** @type {string} */ path) {
  return !!path && openPanes().some((p) => panes.files[p] === path)
}

/** Another project: the panes stay (the layout is global), empty, and the focused one shows its file. */
export function resetPanes(/** @type {string} */ file) {
  panes.files = emptyFiles()
  panes.tabs = emptyTabs()
  panes.untitled = ''
  if (file) {
    panes.files[panes.focused] = file
    panes.tabs[panes.focused] = [file]
  }
}

/**
 * ⇄ of the preview in Multiventana: the preview moves to the other end of
 * the top-level row (the arrangement keeps everything else). Nothing to do
 * when it is not a direct child of a row at the top.
 */
export function mirrorPreview() {
  const root = panes.layout
  if (root.kind !== 'split' || root.dir !== 'row') return
  const items = root.children.map((c) => (c.kind === 'leaf' ? c.item : ''))
  const i = items.indexOf('pdf')
  if (i < 0) return
  const others = itemsOf(root).filter((it) => it !== 'pdf')
  const edge = i === 0 ? others.at(-1) : others[0]
  if (edge) setLayout(placeItem(root, 'pdf', edge, i === 0 ? 'right' : 'left'))
}

/**
 * path after moving `from` to `to` (a file, or a folder and everything
 * below it); other paths unchanged. Separators of the three systems.
 * @param {string} path @param {string} from @param {string} to
 */
export function remapPath(path, from, to) {
  if (!path || !from) return path
  if (path === from) return to
  if (path.startsWith(from + '/') || path.startsWith(from + '\\')) return to + path.slice(from.length)
  return path
}
