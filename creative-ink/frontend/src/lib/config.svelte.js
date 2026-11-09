// Mirror of Go's configuration service (internal/config, docs/MAP.md §4.6).
// Go is the source of truth: values arrive in WorkspaceState (on every
// folder change) and through the config:changed event (e.g. after
// `ci config autocompile on` in the terminal). Nothing here is persisted.
import { EventsOn } from '../../wailsjs/runtime/runtime.js'

export const CONFIG_CHANGED = 'config:changed'

export const config = $state({
  /**
   * compile.auto, effective for the open project (default off). When off,
   * the editor never schedules a compilation: only ⌘S, the Compilar button
   * and `ci compile` compile. Go also rejects automatic requests.
   */
  autoCompile: false
})

/**
 * Applies config:changed events for the open folder. Call once during App
 * init and call the returned function on destroy.
 * @param {() => string} currentFolder the folder the UI shows
 * @returns {() => void} unsubscribe
 */
export function listenConfigEvents(currentFolder) {
  return EventsOn(
    CONFIG_CHANGED,
    (/** @type {{ id: string, value: unknown, folder: string }} */ event) => {
      // The effective value depends on the project layer of `folder`.
      if (event.folder !== currentFolder()) return
      if (event.id === 'compile.auto') config.autoCompile = event.value === true
    }
  )
}
