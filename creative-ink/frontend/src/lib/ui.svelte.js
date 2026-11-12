// Purely visual UI state (which drawer is open, which modal is visible).
// It lives only in the frontend and is never sent to Go.

/** @typedef {'explorer' | 'search'} SidebarTool */
/** @typedef {'problems' | 'terminal' | 'shell'} BottomTab  'terminal' = Terminal CI */
/** @typedef {'primary' | 'secondary'} EditorPane */

// Declared before `ui`: its initial values read them.
const SPLIT_RATIO_KEY = 'creative-ink.splitRatio'
const MIN_SPLIT = 0.2
const MAX_SPLIT = 0.8

// Layout preferences (purely visual: localStorage, never Go). Ratios are the
// share of the first element: the editors next to the preview, the bottom
// panel inside the editor column.
const PREFS = {
  editorRatio: { key: 'creative-ink.editorRatio', min: 0.25, max: 0.8, def: 0.5 },
  editorRatioSplit: { key: 'creative-ink.editorRatioSplit', min: 0.4, max: 0.85, def: 2 / 3 },
  bottomRatio: { key: 'creative-ink.bottomRatio', min: 0.15, max: 0.75, def: 0.35 },
  zoom: { key: 'creative-ink.zoom', min: 0.7, max: 2, def: 1 }
}
export const ZOOM_STEP = 0.1

/** @param {keyof typeof PREFS} name */
function readPref(name) {
  const { key, min, max, def } = PREFS[name]
  try {
    const raw = localStorage.getItem(key)
    const saved = raw === null ? NaN : Number(raw)
    if (saved >= min && saved <= max) return saved
  } catch {
    // Storage unavailable: defaults.
  }
  return def
}

function readFlag(/** @type {string} */ key, /** @type {string} */ def) {
  try {
    return localStorage.getItem(key) ?? def
  } catch {
    return def
  }
}

function writePref(/** @type {string} */ key, /** @type {string} */ value) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Not persisted; it still applies for this session.
  }
}

export const ui = $state({
  sidebarOpen: false,
  /** @type {SidebarTool} */
  activeTool: 'explorer',
  healthOpen: false,
  /** Explorer folders the user expanded or collapsed, by absolute path. */
  /** @type {Record<string, boolean>} */
  expandedDirs: {},
  /** Bottom panel under the editor: "Errores LaTeX" | "Terminal CI" | "Shell". */
  bottomPanelOpen: false,
  /** @type {BottomTab} */
  bottomTab: 'problems',
  /** Opened by a compile error (not by the user): closes again on success. */
  bottomAutoOpened: false,
  /** Start screen reopened by the user while a project is open. */
  startOpen: false,
  /** Two editor panes side by side (⌘\). The main one decides what compiles. */
  splitEditor: false,
  /** Pane that last had the focus: the explorer opens files there. @type {EditorPane} */
  focusedPane: 'primary',
  /** Width of the main pane in the split editor (0.2–0.8), dragged on the divider. */
  splitRatio: readSplitRatio(),
  /** Share of the width for the editors next to the preview (one pane / split). */
  editorRatio: readPref('editorRatio'),
  editorRatioSplit: readPref('editorRatioSplit'),
  /** Share of the editor column's height for the bottom panel. */
  bottomRatio: readPref('bottomRatio'),
  /** The PDF preview is shown (it stays mounted when hidden: same PDF and page). */
  previewVisible: readFlag('creative-ink.previewVisible', 'true') !== 'false',
  /** @type {'left' | 'right'} */
  previewSide: readFlag('creative-ink.previewSide', 'right') === 'left' ? 'left' : 'right',
  /** UI and editor text zoom (⌘+ / ⌘- / ⌘0); the PDF has its own. */
  zoom: readPref('zoom'),
  /**
   * PDF view: 'continuous' (pdf.js PDFViewer: scroll, zoom, text) or
   * 'page' (the original one-page canvas). Block B2 of §4.4 #9, on trial.
   * @type {'continuous' | 'page'}
   */
  pdfMode: readFlag('creative-ink.pdfMode', 'continuous') === 'page' ? 'page' : 'continuous',
  /** The terminals' location banner was hidden (until another folder opens). */
  locationBannerHidden: false,
  /**
   * Explorer: the name field of a new file or folder (in `parent`), and the
   * right-click menu that opens it. @type {{ kind: 'file' | 'folder', parent: string } | null}
   */
  treeCreate: null,
  /** @type {{ x: number, y: number, parent: string } | null} */
  treeMenu: null,
  /** A short message at the bottom (dropped files that cannot be opened…). @type {{ id: number, text: string, level: 'info' | 'warning' } | null} */
  toast: null
})

