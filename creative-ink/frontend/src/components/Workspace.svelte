<script>
  // The dock (docs/MAP.md §4.4, #9): up to 4 editor panes and the PDF
  // preview, each drawn ONCE and placed in the rectangle the layout engine
  // gives it (lib/layout.js). Rearranging only changes rectangles: CodeMirror
  // and PDF.js are never remounted. The three modes are a safety over the
  // same items: Multiventana shows the global arrangement, Minimalista the
  // focused pane next to the preview, Solo PDF only the preview. The bottom
  // panel (Errores LaTeX | Terminal CI | Shell) spans the width below.
  import { FILE_DRAG_TYPE, focusPane, openInPane } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { dock } from '../lib/dock.svelte.js'
  import { computeLayout, nextVisible, resize } from '../lib/layout.js'
  import { openPanes, panes, setLayout } from '../lib/panes.svelte.js'
  import { resetLayoutRatio, setLayoutRatio, ui } from '../lib/ui.svelte.js'
  import { compileTarget } from '../lib/workspace.svelte.js'
  import BottomPanel from './BottomPanel.svelte'
  import Editor from './Editor.svelte'
  import Preview from './Preview.svelte'
  import Splitter from './Splitter.svelte'
  import TabStrip from './TabStrip.svelte'
  import TrustBanner from './TrustBanner.svelte'

  /** Smallest pane along a dragged divider (px). */
  const MIN_ITEM_PX = 160

  /** @type {HTMLDivElement} */
  let workspaceEl
  /** @type {HTMLDivElement} */
  let dockEl

  let pdfOnly = $derived(ui.editorMode === 'pdf')

  /** @typedef {import('../lib/layout.js').LayoutNode} LayoutNode */

  // What is on screen, per mode.
  let view = $derived.by(() => {
    if (ui.editorMode === 'pdf') {
      return { kind: 'pdf', ...computeLayout({ kind: 'leaf', item: 'pdf' }) }
    }
    if (ui.editorMode === 'minimal') {
      const pane = { kind: 'leaf', item: panes.focused }
      /** @type {LayoutNode} */
      const tree = ui.previewVisible
        ? {
            kind: 'split',
            dir: 'row',
            sizes: ui.previewSide === 'right' ? [ui.editorRatio, 1 - ui.editorRatio] : [1 - ui.editorRatio, ui.editorRatio],
            children: ui.previewSide === 'right' ? [pane, { kind: 'leaf', item: 'pdf' }] : [{ kind: 'leaf', item: 'pdf' }, pane]
          }
        : /** @type {LayoutNode} */ (pane)
      return { kind: 'minimal', ...computeLayout(tree) }
    }
    const hidden = ui.previewVisible ? new Set() : new Set(['pdf'])
    return { kind: 'multi', hidden, ...computeLayout(panes.layout, /** @type {Set<any>} */ (hidden)) }
  })

  // The dragging code (dock.svelte.js) hit-tests these rectangles.
  $effect(() => {
    dock.rects = view.rects
    dock.el = dockEl
  })

  /** Where the dragged item would go: half of the target, or all of it (swap). */
  let dropHint = $derived.by(() => {
    const r = dock.target ? view.rects[dock.target] : undefined
    if (!r || !dock.zone) return ''
    const half = {
      left: { ...r, w: r.w / 2 },
      right: { ...r, x: r.x + r.w / 2, w: r.w / 2 },
      top: { ...r, h: r.h / 2 },
      bottom: { ...r, y: r.y + r.h / 2, h: r.h / 2 },
      center: r
    }[dock.zone]
    return place(half)
  })

  /** CSS of an item's rectangle (fractions of the dock). @param {import('../lib/layout.js').Rect | undefined} r */
  function place(r) {
    return r ? `left:${r.x * 100}%;top:${r.y * 100}%;width:${r.w * 100}%;height:${r.h * 100}%` : ''
  }

  /** CSS of a divider: 4px centred on the boundary. @param {import('../lib/layout.js').Handle} h */
  function handleStyle(h) {
    const { pair } = h
    return h.dir === 'row'
      ? `left:calc(${h.at * 100}% - 2px);top:${pair.y * 100}%;height:${pair.h * 100}%;width:4px`
      : `top:calc(${h.at * 100}% - 2px);left:${pair.x * 100}%;width:${pair.w * 100}%;height:4px`
  }

  /** The two items a divider separates, as a box the Splitter can measure. @param {import('../lib/layout.js').Handle} h */
  function pairBox(h) {
    return /** @type {HTMLElement} */ (
      /** @type {unknown} */ ({
        getBoundingClientRect() {
          const d = dockEl.getBoundingClientRect()
          return {
            left: d.left + h.pair.x * d.width,
            top: d.top + h.pair.y * d.height,
            width: h.pair.w * d.width,
            height: h.pair.h * d.height
          }
        }
      })
    )
  }

  /** Current share of the first item of the pair. @param {import('../lib/layout.js').Handle} h */
  function pairRatio(h) {
    return h.dir === 'row' ? (h.at - h.pair.x) / h.pair.w : (h.at - h.pair.y) / h.pair.h
  }

  /** Keeps both items at least MIN_ITEM_PX. @param {import('../lib/layout.js').Handle} h @param {number} r */
  function clampRatio(h, r) {
    const d = dockEl.getBoundingClientRect()
    const size = h.dir === 'row' ? h.pair.w * d.width : h.pair.h * d.height
    const min = Math.min(0.45, MIN_ITEM_PX / Math.max(size, 1))
    return Math.min(1 - min, Math.max(min, r))
  }

  /** A divider moved. @param {import('../lib/layout.js').Handle} h @param {number} r @param {boolean} persist */
  function onresize(h, r, persist) {
    r = clampRatio(h, r)
    if (view.kind === 'minimal') {
      setLayoutRatio('editorRatio', ui.previewSide === 'right' ? r : 1 - r, persist)
      return
    }
    const hidden = /** @type {Set<any>} */ (view.kind === 'multi' ? view.hidden : new Set())
    const other = nextVisible(panes.layout, h.path, h.index, hidden)
    if (other >= 0) setLayout(resize(panes.layout, h.path, h.index, other, r), persist)
  }

  /** Double click on a divider: share the two items equally. @param {import('../lib/layout.js').Handle} h */
  function onreset(h) {
    if (view.kind === 'minimal') return resetLayoutRatio('editorRatio')
    onresize(h, 0.5, true)
  }

  // ── Files dragged from the explorer: they open in the pane under the
  // pointer. Capture-phase handlers so CodeMirror never sees these drags (it
  // would insert the path as text); other drags pass through untouched.
  /** @type {string} */
  let dropTarget = $state('')

  /** @param {DragEvent} event */
  function isFileDrag(event) {
    return event.dataTransfer?.types.includes(FILE_DRAG_TYPE) ?? false
  }

  /** @param {DragEvent} event */
  function paneUnder(event) {
    const el = /** @type {HTMLElement | null} */ (/** @type {HTMLElement} */ (event.target).closest?.('[data-pane]'))
    return el?.dataset.pane ?? ''
  }

  /** @param {DragEvent} event */
  function ondragovercapture(event) {
    if (!isFileDrag(event)) return
    event.preventDefault() // allows the drop
    event.stopPropagation()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
    dropTarget = paneUnder(event) || panes.focused
  }

  /** @param {DragEvent} event */
  function ondragleavecapture(event) {
    if (!isFileDrag(event)) return
    event.stopPropagation()
    if (!dockEl.contains(/** @type {Node | null} */ (event.relatedTarget))) dropTarget = ''
  }

  /** @param {DragEvent} event */
  function ondropcapture(event) {
    if (!isFileDrag(event)) return
    event.preventDefault()
    event.stopPropagation()
    const pane = /** @type {import('../lib/ui.svelte.js').EditorPane} */ (paneUnder(event) || panes.focused)
    dropTarget = ''
    const path = event.dataTransfer?.getData(FILE_DRAG_TYPE) || event.dataTransfer?.getData('text/plain') || ''
    openInPane(path, pane)
  }
