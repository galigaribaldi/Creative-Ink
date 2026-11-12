<script>
  import { FILE_DRAG_TYPE, openInSecondaryPane } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { resetLayoutRatio, setLayoutRatio, setSplitRatio, ui } from '../lib/ui.svelte.js'
  import { compileTarget } from '../lib/workspace.svelte.js'
  import BottomPanel from './BottomPanel.svelte'
  import Editor from './Editor.svelte'
  import Preview from './Preview.svelte'
  import Splitter from './Splitter.svelte'
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
  /** @type {HTMLDivElement} */
  let workspaceEl
  /** @type {HTMLElement} */
  let editorPaneEl

  // Two panes with the dragged proportion and a 4px divider between them.
  let columns = $derived(
    ui.splitEditor ? `minmax(0, ${ui.splitRatio}fr) 4px minmax(0, ${1 - ui.splitRatio}fr)` : 'minmax(0, 1fr)'
  )

  // Editors | divider | preview (or the other way round), each share
  // dragged on the divider. The preview stays mounted when hidden.
  /** @type {'editorRatio' | 'editorRatioSplit'} */
  let ratioName = $derived(ui.splitEditor ? 'editorRatioSplit' : 'editorRatio')
  let editorShare = $derived(ui[ratioName])
  let outerColumns = $derived.by(() => {
    if (!ui.previewVisible) return 'minmax(0, 1fr)'
    const editors = `minmax(0, ${editorShare}fr)`
    const preview = `minmax(0, ${1 - editorShare}fr)`
    return ui.previewSide === 'right' ? `${editors} 4px ${preview}` : `${preview} 4px ${editors}`
  })

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
<div
  class="workspace"
  class:preview-left={ui.previewSide === 'left'}
  bind:this={workspaceEl}
  style:grid-template-columns={outerColumns}
>
  <section
    class="pane editor-pane"
    aria-label="Editor"
    bind:this={editorPaneEl}
    style:--bottom-h="{ui.bottomRatio * 100}%"
  >
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
      <div class="editor-host" data-pane="primary">
        <Editor pane="primary" />
      </div>
      {#if ui.splitEditor}
        <Splitter
          orientation="vertical"
          ratio={ui.splitRatio}
          label="Ancho de los editores"
          container={() => editorsEl}
          onchange={(r, persist) => setSplitRatio(r, persist)}
          onreset={() => setSplitRatio(0.5, true)}
        />
        <div class="editor-host secondary" aria-label="Segundo editor" data-pane="secondary">
          <Editor pane="secondary" />
        </div>
      {/if}
      <!-- Where the dropped file will open: the right half (second pane). -->
      <div class="drop-hint" class:visible={dropActive} aria-hidden="true">
        <span>Soltar para abrir al lado</span>
      </div>
    </div>
    {#if ui.bottomPanelOpen}
      <Splitter
        orientation="horizontal"
        ratio={ui.bottomRatio}
        invert
        label="Alto del panel inferior"
        container={() => editorPaneEl}
        onchange={(r, persist) => setLayoutRatio('bottomRatio', r, persist)}
        onreset={() => resetLayoutRatio('bottomRatio')}
      />
    {/if}
    <!-- "Errores LaTeX" | "Terminal". Always mounted; hides itself. -->
    <BottomPanel />
  </section>

  {#if ui.previewVisible}
    <div class="outer-splitter">
      <Splitter
        orientation="vertical"
        ratio={editorShare}
        invert={ui.previewSide === 'left'}
        label="Ancho de la vista previa"
        container={() => workspaceEl}
        onchange={(r, persist) => setLayoutRatio(ratioName, r, persist)}
        onreset={() => resetLayoutRatio(ratioName)}
      />
    </div>
  {/if}

  <!-- Hidden, not unmounted: the PDF, its page and the worker stay. -->
  <section class="pane preview-pane" aria-label="Vista previa" hidden={!ui.previewVisible}>
    <Preview
      compileState={compilation.status}
      pdfVersion={compilation.pdfVersion}
      documentKey={compileTarget()}
    />
  </section>
</div>

<style>
  /* Columns come from the dragged ratios (inline style). */
  .workspace {
    height: 100%;
    min-width: 0;
    display: grid;
  }

  /* Preview on the left: same DOM (nothing remounts), another order. */
  .preview-left .preview-pane {
    order: 1;
  }

  .preview-left .outer-splitter {
    order: 2;
  }

  .preview-left .editor-pane {
    order: 3;
  }

  .outer-splitter {
    display: flex;
    min-height: 0;
  }

  .preview-pane[hidden] {
    display: none;
  }

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .editor-pane {
    display: flex;
    flex-direction: column;
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
