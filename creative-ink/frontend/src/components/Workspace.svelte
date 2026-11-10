<script>
  import { FILE_DRAG_TYPE, openInSecondaryPane } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
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
<div class="workspace" class:split={ui.splitEditor}>
  <section class="pane editor-pane" aria-label="Editor">
    <!-- Untrusted .latexmkrc: compiling with -norc; offers to trust it. -->
    <TrustBanner />
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="editors"
      {ondragentercapture}
      {ondragovercapture}
      {ondragleavecapture}
      {ondropcapture}
    >
      <div class="editor-host">
        <Editor pane="primary" />
      </div>
      {#if ui.splitEditor}
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
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(0, 1fr);
  }

  .editor-host {
    min-width: 0;
    min-height: 0;
  }

  .editor-host.secondary {
    border-left: 1px solid var(--border);
  }

  .drop-hint {
    position: absolute;
    inset: 8px 8px 8px 50%;
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
