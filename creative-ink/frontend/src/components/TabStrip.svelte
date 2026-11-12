<script>
  // The tabs of an editor pane (docs/MAP.md §4.4, #9, C1): the file's name
  // on top of each pane. In "Multiventana", one tab per open file; in
  // "Minimalista", only the shown file (a header, no tabs to manage).
  import { closeTab, closeUntitled, openFile } from '../lib/actions.js'
  import { tabs } from '../lib/tabs.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  /** @type {{ pane: import('../lib/ui.svelte.js').EditorPane, single?: boolean }} */
  let { pane, single = false } = $props()

  let active = $derived(pane === 'primary' ? workspace.currentFilePath : workspace.secondaryFilePath)
  let untitled = $derived(pane === 'primary' && workspace.untitled)
  let list = $derived(single ? (active ? [active] : []) : tabs[pane])
  let focused = $derived(ui.splitEditor && ui.focusedPane === pane)

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
    ui.focusedPane = pane
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

{#if list.length || untitled}
  <div class="tabs" class:single class:focused role="tablist" aria-label={pane === 'primary' ? 'Archivos abiertos' : 'Archivos del segundo panel'}>
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
