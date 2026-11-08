// User actions triggered from anywhere in the layout (ActionBar, Sidebar,
// shortcuts). The Editor registers its commands here, so layout components
// can save, switch files or compile without holding a reference to
// CodeMirror and without prop drilling.
import {
  GetWorkspace,
  OpenFolderDialog,
  RefreshWorkspace,
  RequestCompile,
  SaveAsDialog
} from '../../wailsjs/go/main/App.js'
import { applyWorkspace, workspace } from './workspace.svelte.js'

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

/** Rescans the open folder. */
export async function refreshWorkspace() {
  try {
    applyWorkspace(await RefreshWorkspace())
  } catch (err) {
    report(err)
  }
}

/**
 * Opens a .tex from the explorer. Only the selection changes here; the
 * Editor reacts to it (saves the current file, then loads the new one).
 * @param {string} path absolute
 */
export function openFile(path) {
  workspace.currentFilePath = path
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
