<script>
  import { FILE_DRAG_TYPE, openInSecondaryPane } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { setSplitRatio, ui } from '../lib/ui.svelte.js'
  import { compileTarget } from '../lib/workspace.svelte.js'
  import BottomPanel from './BottomPanel.svelte'
  import Editor from './Editor.svelte'
  import Preview from './Preview.svelte'
  import TrustBanner from './TrustBanner.svelte'

  // Drag & drop from the explorer: dropping a file on the editor area opens
  // it in the second pane. Capture-phase handlers so CodeMirror never sees
  // these drags (it would insert the path as text); any other drag (e.g.
  // moving text inside the editor) passes through untouched.
  let dropActive = $state(false)
  let dragDepth = 0 // enter/leave pairs of the nested elements

  /** @param {DragEvent} event */
  function isFileDrag(event) {
    return event.dataTransfer?.types.includes(FILE_DRAG_TYPE) ?? false
  }

  /** @param {DragEvent} event */
  function ondragentercapture(event) {
    if (!isFileDrag(event)) return
    event.preventDefault()
    event.stopPropagation()
    dragDepth++
    dropActive = true
  }

  /** @param {DragEvent} event */
  function ondragovercapture(event) {
    if (!isFileDrag(event)) return
    event.preventDefault() // allows the drop
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  }

  /** @param {DragEvent} event */
  function ondragleavecapture(event) {
    if (!isFileDrag(event)) return
    event.stopPropagation()
    dragDepth = Math.max(0, dragDepth - 1)
    if (dragDepth === 0) dropActive = false
  }

  /** @param {DragEvent} event */
  function ondropcapture(event) {
    if (!isFileDrag(event)) return
    event.preventDefault()
    event.stopPropagation()
    dragDepth = 0
    dropActive = false
    const path = event.dataTransfer?.getData(FILE_DRAG_TYPE) || event.dataTransfer?.getData('text/plain') || ''
    openInSecondaryPane(path)
  }

  // ── Resizable split: the divider between the two editor panes ───────────
  /** @type {HTMLDivElement} */
  let editorsEl
  let resizing = $state(false)
  const KEY_STEP = 0.05

  // Two panes with the dragged proportion and a 4px divider between them.
  let columns = $derived(
    ui.splitEditor ? `minmax(0, ${ui.splitRatio}fr) 4px minmax(0, ${1 - ui.splitRatio}fr)` : 'minmax(0, 1fr)'
  )

  /** @param {MouseEvent} event */
  function onDividerMouseDown(event) {
    if (event.button !== 0) return
    event.preventDefault() // no text selection while dragging
    const rect = editorsEl.getBoundingClientRect()
    resizing = true
    /** @param {MouseEvent} e */
    const onmousemove = (e) => setSplitRatio((e.clientX - rect.left) / rect.width)
    const onmouseup = () => {
      resizing = false
      window.removeEventListener('mousemove', onmousemove)
      window.removeEventListener('mouseup', onmouseup)
      setSplitRatio(ui.splitRatio, true) // remember it across sessions
    }
    window.addEventListener('mousemove', onmousemove)
    window.addEventListener('mouseup', onmouseup)
  }

  /** Keyboard: ←/→ move the divider, Home centres it. @param {KeyboardEvent} event */
  function onDividerKeyDown(event) {
    const delta = { ArrowLeft: -KEY_STEP, ArrowRight: KEY_STEP }[event.key]
    if (delta) setSplitRatio(ui.splitRatio + delta, true)
    else if (event.key === 'Home') setSplitRatio(0.5, true)
    else return
    event.preventDefault()
  }

  // A drag cancelled outside (Esc, dropped elsewhere) must clear the hint.
  function ondragend() {
    dragDepth = 0
    dropActive = false
  }
</script>

<svelte:window {ondragend} />

<!-- Layout only: the main Editor and the Preview are never re-keyed or put
     inside {#if}, so CodeMirror and the PDF.js canvas keep their state across
     UI changes. The second pane (split editor, ⌘\) is mounted only while
     split: closing it saves its edits first (actions.js toggleSplitEditor). -->
<div class="workspace" class:split={ui.splitEditor} class:resizing>
  <section class="pane editor-pane" aria-label="Editor">
    <!-- Untrusted .latexmkrc: compiling with -norc; offers to trust it. -->
    <TrustBanner />
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="editors"
      bind:this={editorsEl}
      style:grid-template-columns={columns}
      style:--hint-left={ui.splitEditor ? `calc(${ui.splitRatio * 100}% + 6px)` : '50%'}
      {ondragentercapture}
      {ondragovercapture}
      {ondragleavecapture}
      {ondropcapture}
    >
      <div class="editor-host">
        <Editor pane="primary" />
      </div>
      {#if ui.splitEditor}
        <!-- A focusable separator is an ARIA widget: it takes the keyboard. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
        <div
          class="divider"
          role="separator"
          aria-orientation="vertical"
          aria-label="Ancho de los editores"
          aria-valuemin="20"
          aria-valuemax="80"
          aria-valuenow={Math.round(ui.splitRatio * 100)}
          tabindex="0"
          title="Arrastra para cambiar el ancho · doble clic para centrar"
          onmousedown={onDividerMouseDown}
          onkeydown={onDividerKeyDown}
          ondblclick={() => setSplitRatio(0.5, true)}
        ></div>
        <div class="editor-host secondary" aria-label="Segundo editor">
          <Editor pane="secondary" />
        </div>
      {/if}
      <!-- Where the dropped file will open: the right half (second pane). -->
      <div class="drop-hint" class:visible={dropActive} aria-hidden="true">
        <span>Soltar para abrir al lado</span>
      </div>
    </div>
    <!-- "Errores LaTeX" | "Terminal". Always mounted; hides itself. -->
    <BottomPanel />
  </section>

  <section class="pane" aria-label="Vista previa">
    <Preview
      compileState={compilation.status}
      pdfVersion={compilation.pdfVersion}
      documentKey={compileTarget()}
    />
  </section>
</div>

<style>
  .workspace {
    height: 100%;
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  /* Two editors share two thirds of the width; the preview keeps one. */
  .workspace.split {
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
  }

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .editor-pane {
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--border);
  }

  .editors {
    position: relative;
    flex: 1;
    min-height: 0;
    display: grid; /* columns come from splitRatio (inline style) */
  }

  .editor-host {
    min-width: 0;
    min-height: 0;
  }

  /* 4px hit area with a 1px hairline in the middle; it lights up on hover,
     while dragging and on keyboard focus. */
  .divider {
    position: relative;
    cursor: col-resize;
    background: transparent;
    transition: background-color var(--transition);
  }

  .divider::before {
    content: '';
    position: absolute;
    inset: 0 auto 0 50%;
    width: 1px;
    background: var(--border);
  }

  .divider:hover,
  .resizing .divider {
    background: var(--surface-hover);
  }

  .divider:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }

  /* While dragging: the cursor stays col-resize everywhere and CodeMirror
     does not start selecting text under the pointer. */
  .resizing {
    cursor: col-resize;
    user-select: none;
  }

  .resizing .editor-host {
    pointer-events: none;
  }

  .drop-hint {
    position: absolute;
    inset: 8px 8px 8px var(--hint-left);
    display: grid;
    place-items: center;
    border: 1px dashed var(--border-strong);
    border-radius: var(--radius-md);
    background: var(--surface-active);
    color: var(--text-secondary);
    font-size: var(--text-md);
    pointer-events: none;
    opacity: 0;
    transition: opacity var(--transition);
  }

  .drop-hint.visible {
    opacity: 1;
  }
</style>
