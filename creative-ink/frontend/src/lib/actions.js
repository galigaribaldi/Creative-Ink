// User actions triggered from anywhere in the layout (ActionBar, Sidebar,
// shortcuts). The Editor registers its commands here, so layout components
// can save, switch files or compile without holding a reference to
// CodeMirror and without prop drilling.
import {
  ConfirmClose,
  CreateProject,
  CreateQuickDocument,
  GetWorkspace,
  NewFileDialog,
  OpenFolderDialog,
  OpenProject,
  RefreshWorkspace,
  RequestCompile,
  SaveAsDialog,
  SetMainFile,
  SetupRepository
} from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { toggleBottomTab, toggleSidebar, ui } from './ui.svelte.js'
import { applyTree, applyWorkspace, compileTarget, workspace } from './workspace.svelte.js'

// Events emitted by Go (app.go, menu.go).
export const WORKSPACE_CHANGED = 'workspace:changed' // files changed outside the app
export const APP_BEFORE_CLOSE = 'app:before-close' // the window is closing
export const MENU_COMMAND = 'menu:command' // a native menu item was clicked (macOS)

/**
 * @typedef {{
 *   save: () => void,
 *   cancelScheduledCompile: () => void,
 *   waitIdle: () => Promise<boolean>,
 *   isDirty: () => boolean,
 *   getContent: () => string,
 *   adopt: (path: string) => void,
 *   focus: () => void,
 *   run: (command: string) => boolean
 * }} EditorCommands
 */

/** One Editor.svelte per pane; the secondary exists only while split. */
/** @type {Record<import('./ui.svelte.js').EditorPane, EditorCommands | null>} */
const editors = { primary: null, secondary: null }

/**
 * Called by Editor.svelte once CodeMirror is ready.
 * @param {import('./ui.svelte.js').EditorPane} pane
 * @param {EditorCommands} commands
 * @returns {() => void} unregister (call it on destroy)
 */
export function registerEditor(pane, commands) {
  editors[pane] = commands
  return () => {
    if (editors[pane] === commands) editors[pane] = null
  }
}

/** The pane that has (or last had) the focus. */
function focusedEditor() {
  return (ui.splitEditor && editors[ui.focusedPane]) || editors.primary
}

/** Saves the pending edits of every pane; false if any could not be written. */
async function persistPendingEdits() {
  const pending = [editors.primary, editors.secondary].filter(Boolean).map((e) => e.waitIdle())
  const results = await Promise.all(pending)
  return results.every(Boolean)
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
 * Opens a text file from the explorer (or SyncTeX) in the focused pane.
 * Only the selection changes here; the Editor of that pane reacts to it
 * (saves its current file, then loads the new one). A file already shown in
 * the other pane is focused there instead: two CodeMirror instances on the
 * same file would overwrite each other's autosaves.
 * @param {string} path absolute
 */
export function openFile(path) {
  if (path === workspace.currentFilePath) return focusPane('primary')
  if (ui.splitEditor && path === workspace.secondaryFilePath) return focusPane('secondary')
  if (ui.splitEditor && ui.focusedPane === 'secondary') workspace.secondaryFilePath = path
  else workspace.currentFilePath = path
}

function focusPane(/** @type {import('./ui.svelte.js').EditorPane} */ pane) {
  ui.focusedPane = pane
  editors[pane]?.focus()
}

/**
 * ⌘\ / Ctrl+\: shows or hides the second editor pane. Opening it focuses
 * it, so the next file chosen in the explorer goes there. Closing it saves
 * its pending edits first.
 */
export async function toggleSplitEditor() {
  if (!ui.splitEditor) {
    ui.splitEditor = true
    ui.focusedPane = 'secondary'
    ui.sidebarOpen = true // the explorer is how a file gets into the new pane
    return
  }
  if (editors.secondary && !(await editors.secondary.waitIdle())) return // keep it: unsaved
  ui.splitEditor = false
  workspace.secondaryFilePath = ''
  focusPane('primary')
}

/**
 * ⌘N / Ctrl+N "Nuevo archivo…": native dialog inside the open project
 * (.tex by default; .bib, .sty, .txt… by extension). Without a project it
 * creates a quick document instead. The new file opens in the main pane.
 */
export async function newFile() {
  if (!workspace.currentFolderPath) return newQuickDocument()
  if (!(await persistPendingEdits())) return
  try {
    const state = await NewFileDialog()
    if (!state.canceled) {
      applyWorkspace(state)
      ui.startOpen = false
    }
  } catch (err) {
    report(err)
  }
}

/** ⇧⌘N / Ctrl+Shift+N: tmp-Workspaces/unnamed-tmp<N>/main.tex. */
export async function newQuickDocument() {
  const message = await createQuickDocument()
  if (message) report(message)
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
export async function saveDocument() {
  // This compile replaces the ones the panes had scheduled after typing.
  for (const e of [editors.primary, editors.secondary]) e?.cancelScheduledCompile()
  if (!(await persistPendingEdits())) return // the PDF would not match the text
  if (compileTarget()) void RequestCompile()
}

/**
 * Native "Guardar como…" for the main pane's file (it is Go's active
 * document; the second pane only edits files in place).
 */
export async function saveAs() {
  const editor = editors.primary
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
      saved = await persistPendingEdits()
    } catch {
      saved = false
    }
    void ConfirmClose(saved)
  })
}

/**
 * Every command that a shortcut (App.svelte) or a native menu item (macOS,
 * menu.go) can trigger. Both paths call runCommand, so they behave the
 * same. Editor commands go to the focused pane's CodeMirror.
 */
const COMMANDS = {
  'new-file': newFile,
  'new-quick-doc': newQuickDocument,
  'open-folder': openFolder,
  start: () => (ui.startOpen = true),
  save: saveDocument,
  'save-as': saveAs,
  'toggle-sidebar': toggleSidebar,
  'toggle-terminal': () => toggleBottomTab('terminal'),
  'split-editor': toggleSplitEditor
}

/** Commands run by CodeMirror itself (Editor.svelte `run`). */
const EDITOR_COMMANDS = new Set(['find', 'find-next', 'find-previous', 'select-next', 'goto-line', 'toggle-comment'])

/** @param {string} id */
export function runCommand(id) {
  if (EDITOR_COMMANDS.has(id)) return void focusedEditor()?.run(id)
  const command = COMMANDS[id]
  if (command) void command()
}

/**
 * Native menu clicks (macOS). Call once during App init and call the
 * returned function on destroy.
 * @returns {() => void} unsubscribe
 */
export function listenMenuEvents() {
  return EventsOn(MENU_COMMAND, (/** @type {{ id: string }} */ event) => runCommand(event.id))
}
