// Mirror of the workspace as Go reports it (open folder, active document,
// folder tree). Go is the source of truth for the filesystem: this module
// only holds the last WorkspaceState received, plus the file the user wants
// to have open in the editor.
//
// Paths are absolute. Go validates every one against the open folder.

/**
 * @typedef {{ name: string, path: string, isDir: boolean, editable: boolean, children?: FileNode[] }} FileNode
 * @typedef {{ needsSetup: boolean, canceled: boolean, folderPath: string, filePath: string, mainFilePath: string, tree: FileNode, truncated: boolean }} WorkspaceState
 */

export const workspace = $state({
  /** Absolute path of the open folder ("" before the first load). */
  currentFolderPath: '',
  /**
   * Absolute path of the .tex shown in the editor ("" = none). Changing it
   * makes Editor.svelte save pending edits and load the new file.
   */
  currentFilePath: '',
  /**
   * Absolute path of the project's root document ("" = none). When set, Go
   * compiles and previews it whatever file is being edited.
   */
  mainFilePath: '',
  /** @type {FileNode | null} */
  fileTree: null,
  truncated: false,
  /** First run: the user has not chosen where the central repository lives. */
  needsSetup: false,
  /** Last workspace error (dialogs, scan), shown in the explorer. */
  error: ''
})

/** Applies a WorkspaceState returned by Go. */
export function applyWorkspace(/** @type {WorkspaceState} */ state) {
  workspace.needsSetup = state.needsSetup
  workspace.currentFolderPath = state.folderPath
  workspace.fileTree = state.tree
  workspace.truncated = state.truncated
  workspace.currentFilePath = state.filePath
  workspace.mainFilePath = state.mainFilePath
  workspace.error = ''
}

/**
 * Applies only the folder contents of a WorkspaceState (tree, truncation,
 * main file). Used by refreshes, automatic or manual: they must never touch
 * currentFilePath, or a refresh landing during a file switch would send the
 * editor back to the previous file.
 */
export function applyTree(/** @type {WorkspaceState} */ state) {
  if (state.folderPath !== workspace.currentFolderPath) return // folder changed meanwhile
  workspace.fileTree = state.tree
  workspace.truncated = state.truncated
  workspace.mainFilePath = state.mainFilePath
}

/**
 * What Go compiles and previews: the main file if the project has one,
 * otherwise the file being edited. Reactive when read inside $derived/$effect.
 */
export function compileTarget() {
  return workspace.mainFilePath || workspace.currentFilePath
}

/** Last path segment, for display. */
export function baseName(/** @type {string} */ path) {
  return path.split(/[\\/]/).filter(Boolean).pop() ?? ''
}