let toastTimer = 0
let toastId = 0

/**
 * Shows a short message for a few seconds (purely visual). Go writes the
 * texts; the UI only shows them.
 * @param {string} text
 * @param {'info' | 'warning'} [level]
 */
export function showToast(text, level = 'info') {
  ui.toast = { id: ++toastId, text, level }
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (ui.toast = null), 6000)
}

function readSplitRatio() {
  try {
    const saved = Number(localStorage.getItem(SPLIT_RATIO_KEY))
    if (saved >= MIN_SPLIT && saved <= MAX_SPLIT) return saved
  } catch {
    // Storage unavailable: start centred.
  }
  return 0.5
}

/**
 * Sets the split editor's proportion (clamped to 20–80 %). With persist, it
 * is remembered across sessions (purely visual: localStorage, never Go).
 * @param {number} ratio share of the main pane
 * @param {boolean} [persist]
 */
export function setSplitRatio(ratio, persist = false) {
  ui.splitRatio = Math.min(MAX_SPLIT, Math.max(MIN_SPLIT, ratio))
  if (!persist) return
  try {
    localStorage.setItem(SPLIT_RATIO_KEY, String(ui.splitRatio))
  } catch {
    // Not persisted; it still applies for this session.
  }
}

/**
 * Sets a layout ratio (clamped); with persist, it is remembered.
 * @param {'editorRatio' | 'editorRatioSplit' | 'bottomRatio'} name
 * @param {number} ratio
 * @param {boolean} [persist]
 */
export function setLayoutRatio(name, ratio, persist = false) {
  const { key, min, max } = PREFS[name]
  ui[name] = Math.min(max, Math.max(min, ratio))
  if (persist) writePref(key, String(ui[name]))
}

/** Restores a layout ratio to its default (double click on a divider). */
export function resetLayoutRatio(/** @type {'editorRatio' | 'editorRatioSplit' | 'bottomRatio'} */ name) {
  setLayoutRatio(name, PREFS[name].def, true)
}

/** Shows or hides the PDF preview (⌥⌘P). */
export function togglePreview() {
  ui.previewVisible = !ui.previewVisible
  writePref('creative-ink.previewVisible', String(ui.previewVisible))
}

/** Moves the preview to the other side of the editors. */
export function swapPreviewSide() {
  ui.previewSide = ui.previewSide === 'right' ? 'left' : 'right'
  writePref('creative-ink.previewSide', ui.previewSide)
}

/** Switches the PDF view between continuous and one page. */
export function setPdfMode(/** @type {'continuous' | 'page'} */ mode) {
  ui.pdfMode = mode
  writePref('creative-ink.pdfMode', mode)
}

/** ⌘+ / ⌘- (step) and ⌘0 (0 = back to 100 %). */
export function zoomBy(/** @type {number} */ step) {
  const { key, min, max, def } = PREFS.zoom
  const next = step === 0 ? def : Math.round((ui.zoom + step) * 10) / 10
  ui.zoom = Math.min(max, Math.max(min, next))
  writePref(key, String(ui.zoom))
}

/**
 * Opens the sidebar on `tool`, or closes it if that tool is already showing
 * (the usual activity-bar behaviour).
 * @param {SidebarTool} tool
 */
export function toggleTool(tool) {
  if (ui.sidebarOpen && ui.activeTool === tool) {
    ui.sidebarOpen = false
  } else {
    ui.activeTool = tool
    ui.sidebarOpen = true
  }
}

export function toggleSidebar() {
  ui.sidebarOpen = !ui.sidebarOpen
}

/** Expands or collapses an explorer folder. */
export function setDirExpanded(/** @type {string} */ path, /** @type {boolean} */ expanded) {
  ui.expandedDirs[path] = expanded
}

/**
 * Shows a bottom-panel tab, or closes the panel if that tab is already
 * visible (Ctrl+` / ⌘J and the action bar toggle the terminal this way).
 * @param {BottomTab} tab
 */
export function toggleBottomTab(tab) {
  if (ui.bottomPanelOpen && ui.bottomTab === tab) {
    ui.bottomPanelOpen = false
  } else {
    ui.bottomTab = tab
    ui.bottomPanelOpen = true
  }
  ui.bottomAutoOpened = false // the user is in control now
}

export function closeBottomPanel() {
  ui.bottomPanelOpen = false
  ui.bottomAutoOpened = false
}
