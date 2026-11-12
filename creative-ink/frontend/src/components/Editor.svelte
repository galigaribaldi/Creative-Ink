<script>
  import { onMount, untrack } from 'svelte'
  import { Compartment, EditorState } from '@codemirror/state'
  import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLine,
    highlightActiveLineGutter,
    highlightSpecialChars,
    drawSelection,
    dropCursor,
    rectangularSelection
  } from '@codemirror/view'
  import { defaultKeymap, history, historyKeymap, indentWithTab, toggleComment } from '@codemirror/commands'
  import { bracketMatching, indentOnInput } from '@codemirror/language'
  import {
    search,
    searchKeymap,
    highlightSelectionMatches,
    openSearchPanel,
    findNext,
    findPrevious,
    selectNextOccurrence,
    gotoLine
  } from '@codemirror/search'
  import { LoadFile, OpenFile, RequestCompile, SaveContent } from '../../wailsjs/go/main/App.js'
  import { registerEditor } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { config } from '../lib/config.svelte.js'
  import { editorTheme } from '../lib/editor-theme.js'
  import { languageFor } from '../lib/languages.js'
  import { sync, syncForward } from '../lib/synctex.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
  import { baseName, compileTarget, isTeXPath, workspace } from '../lib/workspace.svelte.js'

  /**
   * pane: 'primary' is Go's active document (OpenFile: it decides what is
   * compiled); 'secondary' is the second pane of the split editor
   * (LoadFile: it only edits and saves in place).
   * @type {{ pane?: import('../lib/ui.svelte.js').EditorPane }}
   */
  let { pane = 'primary' } = $props()
  // A pane never changes role: read once.
  const isPrimary = untrack(() => pane) === 'primary'

  // Two inactivity windows, both restarted by every keystroke:
  //  - 1 s → save to disk (cheap: protects the text early).
  //  - adaptive → ask Go to compile (expensive). The delay is
  //    max(3 s, 2 × last compile duration) from compilation.svelte.js, and
  //    auto-compile pauses for heavy documents (> 10 s).
  //    Cmd/Ctrl+S skips both and does both now.
  const SAVE_DELAY_MS = 1000
  const IS_MAC = navigator.platform.toUpperCase().includes('MAC')
  // macOS shows ⌘S, Linux/Windows Ctrl+S in the paused-compile notice.
  const SAVE_SHORTCUT = IS_MAC ? '⌘S' : 'Ctrl+S'

  // Commands the native menu (macOS) can run in the focused pane.
  const COMMANDS = {
    find: openSearchPanel,
    'find-next': findNext,
    'find-previous': findPrevious,
    'select-next': selectNextOccurrence,
    'goto-line': gotoLine,
    'toggle-comment': toggleComment
  }

  // Spanish labels for the search panel and "go to line" dialog.
  const PHRASES = {
    Find: 'Buscar',
    Replace: 'Reemplazar',
    next: 'siguiente',
    previous: 'anterior',
    all: 'todas',
    'match case': 'mayúsculas',
    regexp: 'expresión regular',
    'by word': 'palabra completa',
    replace: 'reemplazar',
    'replace all': 'reemplazar todas',
    close: 'cerrar',
    'current match': 'coincidencia actual',
    'replaced $ matches': '$ coincidencias reemplazadas',
    'replaced match on line $': 'coincidencia reemplazada en la línea $',
    'on line': 'en la línea',
    'Go to line': 'Ir a la línea',
    go: 'ir'
  }

  const LABELS = {
    empty: 'Sin documento',
    loading: 'Abriendo…',
    saved: 'Guardado',
    dirty: 'Sin guardar',
    saving: 'Guardando…',
    error: 'Error al guardar',
    loadError: 'No se pudo abrir el archivo'
  }

  /** @type {HTMLDivElement} */
  let container
  /** @type {EditorView | null} */
  let view = null
  // Read-only while there is no document or a file switch is in progress.
  const editability = new Compartment()

  // ── Save bookkeeping (non-reactive: never rendered) ──────────────────────
  // docFile is the file whose text is in CodeMirror. Every save captures it
  // at flush time, so text can never be written to another file.
  let docFile = ''
  let timer = null // pending save debounce
  let compileTimer = null // pending compile debounce
  let saving = false // a SaveContent call is in flight
  let pending = false // a save was requested while another was in flight
  let dirty = false // the editor holds edits not yet sent to Go
  /** @type {((clean: boolean) => void)[]} */
  let idleWaiters = []
  let switchSeq = 0 // identifies the latest file switch

  /** @type {keyof typeof LABELS} */
  let status = $state('empty')
  let errorMsg = $state('')
  let fileName = $state('')
  let filePath = $state('')

  // Tooltip of the large-project warning ("" = no warning).
  let healthTooltip = $derived(
    workspace.healthWarnings.length
      ? [...workspace.healthWarnings.map((w) => `• ${w.message}`), '', 'Solo es un aviso: no cambia nada.'].join('\n')
      : ''
  )

  // Called on every document change: restart both inactivity timers.
  // Keystrokes only reset timers; nothing crosses the IPC bridge here.
  function scheduleSave() {
    dirty = true
    status = 'dirty'
    clearTimeout(timer)
    timer = setTimeout(flush, SAVE_DELAY_MS)
    clearTimeout(compileTimer)
    compileTimer = null
    // Read at scheduling time: the delay adapts after every compilation.
    // compile.auto off (the default): typing never compiles; ⌘S does.
    if (config.autoCompile && !compilation.autoCompilePaused) {
      compileTimer = setTimeout(saveAndCompile, compilation.autoCompileDelayMs)
    }
  }

  /**
   * Saves pending edits now and, once they are on disk, asks Go to compile
   * (Go compiles the project's main file when there is one). Used by
   * Cmd/Ctrl+S, the toolbar and the 3 s idle timer. A failed save does not
   * compile: the PDF would not match the text.
   */
  async function saveAndCompile() {
    clearTimeout(compileTimer)
    compileTimer = null
    const clean = await waitIdle()
    // Nothing to compile without a document (e.g. behind the first-run screen).
    // "auto": Go rejects it if compile.auto was turned off meanwhile, and
    // may cancel it if a newer request makes it obsolete.
    if (clean && compileTarget()) void RequestCompile('auto')
  }

  function cancelScheduledCompile() {
    clearTimeout(compileTimer)
    compileTimer = null
  }

  // Sends the latest content of docFile to Go. At most one SaveContent is in
  // flight; requests arriving meanwhile are coalesced into one follow-up.
  async function flush() {
    clearTimeout(timer)
    timer = null
    if (saving) {
      pending = true
      return
    }
    if (!view || !docFile || !dirty) return settle()

    saving = true
    dirty = false
    status = 'saving'
    const path = docFile
    try {
      // Go saves and answers at once with { saved }; it does not wait for latexmk.
      await SaveContent(path, view.state.doc.toString())
      // If the user typed during the save, status is already 'dirty': keep it.
      if (status === 'saving') status = 'saved'
      errorMsg = ''
    } catch (err) {
      dirty = true // still unsaved: the next edit or Cmd+S retries
      status = 'error'
      errorMsg = String(err)
    } finally {
      saving = false
      if (pending) {
        pending = false
        flush()
      } else {
        settle()
      }
    }
  }

  // Resolves waitIdle() callers once nothing is pending.
  function settle() {
    if (saving || pending || timer) return
    const clean = !dirty
    idleWaiters.splice(0).forEach((resolve) => resolve(clean))
  }

  /** Saves pending edits now; resolves true if everything reached the disk. */
  function waitIdle() {
    const done = new Promise((resolve) => idleWaiters.push(resolve))
    flush()
    return done
  }

  function editabilityFor(canEdit) {
    return [EditorView.editable.of(canEdit), EditorState.readOnly.of(!canEdit)]
  }

  function setEditable(canEdit) {
    view?.dispatch({ effects: editability.reconfigure(editabilityFor(canEdit)) })
  }

  /** Selects the file of this pane ("" = none) in the shared state. */
  function select(path) {
    if (isPrimary) workspace.currentFilePath = path
    else workspace.secondaryFilePath = path
  }

  /**
   * Basic editor commands, all built into CodeMirror (no shortcuts of our own
   * except SyncTeX): undo/redo, find & replace (⌘F, ⌘G, ⇧⌘G), go to line
   * (⌥⌘G), next occurrence (⌘D), extra cursors with ⌥-click and column
   * selection with ⇧⌥-drag (like VS Code; ⌘-click stays SyncTeX), toggle
   * comment (⌘/), move/copy lines (⌥↑/↓, ⇧⌥↑/↓), delete line (⇧⌘K), indent
   * (⌘[ / ⌘]). On Linux/Windows ⌘ is Ctrl and ⌥ is Alt.
   */
  function createState(doc, canEdit, path = '') {
    return EditorState.create({
      doc,
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        highlightSpecialChars(),
        drawSelection(),
        dropCursor(),
        EditorState.allowMultipleSelections.of(true),
        EditorView.clickAddsSelectionRange.of((event) => event.altKey && !event.shiftKey),
        rectangularSelection({ eventFilter: (event) => event.altKey && event.shiftKey }),
        history(),
        indentOnInput(),
        bracketMatching(),
        highlightSelectionMatches(),
        search({ top: true }),
        EditorState.phrases.of(PHRASES),
        languageFor(path),
        EditorView.lineWrapping,
        editability.of(editabilityFor(canEdit)),
        // SyncTeX forward: ⌘-click (Ctrl-click on Linux/Windows) on a line.
        EditorView.domEventHandlers({
          focus() {
            ui.focusedPane = pane
            return false
          },
          mousedown(event, v) {
            const mod = IS_MAC ? event.metaKey : event.ctrlKey
            if (!mod || event.button !== 0 || !isTeXPath(docFile)) return false
            const pos = v.posAtCoords({ x: event.clientX, y: event.clientY })
            if (pos == null) return false
            event.preventDefault()
            void syncForward(docFile, v.state.doc.lineAt(pos).number)
            return true
          }
        }),
        keymap.of([
          // SyncTeX forward from the cursor line, without the mouse.
          {
            key: 'Mod-Alt-j',
            preventDefault: true,
            run: (v) => {
              if (isTeXPath(docFile)) void syncForward(docFile, v.state.doc.lineAt(v.state.selection.main.head).number)
              return true
            }
          },
          ...defaultKeymap,
          ...historyKeymap,
          ...searchKeymap,
          indentWithTab
        ]),
        // Tokens only (lib/editor-theme.js): follows the light/dark theme
        // through CSS variables, without reconfiguring the view.
        editorTheme,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) scheduleSave()
        })
      ]
    })
  }

  /** Shows `content` as the document `path`. setState fires no update listener (no autosave) and starts a fresh undo history for the file. */
  function load(path, content, target = '') {
    clearTimeout(compileTimer) // OpenFile already queued a compile in Go
    compileTimer = null
    docFile = path
    dirty = false
    view.setState(createState(content, path !== '', path))
    // Go's compile target without a main file is the last .tex of this pane.
    if (isPrimary && isTeXPath(path)) workspace.texFilePath = path
    // What Go compiles now (a chapter of a collection: its document's main.tex).
    if (isPrimary && target) workspace.compileTargetPath = target
    adopt(path)
    status = path ? 'saved' : 'empty'
    errorMsg = ''
    if (path) view.focus()
    // A backward search may have asked for a line of this file.
    if (sync.reveal && sync.reveal.file === path) revealLine(sync.reveal.line)
  }

  /** Selects line n (1-based) and scrolls it to the middle (SyncTeX backward). */
  function revealLine(n) {
    if (!view) return
    const line = view.state.doc.line(Math.min(Math.max(1, n), view.state.doc.lines))
    view.dispatch({
      selection: { anchor: line.from, head: line.to },
      effects: EditorView.scrollIntoView(line.from, { y: 'center' })
    })
    view.focus()
    sync.reveal = null
  }

  /** Renames the current document in place (Save As): same text, new file. */
  function adopt(path) {
    docFile = path
    filePath = path
    fileName = baseName(path)
  }

  /** Switches the editor to `target`, saving the current file first. */
  async function switchTo(target) {
    if (!view || target === docFile) return
    const seq = ++switchSeq
    setEditable(false)

    const clean = await waitIdle()
    if (seq !== switchSeq) return
    if (!clean) {
      // The current file could not be saved: stay on it instead of losing edits.
      sync.reveal = null // a pending SyncTeX jump must not fire later
      setEditable(docFile !== '')
      select(docFile)
      return
    }

    if (!target) return load('', '')
    status = 'loading'
    try {
      // The main pane makes it Go's active document; the second one only reads it.
      const file = await (isPrimary ? OpenFile(target) : LoadFile(target))
      if (seq !== switchSeq) return
      load(file.path, file.content, isPrimary ? file.compileTarget : '')
    } catch (err) {
      if (seq !== switchSeq) return
      status = 'loadError'
      errorMsg = String(err)
      sync.reveal = null
      setEditable(docFile !== '')
      // Keep the explorer selection truthful: the old file is still open.
      select(docFile)
    }
  }

  // Declared before the $effect below: effects run in order, so the view
  // exists when the first file switch happens.
  onMount(() => {
    view = new EditorView({ parent: container, state: createState('', false) })
    const unregister = registerEditor(pane, {
      save: flush,
      cancelScheduledCompile,
      waitIdle,
      isDirty: () => dirty || saving || timer !== null,
      getContent: () => view?.state.doc.toString() ?? '',
      adopt,
      focus: () => view?.focus(),
      run: (id) => {
        const command = COMMANDS[id]
        if (!view || !command) return false
        view.focus()
        return command(view)
      }
    })

    return () => {
      unregister()
      clearTimeout(compileTimer)
      switchSeq++ // abandon an in-flight switch
      if (dirty) flush() // persist edits still inside the debounce window
      view?.destroy()
      view = null
    }
  })

  // SyncTeX backward into the file already open. For another file, load()
  // reveals the line once the switch finishes.
  $effect(() => {
    const reveal = sync.reveal
    if (reveal) untrack(() => reveal.file === docFile && revealLine(reveal.line))
  })

  // A heavy result pauses auto-compile, and `ci config autocompile off`
  // disables it: drop a compile already scheduled so it does not fire.
  $effect(() => {
    if (compilation.autoCompilePaused || !config.autoCompile) {
      untrack(() => {
        clearTimeout(compileTimer)
        compileTimer = null
      })
    }
  })

  // The only place where the editor's document changes: when the selected
  // file changes. CodeMirror itself is never remounted.
  $effect(() => {
    const target = isPrimary ? workspace.currentFilePath : workspace.secondaryFilePath
    untrack(() => void switchTo(target))
  })