</script>

<svelte:window ondragend={() => (dropTarget = '')} />

<div class="workspace" bind:this={workspaceEl} style:--bottom-h="{ui.bottomRatio * 100}%">
  <!-- Untrusted .latexmkrc: compiling with -norc; offers to trust it. -->
  <TrustBanner />

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="dock" bind:this={dockEl} {ondragovercapture} {ondragleavecapture} {ondropcapture}>
    {#each openPanes() as id (id)}
      {@const rect = view.rects[id]}
      <section
        class="item editor-item"
        class:off={!rect}
        class:drop-target={dropTarget === id}
        data-pane={id}
        style={place(rect)}
        aria-label="Editor"
        onpointerdowncapture={() => panes.focused !== id && focusPane(id, false)}
      >
        <!-- The file's name on top: tabs (Multiventana) or one header. -->
        <TabStrip pane={id} single={ui.editorMode !== 'multi'} />
        <Editor pane={id} />
      </section>
    {/each}

    <!-- Hidden, not unmounted: the PDF, its page and the worker stay. -->
    <section class="item preview-item" class:off={!view.rects.pdf} style={place(view.rects.pdf)} aria-label="Vista previa">
      <Preview compileState={compilation.status} pdfVersion={compilation.pdfVersion} documentKey={compileTarget()} />
    </section>

    {#if dropHint}
      <div class="dock-hint" class:swap={dock.zone === 'center'} style={dropHint} aria-hidden="true">
        <span>{dock.zone === 'center' ? 'Intercambiar' : 'Colocar aquí'}</span>
      </div>
    {/if}

    {#each view.handles as h (h.key)}
      <div class="handle {h.dir}" style={handleStyle(h)}>
        <Splitter
          orientation={h.dir === 'row' ? 'vertical' : 'horizontal'}
          ratio={pairRatio(h)}
          label={h.dir === 'row' ? 'Ancho de los paneles' : 'Alto de los paneles'}
          container={() => pairBox(h)}
          onchange={(r, persist) => onresize(h, r, persist)}
          onreset={() => onreset(h)}
        />
      </div>
    {/each}
  </div>

  <!-- "Errores LaTeX" | "Terminal CI" | "Shell": always mounted, hidden in Solo PDF. -->
  <div class="bottom" class:open={ui.bottomPanelOpen} class:off={pdfOnly}>
    {#if ui.bottomPanelOpen}
      <Splitter
        orientation="horizontal"
        ratio={ui.bottomRatio}
        invert
        label="Alto del panel inferior"
        container={() => workspaceEl}
        onchange={(r, persist) => setLayoutRatio('bottomRatio', r, persist)}
        onreset={() => resetLayoutRatio('bottomRatio')}
      />
    {/if}
    <BottomPanel />
  </div>
</div>

<style>
  .workspace {
    height: 100%;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .dock {
    position: relative;
    flex: 1;
    min-height: 0;
  }

  /* Every item fills the rectangle the layout engine gives it. */
  .item {
    position: absolute;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }

  .item.off {
    display: none;
  }

  /* Tabs (auto height) above the editor (the rest). */
  .editor-item {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
  }

  /* Always the second row, even when there is no tab to show above. */
  .editor-item > :global(.editor) {
    grid-row: 2;
  }

  /* Where an explorer file will open: the pane under the pointer. */
  .editor-item.drop-target::after {
    content: '';
    position: absolute;
    inset: 4px;
    border: 1px dashed var(--border-strong);
    border-radius: var(--radius-md);
    background: var(--surface-active);
    opacity: 0.6;
    pointer-events: none;
  }

  /* Docking: where the dragged pane or preview will land (A3: soft surface,
     dashed border; never the accent). */
  .dock-hint {
    position: absolute;
    z-index: 4;
    display: grid;
    place-items: center;
    margin: 4px;
    box-sizing: border-box;
    border: 2px dashed var(--border-strong);
    border-radius: var(--radius-md);
    background: var(--surface-active);
    color: var(--text-secondary);
    font-size: var(--text-sm);
    opacity: 0.85;
    pointer-events: none;
    transition: left var(--transition), top var(--transition), width var(--transition), height var(--transition);
  }

  .handle {
    position: absolute;
    z-index: 3;
    display: flex;
  }

  .handle.col {
    flex-direction: column;
  }

  .handle.col :global(.splitter) {
    width: 100%;
  }

  /* Below the dock, the whole width (like VS Code's panel); its height is
     dragged on the divider above it. Closed, it takes no space. */
  .bottom {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .bottom.open {
    flex: 0 0 var(--bottom-h, 35%);
  }

  .bottom.off {
    display: none;
  }
</style>
