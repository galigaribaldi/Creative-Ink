// "Importar proyecto" (docs/MAP.md §4.7, phase 2). Go analyses the folder
// (InspectImport), keeps its path under an id and copies it in the
// background (import:progress / import:finished). This module only mirrors
// that: the summary, the progress and the outcome.
import { CancelImport, ImportProject, InspectDrop, InspectImport } from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { openFile, openImportInPlace, openInSecondaryPane, openProject } from './actions.js'
import { showToast } from './ui.svelte.js'

export const IMPORT_PROGRESS = 'import:progress'
export const IMPORT_FINISHED = 'import:finished'
export const FILE_DROP = 'drop:paths' // files/folders dropped from the system (drop_app.go)

/**
 * @typedef {{ file: string, line: number, ref: string, exists: boolean }} ExternalRef
 * @typedef {{
 *   canceled: boolean, id: string, source: string, file: string, inPlaceSingle: boolean, proposedName: string, files: number, bytes: number,
 *   hasGit: boolean, gitRoot: string, gitFiles: number, gitBytes: number,
 *   documents: string[], documentCount: number, collection: boolean, mainCandidates: string[],
 *   externalRefs: ExternalRef[], externalRefCount: number, notUtf8: string[], symlinks: string[], rcFiles: string[],
 *   freeBytes: number, large: boolean, canCopy: boolean, copyBlocked: string,
 *   notices: import('./workspace.svelte.js').LocationNotice[]
 * }} ImportPreview
 * @typedef {{ files: number, bytes: number, totalFiles: number, totalBytes: number }} ImportProgress
 */

export const importer = $state({
  /** The summary on screen (null = closed). @type {ImportPreview | null} */
  preview: null,
  /** Opened by a drop: the summary asks "¿Qué desea hacer?". */
  fromDrop: false,
  copying: false,
  /** @type {ImportProgress} */
  progress: { files: 0, bytes: 0, totalFiles: 0, totalBytes: 0 },
  error: ''
})

/**
 * Asks Go for a folder and its analysis. Returns an error message or ''
 * (the start screen shows it; a closed dialog is not an error).
 */
export async function startImport() {
  try {
    const preview = await InspectImport()
    if (!preview.canceled) {
      importer.preview = preview
      importer.fromDrop = false
      importer.error = ''
    }
    return ''
  } catch (err) {
    return String(err)
  }
}

/** Starts the copy; the outcome arrives with import:finished. */
export async function copyImport(/** @type {{ name: string, mainFile: string, includeGit: boolean }} */ choice) {
  if (!importer.preview || importer.copying) return
  importer.error = ''
  try {
    await ImportProject({ id: importer.preview.id, ...choice })
    importer.copying = true
    importer.progress = { files: 0, bytes: 0, totalFiles: 0, totalBytes: 0 }
  } catch (err) {
    importer.error = String(err)
  }
}

/** Opens the analysed folder where it is (always possible). */
export async function openInPlace() {
  if (!importer.preview || importer.copying) return
  const error = await openImportInPlace(importer.preview.id)
  if (error) importer.error = error
  else closeImport()
}

export function cancelImport() {
  if (importer.copying) void CancelImport()
}

export function closeImport() {
  if (importer.copying) return // cancel first
  importer.preview = null
  importer.error = ''
}

/**
 * Subscribes to the import events. Call once during App init and call the
 * returned function on destroy.
 * @returns {() => void} unsubscribe
 */
export function listenImportEvents() {
  const offs = [
    EventsOn(IMPORT_PROGRESS, (/** @type {ImportProgress & { id: string }} */ event) => {
      if (event.id === importer.preview?.id) importer.progress = event
    }),
    EventsOn(IMPORT_FINISHED, async (/** @type {{ id: string, success: boolean, canceled: boolean, error: string, folder: string }} */ event) => {
      if (event.id !== importer.preview?.id) return
      importer.copying = false
      if (event.success) {
        // A project of the repository now: opened like any other.
        const error = await openProject(event.folder)
        if (error) importer.error = error
        else closeImport()
      } else {
        importer.error = event.canceled ? 'Copia cancelada: no se creó nada.' : `No se pudo importar: ${event.error}`
      }
    })
  ]
  return () => offs.forEach((off) => off())
}

/**
 * @typedef {{ id: string, kind: 'open' | 'in-place' | 'choose' | 'rejected', path: string, name: string,
 *   isDir: boolean, extra: number, message: string, x: number, y: number }} FileDrop
 */

/**
 * Files and folders dropped from the system (Go classifies them; the path
 * stays in Go except for files of the open folder). Call once during App
 * init and call the returned function on destroy.
 * @returns {() => void} unsubscribe
 */
export function listenDropEvents() {
  return EventsOn(FILE_DROP, async (/** @type {FileDrop} */ drop) => {
    if (drop.extra > 0) showToast(`Se usa solo ${drop.name}: arrastra los elementos de uno en uno.`)
    switch (drop.kind) {
      case 'rejected':
        showToast(drop.message, 'warning')
        return
      case 'open': {
        // A file of the open folder: the pane under the cursor.
        const pane = document.elementFromPoint(drop.x, drop.y)?.closest('[data-pane]')?.getAttribute('data-pane')
        if (pane !== 'secondary' || !openInSecondaryPane(drop.path)) openFile(drop.path)
        return
      }
      case 'in-place': {
        // A project of the repository (or one of its files): no question.
        const error = await openImportInPlace(drop.id)
        if (error) showToast(error, 'warning')
        return
      }
      case 'choose':
        if (importer.copying) {
          showToast('Hay una importación en curso: espera a que termine o cancélala.', 'warning')
          return
        }
        try {
          importer.preview = await InspectDrop(drop.id)
          importer.fromDrop = true
          importer.error = ''
        } catch (err) {
          showToast(String(err), 'warning')
        }
    }
  })
}

/**
 * A file dragged from the system over the editor must never reach
 * CodeMirror: it would insert the file's text into the open document (and
 * the autosave would write it). preventDefault in the capture phase is
 * enough (CodeMirror skips prevented events); propagation goes on, because
 * on Windows Wails' own listener passes the paths to Go.
 * @returns {() => void} remove
 */
export function guardSystemDrops() {
  const guard = (/** @type {DragEvent} */ event) => {
    if (event.dataTransfer?.types.includes('Files')) event.preventDefault()
  }
  window.addEventListener('dragover', guard, true)
  window.addEventListener('drop', guard, true)
  return () => {
    window.removeEventListener('dragover', guard, true)
    window.removeEventListener('drop', guard, true)
  }
}
