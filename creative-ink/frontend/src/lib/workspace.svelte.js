// Mirror of the workspace as Go reports it (open folder, active document,
// folder tree). Go is the source of truth for the filesystem: this module
// only holds the last WorkspaceState received, plus the file the user wants
// to have open in the editor.
//
// Paths are absolute. Go validates every one against the open folder.

/**
 * @typedef {{ name: string, path: string, isDir: boolean, editable: boolean, children?: FileNode[] }} FileNode
 * @typedef {'' | 'trusted' | 'untrusted' | 'restricted'} TrustStatus
 * @typedef {{ id: string, level: 'info' | 'warning', title: string, text: string, steps: string[] }} LocationNotice
 * @typedef {{ kind: '' | 'repository' | 'quickdoc' | 'external', folder: string, gitRoot: string, syncService: string, collection: boolean, documents: number, notices: LocationNotice[] }} ProjectLocation
 * @typedef {{ location: ProjectLocation, compileTarget: string, singleFile: string, needsSetup: boolean, needsStart: boolean, repositoryPath: string, canceled: boolean, folderPath: string, filePath: string, texFilePath: string, mainFilePath: string, autoCompile: boolean, tree: FileNode, truncated: boolean, trustStatus: TrustStatus, trustFiles: string[] }} WorkspaceState
 * @typedef {{ kind: 'files' | 'size' | 'images', message: string }} ProjectWarning
 */
import { config } from './config.svelte.js'
import { addTab, removeTab, resetTabs } from './tabs.svelte.js'
import { noteLocation } from './terminal.svelte.js'

/** @type {ProjectLocation} */
const NO_LOCATION = { kind: '', folder: '', gitRoot: '', syncService: '', collection: false, documents: 0, notices: [] }

export const workspace = $state({
  /** Absolute path of the open folder ("" before the first load). */
  currentFolderPath: '',
  /**
   * Absolute path of the file shown in the main editor pane ("" = none):
   * any text file (.tex, .bib, .sty…). Changing it makes Editor.svelte save
   * pending edits and load the new file.
   */
  currentFilePath: '',
  /**
   * Last .tex opened in the main pane: what Go compiles when the project has
   * no main file. Opening refs.bib keeps it, so the preview does not change.
   */
  texFilePath: '',
  /** File of the second editor pane ("" = none or split closed). Never equal to currentFilePath. */
  secondaryFilePath: '',
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
  /** No project open yet: the start screen must be shown. */
  needsStart: false,
  /** Absolute path of My-Creative-Ink-Repository (the launcher). */
  repositoryPath: '',
  /** Last workspace error (dialogs, scan), shown in the explorer. */
  error: '',
  /**
   * Large-project detector (Go: project:health). Only informative: the
   * editor shows a warning with a tooltip and changes nothing else.
   * @type {ProjectWarning[]}
   */
  healthWarnings: [],
  /**
   * Trust of the folder latexmk runs in (Go decides; docs/MAP.md §4.6).
   * 'untrusted': its .latexmkrc is ignored (latexmk -norc) and the UI asks.
   * @type {TrustStatus}
   */
  trustStatus: '',
  /** The rc files found there (".latexmkrc", "latexmkrc"). @type {string[]} */
  trustFiles: [],
  /**
   * Where the open folder lives (Go decides, before opening it): outside
   * the repository it carries default notices, light without git and strong
   * with git (docs/MAP.md §4.7). The terminal only shows them.
   * @type {ProjectLocation}
   */
  location: NO_LOCATION,
  /**
   * What Go compiles and previews (WorkspaceState.compileTarget / OpenFile):
   * the main file, or the root document of the last .tex — in a collection,
   * the main.tex of the document being edited. "" = nothing.
   */
  compileTargetPath: '',
  /** A loose file open where it is (Desktop, Downloads): only it is shown. */
  singleFile: '',
  /**
   * ⌘N: the main pane shows a new untitled document. It lives only in the
   * editor (Go knows nothing until ⌘S saves it with SaveNewDialog); while
   * it has text, the editor refuses to switch away from it.
   */
  untitled: false
})

/** Applies a WorkspaceState returned by Go. */
export function applyWorkspace(/** @type {WorkspaceState} */ state) {
  // Another folder: the second pane's file belongs to the old one (its edits
  // were saved before the switch, see actions.js persistPendingEdits).
  if (state.folderPath !== workspace.currentFolderPath || state.filePath === workspace.secondaryFilePath) {
    if (state.filePath && state.filePath === workspace.secondaryFilePath) removeTab('secondary', state.filePath)
    workspace.secondaryFilePath = ''
  }
  // Tabs: another project starts with its open file; in the same one, Go's
  // active file (Save As, a new file…) gets its tab next to the shown one.
  if (state.folderPath !== workspace.currentFolderPath) resetTabs(state.filePath)
  else if (state.filePath) addTab('primary', state.filePath, workspace.currentFilePath)
  const folderChanged = state.folderPath !== workspace.currentFolderPath
  // Another project: its own analysis arrives later (project:health).
  if (folderChanged) {
    workspace.healthWarnings = []
    workspace.untitled = false // another project: the editor shows its file
  }
  workspace.location = state.location ?? NO_LOCATION
  // Opening a folder outside the repository leaves a line in both terminals.
  if (folderChanged && state.folderPath) noteLocation(workspace.location)
  config.autoCompile = state.autoCompile
  applyTrust(state)
  workspace.needsSetup = state.needsSetup
  workspace.needsStart = state.needsStart
  workspace.repositoryPath = state.repositoryPath
  workspace.currentFolderPath = state.folderPath
  workspace.fileTree = state.tree
  workspace.truncated = state.truncated
  workspace.currentFilePath = state.filePath
  workspace.texFilePath = state.texFilePath
  workspace.mainFilePath = state.mainFilePath
  workspace.compileTargetPath = state.compileTarget ?? ''
  workspace.singleFile = state.singleFile ?? ''
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
  workspace.compileTargetPath = state.compileTarget ?? workspace.compileTargetPath
  applyTrust(state)
}

/** Applies the trust part of a WorkspaceState or of a trust:changed event. */
export function applyTrust(/** @type {{ trustStatus?: TrustStatus, trustFiles?: string[] }} */ state) {
  workspace.trustStatus = state.trustStatus ?? ''
  workspace.trustFiles = state.trustFiles ?? []
}

/**
 * What Go compiles and previews: the main file if the project has one,
 * otherwise the last .tex of the main pane (App.compileTarget in Go).
 * Reactive when read inside $derived/$effect.
 */
export function compileTarget() {
  return workspace.compileTargetPath || workspace.mainFilePath || workspace.texFilePath
}

/** LaTeX source (can be compiled, be the main file, use SyncTeX). */
export function isTeXPath(/** @type {string} */ path) {
  return /\.tex$/i.test(path)
}

/** Last path segment, for display. */
export function baseName(/** @type {string} */ path) {
  return path.split(/[\\/]/).filter(Boolean).pop() ?? ''
}
