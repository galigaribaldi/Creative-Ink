// "Importar proyecto" (docs/MAP.md §4.7, phase 2). Go analyses the folder
// (InspectImport), keeps its path under an id and copies it in the
// background (import:progress / import:finished). This module only mirrors
// that: the summary, the progress and the outcome.
import { CancelImport, ImportProject, InspectImport } from '../../wailsjs/go/main/App.js'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { openImportInPlace, openProject } from './actions.js'

export const IMPORT_PROGRESS = 'import:progress'
export const IMPORT_FINISHED = 'import:finished'

/**
 * @typedef {{ file: string, line: number, ref: string, exists: boolean }} ExternalRef
 * @typedef {{
 *   canceled: boolean, id: string, source: string, proposedName: string, files: number, bytes: number,
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
