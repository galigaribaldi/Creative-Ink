// Purely visual UI state (which drawer is open, which modal is visible).
// It lives only in the frontend and is never sent to Go.

/** @typedef {'explorer'} SidebarTool */
/** @typedef {'problems' | 'terminal' | 'shell'} BottomTab  'terminal' = Terminal CI */
/** @typedef {'primary' | 'secondary'} EditorPane */

// Declared before `ui`: its initial splitRatio reads them.
const SPLIT_RATIO_KEY = 'creative-ink.splitRatio'
const MIN_SPLIT = 0.2
const MAX_SPLIT = 0.8

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
  splitRatio: readSplitRatio()
})

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
