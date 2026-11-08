// Mirror of the workspace as Go reports it (open folder, active document,
// folder tree). Go is the source of truth for the filesystem: this module
// only holds the last WorkspaceState received, plus the file the user wants
// to have open in the editor.
//
// Paths are absolute. Go validates every one against the open folder.

/**
 * @typedef {{ name: string, path: string, isDir: boolean, editable: boolean, children?: FileNode[] }} FileNode
 * @typedef {{ canceled: boolean, folderPath: string, filePath: string, tree: FileNode, truncated: boolean }} WorkspaceState
 */

export const workspace = $state({
  /** Absolute path of the open folder ("" before the first load). */
  currentFolderPath: '',
  /**
   * Absolute path of the .tex shown in the editor ("" = none). Changing it
   * makes Editor.svelte save pending edits and load the new file.
   */
  currentFilePath: '',
  /** @type {FileNode | null} */
  fileTree: null,
  truncated: false,
  /** Last workspace error (dialogs, scan), shown in the explorer. */
  error: ''
})

/** Applies a WorkspaceState returned by Go. */
export function applyWorkspace(/** @type {WorkspaceState} */ state) {
  workspace.currentFolderPath = state.folderPath
  workspace.fileTree = state.tree
  workspace.truncated = state.truncated
  workspace.currentFilePath = state.filePath
  workspace.error = ''
}

/** Last path segment, for display. */
export function baseName(/** @type {string} */ path) {
  return path.split(/[\\/]/).filter(Boolean).pop() ?? ''
}
