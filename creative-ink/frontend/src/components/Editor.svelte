<script>
  import { onMount } from 'svelte'
  import { EditorState } from '@codemirror/state'
  import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLine,
    highlightActiveLineGutter,
    drawSelection
  } from '@codemirror/view'
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
  import { StreamLanguage, bracketMatching, indentOnInput } from '@codemirror/language'
  import { stex } from '@codemirror/legacy-modes/mode/stex'
  import { oneDark } from '@codemirror/theme-one-dark'
  import { DocumentPath, LoadContent, SaveContent } from '../../wailsjs/go/main/App.js'

  // Inactivity window before autosaving. Each successful save also queues a
  // background compilation in Go, reported through Wails events (see App.svelte).
  const SAVE_DELAY_MS = 1000

  const LABELS = {
    loading: 'Cargando…',
    saved: 'Guardado',
    dirty: 'Sin guardar',
    saving: 'Guardando…',
    error: 'Error al guardar',
    loadError: 'Error al abrir el documento'
  }

  /** @type {HTMLDivElement} */
  let container
  /** @type {EditorView | null} */
  let view = null

  // Debounce / save-queue bookkeeping (not reactive: never rendered).
  let timer = null // pending debounce timeout
  let saving = false // a SaveContent call is in flight
  let pending = false // a save was requested while another was in flight

  /** @type {'loading' | 'saved' | 'dirty' | 'saving' | 'error' | 'loadError'} */
  let status = $state('loading')
  let errorMsg = $state('')
  let docPath = $state('')
  let fileName = $derived(docPath.split(/[\\/]/).pop() || '')

  // Called on every document change: restart the 1 s inactivity timer.
  // Keystrokes only reset a timer; nothing crosses the IPC bridge here.
  function scheduleSave() {
    status = 'dirty'
    clearTimeout(timer)
    timer = setTimeout(flush, SAVE_DELAY_MS)
  }

  // Sends the latest content to Go. At most one SaveContent is in flight;
  // requests arriving meanwhile are coalesced into a single follow-up save.
  async function flush() {
    clearTimeout(timer)
    timer = null
    if (!view) return
    if (saving) {
      pending = true
      return
    }

    saving = true
    status = 'saving'
    try {
      // The document is serialised once, at save time, not per keystroke.
      // Go saves and answers at once with { saved }; it does not wait for latexmk.
      await SaveContent(view.state.doc.toString())
      // If the user typed during the save, status is already 'dirty': keep it.
      if (status === 'saving') status = 'saved'
      errorMsg = ''
    } catch (err) {
      status = 'error'
      errorMsg = String(err)
    } finally {
      saving = false
      if (pending) {
        pending = false
        flush()
      }
    }
  }

  function createView(doc) {
    view = new EditorView({
      parent: container,
      state: EditorState.create({
        doc,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          drawSelection(),
          history(),
          indentOnInput(),
          bracketMatching(),
          StreamLanguage.define(stex),
          EditorView.lineWrapping,
          keymap.of([
            // Cmd/Ctrl+S saves immediately, skipping the debounce.
            { key: 'Mod-s', preventDefault: true, run: () => (flush(), true) },
            ...defaultKeymap,
            ...historyKeymap,
            indentWithTab
          ]),
          oneDark,
          EditorView.theme({
            '&': { height: '100%', fontSize: '14px' },
            '.cm-scroller': { fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace' }
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) scheduleSave()
          })
        ]
      })
    })
    view.focus()
  }

  onMount(() => {
    let destroyed = false

    ;(async () => {
      try {
        const [content, path] = await Promise.all([LoadContent(), DocumentPath()])
        if (destroyed) return
        docPath = path
        createView(content)
        status = 'saved'
        // Save once on start so Go compiles what is on disk and the preview is not empty.
        if (content.trim()) flush()
      } catch (err) {
        // Never mount an empty editor if loading failed: the first
        // autosave would overwrite the real file with nothing.
        status = 'loadError'
        errorMsg = String(err)
      }
    })()

    return () => {
      destroyed = true
      if (timer) flush() // persist edits still inside the debounce window
      view?.destroy()
      view = null
    }
  })
</script>

<div class="editor">
  <div class="surface" bind:this={container}></div>
  <footer class="statusbar">
    <span class="file" title={docPath}>{fileName}</span>
    <span class="status status-{status}" title={errorMsg}>{LABELS[status]}</span>
  </footer>
</div>

<style>
  .editor {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .surface {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .statusbar {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.25rem 0.75rem;
    border-top: 1px solid #333;
    background: #1b1d23;
    font-size: 0.75rem;
    color: #9d9d9d;
  }

  .file {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-saved {
    color: #4ade80;
  }

  .status-dirty,
  .status-saving {
    color: #fbbf24;
  }

  .status-error,
  .status-loadError {
    color: #f87171;
  }
</style>
