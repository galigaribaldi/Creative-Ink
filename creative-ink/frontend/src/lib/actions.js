// User actions triggered from anywhere in the layout (ActionBar, Sidebar,
// shortcuts). The Editor registers its commands here, so layout components
// can save, switch files or compile without holding a reference to
// CodeMirror and without prop drilling.
import {
  ConfirmClose,
  CreateProject,
  CreateFile,
  CreateFolder,
  CreateProjectFromTemplate,
  CreateQuickDocument,
  GetProjectHealth,
  GetWorkspace,
  NewWindow,
  OpenFolderDialog,
  OpenBlankDocument,
  OpenImportInPlace,
  OpenProject,
  OpenSystemTerminal,
  RefreshWorkspace,
  RequestCompile,
  SaveAsDialog,
  SaveNewDialog,
  SetMainFile,
  SetupRepository,
  TrustProject,
  UntrustProject
} from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { addTab, paneOf, removeTab, tabs } from './tabs.svelte.js'
import {
  saveEditorMode,
  setDirExpanded,
  setPreviewVisible,
  showToast,
  swapPreviewSide,
  toggleBottomTab,
  togglePreview,
  toggleSidebar,
  ui,
  ZOOM_STEP,
  zoomBy
} from './ui.svelte.js'
import { applyTree, applyTrust, applyWorkspace, compileTarget, workspace } from './workspace.svelte.js'

// Events emitted by Go (app.go, menu.go).
export const WORKSPACE_CHANGED = 'workspace:changed' // files changed outside the app
export const APP_BEFORE_CLOSE = 'app:before-close' // the window is closing
export const MENU_COMMAND = 'menu:command' // a native menu item was clicked (macOS)
export const PROJECT_HEALTH = 'project:health' // large-project detector (informative)
export const TRUST_CHANGED = 'trust:changed' // trust of the project's .latexmkrc changed

