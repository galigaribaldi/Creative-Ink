// User actions triggered from anywhere in the layout (ActionBar, Sidebar,
// shortcuts). The Editor registers its commands here, so layout components
// can save, switch files or compile without holding a reference to
// CodeMirror and without prop drilling.
import {
  GetWorkspace,
  OpenFolderDialog,
  RefreshWorkspace,
  RequestCompile,
  SaveAsDialog,
  SetMainFile,
  SetupRepository
} from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { applyTree, applyWorkspace, workspace } from './workspace.svelte.js'

// Event emitted by Go when files or folders change outside the app (app.go).
export const WORKSPACE_CHANGED = 'workspace:changed'

/**
 * @typedef {{
 *   save: () => void,
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
    if (!state.canceled) applyWorkspace(state)
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

/** Saves now, skipping the autosave debounce. A save also queues a compile. */
export function saveDocument() {
  editor?.save()
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
 * Compiles the active document. Unsaved edits are saved first (the save
 * queues the compilation itself); otherwise Go compiles what is on disk.
 */
export function compileDocument() {
  if (editor?.isDirty()) {
    editor.save()
  } else {
    void RequestCompile()
  }
}
