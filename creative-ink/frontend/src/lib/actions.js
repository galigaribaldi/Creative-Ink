// User actions triggered from anywhere in the layout (ActionBar, shortcuts).
// The Editor registers its commands here, so layout components can save or
// compile without holding a reference to CodeMirror and without prop drilling.
import { RequestCompile } from '../../wailsjs/go/main/App.js'

/**
 * @typedef {{ save: () => void, isDirty: () => boolean }} EditorCommands
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

/** Saves now, skipping the autosave debounce. A save also queues a compile. */
export function saveDocument() {
  editor?.save()
}

/**
 * Compiles the current document. Unsaved edits are saved first (the save
 * queues the compilation itself); otherwise Go compiles what is on disk.
 */
export function compileDocument() {
  if (editor?.isDirty()) {
    editor.save()
  } else {
    void RequestCompile()
  }
}