</script>

<div class="editor" class:focused={ui.splitEditor && ui.focusedPane === pane}>
  <div class="surface" bind:this={container}></div>
  {#if status === 'empty'}
    {#if isPrimary}
      <p class="empty">Abre una carpeta y elige un archivo (<code>.tex</code>, <code>.bib</code>…) en el explorador.</p>
    {:else}
      <p class="empty">Elige en el explorador el archivo que quieres ver junto al otro.</p>
    {/if}
  {/if}
  <footer class="statusbar">
    <span class="file" title={filePath}>{fileName}</span>
    {#if isPrimary && healthTooltip}
      <!-- Large-project detector: only a warning with a tooltip, it changes nothing. -->
      <span class="health" role="note" title={healthTooltip} aria-label={healthTooltip}>⚠ Proyecto grande</span>
    {/if}
    {#if isPrimary && !config.autoCompile && status !== 'empty'}
      <span class="auto-off" title="Actívala en la terminal: ci config autocompile on (o --project solo para este proyecto)">
        Compilación automática: off · {SAVE_SHORTCUT} compila
      </span>
    {:else if compilation.autoCompilePaused && status !== 'empty'}
      <span
        class="paused"
        role="status"
        title="La última compilación tardó {(compilation.durationMs / 1000).toFixed(1)} s"
      >
        Auto-compilación pausada (Documento pesado). Usa {SAVE_SHORTCUT} para compilar
      </span>
    {/if}
    <span class="status status-{status}" title={errorMsg}>{LABELS[status]}</span>
  </footer>
</div>

<style>
  .editor {
    position: relative;
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--surface-editor);
  }

  /* Split editor: a thin accent marks the pane the explorer opens files in
     (A2, accent rule 2: the focused pane). */
  .editor.focused {
    box-shadow: inset 0 2px 0 var(--accent);
  }

  .surface {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .empty {
    position: absolute;
    inset: 0 0 1.6rem;
    display: grid;
    place-items: center;
    margin: 0;
    padding: 2rem;
    text-align: center;
    font-size: var(--text-md);
    color: var(--text-muted);
    pointer-events: none;
  }

  .empty code {
    font-family: var(--font-mono);
    color: var(--text-secondary);
  }

  /* Chrome: quiet, and stable while typing (A1: items keep their slot). */
  .statusbar {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.25rem 0.75rem;
    border-top: 1px solid var(--border);
    background: var(--surface-base);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .paused {
    flex-shrink: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--warning);
  }

  .health {
    flex-shrink: 0;
    color: var(--warning);
    cursor: help;
  }

  .auto-off {
    flex-shrink: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    cursor: help;
  }

  .file {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .status {
    flex-shrink: 0;
  }

  /* Quiet success, visible problems (A1). */
  .status-saved {
    color: var(--text-muted);
  }

  .status-dirty,
  .status-saving,
  .status-loading {
    color: var(--text-secondary);
  }

  .status-error,
  .status-loadError {
    color: var(--danger);
  }
</style>