/**
 * @typedef {{
 *   save: () => void,
 *   cancelScheduledCompile: () => void,
 *   waitIdle: () => Promise<boolean>,
 *   isDirty: () => boolean,
 *   getContent: () => string,
 *   adopt: (path: string) => void,
 *   isUntitled: () => boolean,
 *   hasUntitledText: () => boolean,
 *   affectsBuild: () => boolean,
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
    // After a reload (wails dev) the last analysis is not re-emitted.
    applyHealth(await GetProjectHealth())
  } catch (err) {
    report(err)
  }
}

/** @param {{ folder: string, warnings: import('./workspace.svelte.js').ProjectWarning[] }} health */
function applyHealth(health) {
  if (health.folder === workspace.currentFolderPath) workspace.healthWarnings = health.warnings ?? []
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
  if (editors.primary?.hasUntitledText()) return UNTITLED_BLOCKS
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
export function createProject(/** @type {string} */ name, /** @type {string} */ template = '') {
  // template: the path of a template listed by the start screen ('' = built-in).
  return switchProject(() => (template ? CreateProjectFromTemplate(name, template) : CreateProject(name)))
}

/** Creates tmp-Workspaces/unnamed-tmp<N>/ and opens it. */
export function createQuickDocument() {
  return switchProject(() => CreateQuickDocument())
}

/** Start screen ✕ with nothing open: an empty quick document (reused if untouched). */
export function openBlankDocument() {
  return switchProject(() => OpenBlankDocument())
}

/** Import summary: opens the analysed folder where it is (no copy). */
export function openImportInPlace(/** @type {string} */ id) {
  return switchProject(() => OpenImportInPlace(id))
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
  const offs = [
    EventsOn(WORKSPACE_CHANGED, (/** @type {{ folder: string }} */ event) => {
      if (event.folder === workspace.currentFolderPath) void refreshWorkspace()
    }),
    // The large-project warning only informs: it changes nothing else.
    EventsOn(PROJECT_HEALTH, applyHealth),
    // `ci trust …` in the terminal, the banner, or a .latexmkrc that changed.
    EventsOn(
      TRUST_CHANGED,
      (/** @type {{ project: string, status: import('./workspace.svelte.js').TrustStatus, rcFiles: string[] }} */ event) => {
        if (event.project === workspace.currentFolderPath) {
          applyTrust({ trustStatus: event.status, trustFiles: event.rcFiles })
        }
      }
    )
  ]
  return () => offs.forEach((off) => off())
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
  // A file is never in both panes: if one has it (as a tab), it opens there.
  const owner = paneOf(path)
  /** @type {import('./ui.svelte.js').EditorPane} */
  let pane = ui.splitEditor && ui.focusedPane === 'secondary' ? 'secondary' : 'primary'
  if (owner === 'primary' || (owner === 'secondary' && ui.splitEditor)) pane = owner
  // Leaving the untitled document (the editor refuses it if it has text).
  const leavingUntitled = pane === 'primary' && workspace.untitled
  if (leavingUntitled) workspace.untitled = false
  const current = pane === 'primary' ? workspace.currentFilePath : workspace.secondaryFilePath
  // Multiventana: one more tab. Minimalista: it takes the place of the shown one.
  addTab(pane, path, current, ui.editorMode !== 'multi')
  ui.focusedPane = pane
  if (path === current && !leavingUntitled) return void editors[pane]?.focus()
  if (pane === 'primary') workspace.currentFilePath = path
  else workspace.secondaryFilePath = path
}

/**
 * Closes a tab. The shown one is saved first (a failed save keeps it open);
 * the neighbour tab is shown instead, and the second pane closes with its
 * last tab.
 * @param {import('./ui.svelte.js').EditorPane} pane
 * @param {string} path
 */
export async function closeTab(pane, path) {
  const active = pane === 'primary' ? workspace.currentFilePath : workspace.secondaryFilePath
  if (path === active) {
    const editor = editors[pane]
    if (editor && !(await editor.waitIdle())) {
      showToast('No se pudo guardar el archivo: la pestaña sigue abierta.', 'warning')
      return
    }
  }
  const next = removeTab(pane, path)
  if (path !== active) return
  if (pane === 'secondary' && !next) {
    await closeSplit()
    return
  }
  if (pane === 'primary') workspace.currentFilePath = next
  else workspace.secondaryFilePath = next
}

/** Closes the untitled tab: refused while it has text (it would be lost). */
export function closeUntitled() {
  if (editors.primary?.hasUntitledText()) return showToast(UNTITLED_BLOCKS.replace('otro proyecto', 'cerrarlo'), 'warning')
  workspace.untitled = false
  workspace.currentFilePath = tabs.primary.at(-1) ?? ''
}

/**
 * Editor mode (ActionBar menu, Ver menu): 'pdf' Solo PDF, 'minimal' one
 * code pane + viewer, 'multi' panes with tabs. Leaving 'multi' closes the
 * second pane (saving it first); the tabs stay for when 'multi' returns.
 * @param {import('./ui.svelte.js').EditorMode} mode
 */
export async function setEditorMode(mode) {
  ui.modeMenuOpen = false
  if (mode === ui.editorMode) return
  if (mode !== 'multi' && ui.splitEditor && !(await closeSplit())) return
  if (mode === 'pdf') setPreviewVisible(true)
  saveEditorMode(mode)
  if (mode !== 'pdf') editors.primary?.focus()
}

function focusPane(/** @type {import('./ui.svelte.js').EditorPane} */ pane) {
  ui.focusedPane = pane
  editors[pane]?.focus()
}

/** MIME type of a file dragged from the explorer (FileTreeItem → Workspace). */
export const FILE_DRAG_TYPE = 'application/x-creative-ink-path'

/**
 * Drag & drop into the editor area: opens `path` in the SECOND pane
 * (opening the split if needed), where the Editor reads it with LoadFile.
 * Only editable files of the open folder's tree are accepted: a drop can
 * carry any text. A file already shown in a pane is focused there instead
 * (never the same file in both panes).
 * @param {string} path absolute
 * @returns {boolean} whether the drop was accepted
 */
export function openInSecondaryPane(path) {
  if (!isEditableInTree(workspace.fileTree, path)) return false
  if (path === workspace.currentFilePath || paneOf(path) === 'primary') {
    focusPane('primary')
    if (path !== workspace.currentFilePath) openFile(path)
    return true
  }
  if (ui.editorMode !== 'multi') saveEditorMode('multi') // two panes side by side
  ui.splitEditor = true
  ui.focusedPane = 'secondary'
  addTab('secondary', path, workspace.secondaryFilePath)
  if (path === workspace.secondaryFilePath) editors.secondary?.focus()
  else workspace.secondaryFilePath = path
  return true
}

/** @param {import('./workspace.svelte.js').FileNode | null} node @param {string} path */
function isEditableInTree(node, path) {
  if (!node) return false
  if (!node.isDir) return node.editable && node.path === path
  return (node.children ?? []).some((child) => isEditableInTree(child, path))
}

/**
 * ⌘\ / Ctrl+\: shows or hides the second editor pane. Opening it focuses
 * it, so the next file chosen in the explorer goes there. Closing it saves
 * its pending edits first.
 */
export async function toggleSplitEditor() {
  if (!ui.splitEditor) {
    if (ui.editorMode !== 'multi') saveEditorMode('multi') // two panes = Multiventana
    ui.splitEditor = true
    ui.focusedPane = 'secondary'
    ui.sidebarOpen = true // the explorer is how a file gets into the new pane
    return
  }
  await closeSplit()
}

/** Closes the second pane after saving it; false if it could not be saved. */
async function closeSplit() {
  if (editors.secondary && !(await editors.secondary.waitIdle())) return false // keep it: unsaved
  ui.splitEditor = false
  workspace.secondaryFilePath = ''
  tabs.secondary = []
  focusPane('primary')
  return true
}

/**
 * ⌘N / Ctrl+N "Nuevo archivo…": native dialog inside the open project
 * (.tex by default; .bib, .sty, .txt… by extension). Without a project it
 * creates a quick document instead. The new file opens in the main pane.
 */
const MOD_S = navigator.platform.toLowerCase().includes('mac') ? '⌘S' : 'Ctrl+S'
export const UNTITLED_BLOCKS = `Guarda el documento sin título (${MOD_S}) o bórralo antes de abrir otro proyecto.`

/**
 * ⌘N / Ctrl+N: a new untitled document in this window's main pane, to write
 * at once; ⌘S saves it (the dialog opens in the open project). Without a
 * project, a quick document.
 */
export function newFile() {
  if (!workspace.currentFolderPath) return newQuickDocument()
  ui.startOpen = false
  ui.focusedPane = 'primary'
  if (workspace.untitled) return editors.primary?.focus()
  workspace.untitled = true
}

/** ⇧⌘N / Ctrl+Shift+N: another, independent Creative-ink window. */
export async function newWindow() {
  try {
    await NewWindow()
  } catch (err) {
    showToast(String(err), 'warning')
  }
}

/** Saves the untitled document of ⌘N; returns true if it was saved. */
async function saveUntitled() {
  const editor = editors.primary
  if (!editor) return false
  try {
    const state = await SaveNewDialog(editor.getContent())
    if (state.canceled) return false
    workspace.untitled = false
    editor.adopt(state.filePath) // same text, now a file: no reload
    applyWorkspace(state)
    return true
  } catch (err) {
    showToast(String(err), 'warning')
    return false
  }
}

/**
 * The explorer's "Nuevo archivo" / "Nueva carpeta" (its header and its
 * right-click menu): Go normalizes the name, keeps it inside the folder and
 * never replaces anything. A new file becomes the open document. Returns an
 * error message or ''.
 * @param {'file' | 'folder'} kind
 * @param {string} parent absolute folder of the tree
 * @param {string} name as typed
 */
export async function createInExplorer(kind, parent, name) {
  if (kind === 'file') {
    if (editors.primary?.hasUntitledText()) return UNTITLED_BLOCKS
    if (!(await persistPendingEdits())) return 'No se pudieron guardar los cambios del archivo abierto.'
  }
  try {
    const state = kind === 'file' ? await CreateFile(parent, name) : await CreateFolder(parent, name)
    setDirExpanded(parent, true)
    if (kind === 'file') {
      workspace.untitled = false
      applyWorkspace(state)
    } else {
      applyTree(state)
    }
    return ''
  } catch (err) {
    return String(err)
  }
}

/** ⌥⌘N / Ctrl+Alt+N (and ⌘N without a project): tmp-Workspaces/unnamed-tmp<N>/main.tex. */
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
  if (editors.primary?.isUntitled()) {
    if (await saveUntitled()) void RequestCompile('manual')
    return
  }
  // This compile replaces the ones the panes had scheduled after typing.
  for (const e of [editors.primary, editors.secondary]) e?.cancelScheduledCompile()
  if (!(await persistPendingEdits())) return // the PDF would not match the text
  // Only when an open file can change the PDF: saving a README.md never
  // compiles (Go decides, core.AffectsBuild). The toolbar's Compilar always does.
  const affects = [editors.primary, editors.secondary].some((e) => e?.affectsBuild())
  // "manual": always compiles (and is never cancelled), whatever compile.auto says.
  if (affects && compileTarget()) void RequestCompile('manual')
}

