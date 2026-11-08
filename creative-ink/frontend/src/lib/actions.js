// User actions triggered from anywhere in the layout (ActionBar, Sidebar,
// shortcuts). The Editor registers its commands here, so layout components
// can save, switch files or compile without holding a reference to
// CodeMirror and without prop drilling.
import {
  ConfirmClose,
  CreateProject,
  CreateQuickDocument,
  GetWorkspace,
  OpenFolderDialog,
  OpenProject,
  RefreshWorkspace,
  RequestCompile,
  SaveAsDialog,
  SetMainFile,
  SetupRepository
} from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { ui } from './ui.svelte.js'
import { applyTree, applyWorkspace, compileTarget, workspace } from './workspace.svelte.js'

// Events emitted by Go (app.go).
export const WORKSPACE_CHANGED = 'workspace:changed' // files changed outside the app
export const APP_BEFORE_CLOSE = 'app:before-close' // the window is closing

/**
 * @typedef {{
 *   save: () => void,
 *   saveAndCompile: () => Promise<void>,
 *   waitIdle: () => Promise<boolean>,
 *   isDirty: () => boolean,
 *   getContent: () => string,
 *   adopt: (path: string) => void
 * }} EditorCommands
 */

/** @type {EditorCommands | null} */
let editor = null

/**
 * Called by Editor.svelte once CodeMirror is ready.
 * @param {EditorCommands} commands
 * @returns {() => void} unregister (call it on destroy)
 */
export function registerEditor(commands) {
  editor = commands
  return () => {
    if (editor === commands) editor = null
  }
}

/** Saves pending edits; false if they could not be written. */
async function persistPendingEdits() {
  return editor ? editor.waitIdle() : true
}

function report(err) {
  workspace.error = String(err)
}

/** Loads the folder Go has open at startup. */
export async function initWorkspace() {
  try {
    applyWorkspace(await GetWorkspace())
  } catch (err) {
    report(err)
  }
}

/**
 * Native "Abrir carpeta". Pending edits are saved first, while their folder
 * is still the open one: after the switch Go would reject a late save.
 */
export async function openFolder() {
  if (!(await persistPendingEdits())) return
  try {
    const state = await OpenFolderDialog()
    if (!state.canceled) {
      applyWorkspace(state)
      ui.startOpen = false
    }
  } catch (err) {
    report(err)
  }
}

/**
 * First run: creates the central repository for the chosen scope and opens
 * it. Go computes the path for this OS; only the scope is sent.
 * @param {'user' | 'shared'} scope
 * @returns {Promise<boolean>} true on success
 */
export async function setupRepository(scope) {
  try {
    applyWorkspace(await SetupRepository(scope))
    return true
  } catch (err) {
    report(err)
    return false
  }
}

// At most one RefreshWorkspace in flight; requests meanwhile collapse into one more.
let refreshing = false
let refreshPending = false

/**
 * Start screen actions. Each saves pending edits first (the folder changes)
 * and closes the start screen on success. They return an error message or ''.
 */
async function switchProject(/** @type {() => Promise<import('./workspace.svelte.js').WorkspaceState>} */ open) {
  if (!(await persistPendingEdits())) return 'No se pudieron guardar los cambios del archivo abierto.'
  try {
    applyWorkspace(await open())
    ui.startOpen = false
    return ''
  } catch (err) {
    return String(err)
  }
}

/** Creates <repo>/<name normalized with "-">/ from the template and opens it. */
export function createProject(/** @type {string} */ name) {
  return switchProject(() => CreateProject(name))
}

/** Creates tmp-Workspaces/unnamed-tmp<N>/ and opens it. */
export function createQuickDocument() {
  return switchProject(() => CreateQuickDocument())
}

/** Opens a project, quick document or recent folder listed by the start screen. */
export function openProject(/** @type {string} */ path) {
  return switchProject(() => OpenProject(path))
}

/** Rescans the open folder and updates the explorer (never the open file). */
export async function refreshWorkspace() {
  if (refreshing) {
    refreshPending = true
    return
  }
  refreshing = true
  try {
    applyTree(await RefreshWorkspace())
  } catch (err) {
    report(err)
  } finally {
    refreshing = false
    if (refreshPending) {
      refreshPending = false
      void refreshWorkspace()
    }
  }
}

/**
 * Refreshes the explorer when Go reports external changes (already
 * debounced and de-duplicated in Go). Call once during App init and call
 * the returned function on destroy.
 * @returns {() => void} unsubscribe
 */
export function listenWorkspaceEvents() {
  return EventsOn(WORKSPACE_CHANGED, (/** @type {{ folder: string }} */ event) => {
    if (event.folder === workspace.currentFolderPath) void refreshWorkspace()
  })
}

/**
 * Opens a .tex from the explorer. Only the selection changes here; the
 * Editor reacts to it (saves the current file, then loads the new one).
 * @param {string} path absolute
 */
export function openFile(path) {
  workspace.currentFilePath = path
}

/**
 * Makes `path` the project's main file, or clears it with "". Go stores it
 * in the folder's .creative-ink-meta and recompiles the new target.
 * @param {string} path absolute, or ""
 */
export async function setMainFile(path) {
  try {
    applyWorkspace(await SetMainFile(path))
  } catch (err) {
    report(err)
  }
}

/**
 * Cmd/Ctrl+S and the toolbar's Guardar: saves now, skipping the debounce,
 * and then compiles. Without a document, it only compiles.
 */
export function saveDocument() {
  if (editor) void editor.saveAndCompile()
  else if (compileTarget()) void RequestCompile()
}

/** Native "Guardar como…": writes the editor content to a new .tex. */
export async function saveAs() {
  if (!editor || !(await persistPendingEdits())) return
  try {
    const state = await SaveAsDialog(editor.getContent())
    if (state.canceled) return
    // Same content, new file: rename the editor's document in place instead
    // of reloading it (keeps cursor, scroll and undo history).
    editor.adopt(state.filePath)
    applyWorkspace(state)
  } catch (err) {
    report(err)
  }
}

/**
 * The toolbar's Compilar: pending edits are saved first, then Go compiles
 * the target (main file or active document). Same as Cmd/Ctrl+S.
 */
export function compileDocument() {
  saveDocument()
}

/**
 * Safe close (Go's OnBeforeClose, see app.go): when the window is closing,
 * flush CodeMirror to disk and tell Go whether everything was saved. Go
 * quits anyway after 2 s, and asks before discarding a failed save.
 * @returns {() => void} unsubscribe
 */
export function listenAppEvents() {
  return EventsOn(APP_BEFORE_CLOSE, async () => {
    let saved = true
    try {
      saved = editor ? await editor.waitIdle() : true
    } catch {
      saved = false
    }
    void ConfirmClose(saved)
  })
}
