// Purely visual UI state (which drawer is open, which modal is visible).
// It lives only in the frontend and is never sent to Go.

/** @typedef {'explorer'} SidebarTool */

export const ui = $state({
  sidebarOpen: false,
  /** @type {SidebarTool} */
  activeTool: 'explorer',
  healthOpen: false,
  /** Explorer folders the user expanded or collapsed, by absolute path. */
  /** @type {Record<string, boolean>} */
  expandedDirs: {}
})

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