/**
 * Native "Guardar como…" for the main pane's file (it is Go's active
 * document; the second pane only edits files in place).
 */
export async function saveAs() {
  if (editors.primary?.isUntitled()) return void saveUntitled()
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
 * the target (main file or active document) whatever file is open: unlike ⌘S,
 * it compiles even after saving a README.md.
 */
export async function compileDocument() {
  for (const e of [editors.primary, editors.secondary]) e?.cancelScheduledCompile()
  if (editors.primary?.isUntitled()) return saveDocument()
  if (!(await persistPendingEdits())) return
  if (compileTarget()) void RequestCompile('manual') // explicit: whatever file is open
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
  'new-window': newWindow,
  'new-quick-doc': newQuickDocument,
  'open-folder': openFolder,
  start: () => (ui.startOpen = true),
  save: saveDocument,
  'save-as': saveAs,
  'toggle-sidebar': toggleSidebar,
  'toggle-terminal': () => toggleBottomTab('terminal'),
  'split-editor': toggleSplitEditor,
  'mode-pdf': () => setEditorMode('pdf'),
  'mode-minimal': () => setEditorMode('minimal'),
  'mode-multi': () => setEditorMode('multi'),
  'search-project': () => void import('./search.svelte.js').then((m) => m.focusSearch()),
  'toggle-preview': togglePreview,
  'preview-side': swapPreviewSide,
  'zoom-in': () => zoomBy(ZOOM_STEP),
  'zoom-out': () => zoomBy(-ZOOM_STEP),
  'zoom-reset': () => zoomBy(0)
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

/**
 * Opens the OS's own terminal in the open folder (Go picks the folder and
 * the emulator). Returns an error message or ''.
 */
export async function openSystemTerminal() {
  try {
    await OpenSystemTerminal()
    return ''
  } catch (err) {
    return String(err)
  }
}

/**
 * Trust banner: trust the folder's .latexmkrc as it is now (Go saves its
 * hash) and recompile with it. Returns an error message or ''.
 */
export async function trustProject() {
  try {
    applyTree(await TrustProject())
    return ''
  } catch (err) {
    return String(err)
  }
}

/**
 * Trust banner: keep the folder restricted (latexmk -norc) and stop asking.
 * Returns an error message or ''.
 */
export async function untrustProject() {
  try {
    applyTree(await UntrustProject())
    return ''
  } catch (err) {
    return String(err)
  }
}
