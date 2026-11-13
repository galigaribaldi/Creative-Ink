<script>
  // The tabs of an editor pane (docs/MAP.md §4.4, #9, C1): the file's name
  // on top of each pane. In "Multiventana", one tab per open file; in
  // "Minimalista", only the shown file (a header, no tabs to manage).
  import { closePane, closeTab, closeUntitled, focusPane, openFile } from '../lib/actions.js'
  import { startDockDrag } from '../lib/dock.svelte.js'
  import { openPanes, panes } from '../lib/panes.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  /** @type {{ pane: import('../lib/ui.svelte.js').EditorPane, single?: boolean }} */
  let { pane, single = false } = $props()

  let active = $derived(panes.files[pane])
  let untitled = $derived(panes.untitled === pane)
  let list = $derived(single ? (active ? [active] : []) : panes.tabs[pane])
  let several = $derived(openPanes().length > 1)
  let focused = $derived(several && panes.focused === pane)

  /** Path from the open folder, for the tooltip. */
  function relative(/** @type {string} */ path) {
    const root = workspace.currentFolderPath
    return root && path.startsWith(root) ? path.slice(root.length + 1) : path
  }

  /** Two files with the same name: show their folder too. */
  function label(/** @type {string} */ path) {
    const name = baseName(path)
    const twins = list.filter((p) => baseName(p) === name).length > 1
    if (!twins) return name
    const rel = relative(path).split(/[\\/]/)
    return rel.length > 1 ? `${name} · ${rel.at(-2)}` : name
  }

  function show(/** @type {string} */ path) {
    focusPane(pane, false)
    openFile(path)
  }

  /** Middle click closes, like in browsers and VS Code. @param {MouseEvent} event @param {string} path */
  function onauxclick(event, path) {
    if (event.button === 1) {
      event.preventDefault()
      void closeTab(pane, path)
    }
  }
</script>

{#if list.length || untitled || !single}
  <!-- A pointer anywhere in the pane focuses it (Workspace.svelte). -->
  <div class="tabs" class:single class:focused role="tablist" aria-label="Archivos abiertos en el panel">
    {#each list as path (path)}
      {@const isActive = path === active && !untitled}
      <div class="tab" class:active={isActive} title={relative(path)}>
        <button
          class="name"
          role="tab"
          aria-selected={isActive}
          onclick={() => show(path)}
          onauxclick={(e) => onauxclick(e, path)}
        >{label(path)}</button>
        {#if !single}
          <button class="close" onclick={() => closeTab(pane, path)} aria-label="Cerrar {baseName(path)}" title="Cerrar (clic central)">×</button>
        {/if}
      </div>
    {/each}
    {#if untitled}
      <div class="tab active untitled" title="Documento sin título: ⌘S para guardarlo">
        <button class="name" role="tab" aria-selected="true">Sin título</button>
        <button class="close" onclick={closeUntitled} aria-label="Cerrar el documento sin título" title="Cerrar">×</button>
      </div>
    {/if}
    {#if !single && ui.editorMode === 'multi'}
      <!-- The pane itself (Multiventana): drag it anywhere, or close it. -->
      <span class="pane-actions">
        <button
          class="grip"
          onpointerdown={(e) => startDockDrag(pane, e)}
          aria-label="Mover el panel"
          title="Arrastra para colocar el panel donde quieras (Esc cancela)"
        >
          <!-- Lucide "grip-vertical" -->
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="9" cy="12" r="1" /><circle cx="9" cy="5" r="1" /><circle cx="9" cy="19" r="1" />
            <circle cx="15" cy="12" r="1" /><circle cx="15" cy="5" r="1" /><circle cx="15" cy="19" r="1" />
          </svg>
        </button>
        {#if several}
          <button class="close-pane" onclick={(e) => (e.stopPropagation(), closePane(pane))} aria-label="Cerrar el panel" title="Cerrar el panel">
            <!-- Lucide "x" -->
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18" /><path d="m6 6 12 12" /></svg>
          </button>
        {/if}
      </span>
    {/if}
  </div>
{/if}

<style>
  /* A thin strip on the chrome surface (A3); scrolls sideways when full. */
  .tabs {
    display: flex;
    min-width: 0;
    overflow-x: auto;
    overflow-y: hidden;
    border-bottom: 1px solid var(--border);
    background: var(--surface-base);
    scrollbar-width: none;
  }

  .tabs::-webkit-scrollbar {
    display: none;
  }

  .tab {
    position: relative;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    max-width: 14rem;
    border-right: 1px solid var(--border);
    color: var(--text-muted);
    transition: background-color var(--transition), color var(--transition);
  }

  .tab:hover {
    color: var(--text-secondary);
    background: var(--surface-hover);
  }

  /* The shown file sits on the editor's surface, like a sheet in front. */
  .tab.active {
    background: var(--surface-editor);
    color: var(--text-primary);
  }

  /* Focused pane (split): the accent line on its shown tab (accent rule 2). */
  .focused .tab.active::before {
    content: '';
    position: absolute;
    inset: 0 0 auto 0;
    height: 2px;
    background: var(--accent);
  }

  .pane-actions {
    position: sticky;
    right: 0;
    display: flex;
    align-items: center;
    margin-left: auto;
    padding: 0 0.3rem;
    background: var(--surface-base);
  }

  .grip {
    display: grid;
    place-items: center;
    width: 1.3rem;
    height: 1.4rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    cursor: grab;
    touch-action: none;
    transition: background-color var(--transition), color var(--transition);
  }

  .grip:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .grip svg {
    width: 14px;
    height: 14px;
    fill: currentColor;
    stroke: none;
  }

  .close-pane {
    display: grid;
    place-items: center;
    width: 1.4rem;
    height: 1.4rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    transition: background-color var(--transition), color var(--transition);
  }

  .close-pane:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .close-pane svg {
    width: 14px;
    height: 14px;
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
  }

  .untitled .name {
    font-style: italic;
  }

  button {
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .name {
    min-width: 0;
    padding: 0.35rem 0.25rem 0.35rem 0.75rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
  }

  .single .name {
    padding-right: 0.75rem;
    cursor: default;
  }

  .close {
    width: 1.25rem;
    height: 1.25rem;
    margin-right: 0.35rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    line-height: 1;
    opacity: 0;
    transition: opacity var(--transition), background-color var(--transition);
  }

  .tab:hover .close,
  .tab.active .close,
  .close:focus-visible {
    opacity: 1;
  }

  .close:hover {
    color: var(--text-primary);
    background: var(--surface-active);
  }

  .name:focus-visible,
  .close:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
</style>
