<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { openFile, setMainFile } from '../lib/actions.js'
  import { setDirExpanded, ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'

  /** @type {{ node: import('../lib/workspace.svelte.js').FileNode, depth: number }} */
  let { node, depth } = $props()

  // Folders start collapsed; the user's choice is kept in ui.expandedDirs.
  let expanded = $derived(ui.expandedDirs[node.path] ?? false)
  let active = $derived(!node.isDir && node.path === workspace.currentFilePath)
  let isMain = $derived(!node.isDir && node.path === workspace.mainFilePath)
  let disabled = $derived(!node.isDir && !node.editable)

  function onclick() {
    if (node.isDir) setDirExpanded(node.path, !expanded)
    else if (node.editable) openFile(node.path)
  }

  function toggleMain() {
    void setMainFile(isMain ? '' : node.path)
  }
</script>

<li
  role="treeitem"
  aria-expanded={node.isDir ? expanded : undefined}
  aria-selected={active}
  aria-disabled={disabled}
>
  <!-- Two sibling buttons (a button cannot contain another one): open the
       file, and, on hover/focus, set or unset it as the project's main file. -->
  <div class="row" class:active class:disabled style:padding-left="{0.5 + depth * 0.75}rem">
    <button
      class="entry"
      {onclick}
      title={disabled ? `${node.name} (solo se pueden abrir archivos .tex)` : node.path}
      tabindex={disabled ? -1 : 0}
    >
      {#if node.isDir}
        <span class="chevron" class:open={expanded} aria-hidden="true">›</span>
      {:else}
        <span class="badge" class:tex={node.editable} aria-hidden="true">
          {node.editable ? 'TeX' : '·'}
        </span>
      {/if}
      <span class="name">{node.name}</span>
      {#if isMain}
        <span class="main-badge" title="Archivo principal: es el que se compila">★ MAIN</span>
      {/if}
    </button>

    {#if node.editable}
      <button
        class="main-toggle"
        class:is-main={isMain}
        onclick={toggleMain}
        aria-label={isMain ? `Quitar ${node.name} como archivo principal` : `Establecer ${node.name} como principal`}
        title={isMain ? 'Quitar como principal' : 'Establecer como principal'}
      >
        {isMain ? '★' : '☆'}
      </button>
    {/if}
  </div>

  {#if node.isDir && expanded && node.children?.length}
    <ul role="group">
      {#each node.children as child (child.path)}
        <FileTreeItem node={child} depth={depth + 1} />
      {/each}
    </ul>
  {/if}
</li>

<style>
  li {
    list-style: none;
  }

  ul {
    margin: 0;
    padding: 0;
  }

  .row {
    display: flex;
    align-items: center;
    padding-right: 0.25rem;
  }

  .row:hover:not(.disabled) {
    background: #252526;
  }

  .row.active {
    background: #37373d;
  }

  .entry {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.2rem 0.25rem 0.2rem 0;
    border: 0;
    background: transparent;
    color: #cccccc;
    font: inherit;
    font-size: 0.8rem;
    text-align: left;
    cursor: pointer;
  }

  .row.active .entry {
    color: #ffffff;
  }

  .row.disabled .entry {
    color: #6b6b6b;
    cursor: default;
  }

  .entry:focus-visible,
  .main-toggle:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }

  .main-badge {
    flex-shrink: 0;
    margin-left: auto;
    padding: 0 0.3rem;
    border-radius: 3px;
    background: rgba(251, 191, 36, 0.15);
    color: #fbbf24;
    font-size: 0.55rem;
    font-weight: 700;
    letter-spacing: 0.04em;
  }

  /* Hidden until the row is hovered or the toggle is focused (keyboard).
     The main file is always marked by the ★ MAIN badge. */
  .main-toggle {
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    padding: 0;
    border: 0;
    border-radius: 3px;
    background: transparent;
    color: #8a8a8a;
    font-size: 0.8rem;
    line-height: 1;
    cursor: pointer;
    opacity: 0;
  }

  .row:hover .main-toggle,
  .main-toggle:focus-visible {
    opacity: 1;
  }

  .main-toggle:hover {
    color: #fbbf24;
    background: #2f2f2f;
  }

  .main-toggle.is-main {
    color: #fbbf24;
  }

  .chevron {
    width: 0.8rem;
    display: inline-block;
    color: #8a8a8a;
    transition: transform 100ms ease-out;
  }

  .chevron.open {
    transform: rotate(90deg);
  }

  .badge {
    width: 0.8rem;
    font-size: 0.55rem;
    font-weight: 700;
    text-align: center;
    color: #6b6b6b;
  }

  .badge.tex {
    width: auto;
    color: #4ade80;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
