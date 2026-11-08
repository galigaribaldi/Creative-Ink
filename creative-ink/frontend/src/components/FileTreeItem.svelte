<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { openFile } from '../lib/actions.js'
  import { setDirExpanded, ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'

  /** @type {{ node: import('../lib/workspace.svelte.js').FileNode, depth: number }} */
  let { node, depth } = $props()

  // Folders start collapsed; the user's choice is kept in ui.expandedDirs.
  let expanded = $derived(ui.expandedDirs[node.path] ?? false)
  let active = $derived(!node.isDir && node.path === workspace.currentFilePath)
  let disabled = $derived(!node.isDir && !node.editable)

  function onclick() {
    if (node.isDir) setDirExpanded(node.path, !expanded)
    else if (node.editable) openFile(node.path)
  }
</script>

<li
  role="treeitem"
  aria-expanded={node.isDir ? expanded : undefined}
  aria-selected={active}
  aria-disabled={disabled}
>
  <button
    class="row"
    class:active
    class:disabled
    style:padding-left="{0.5 + depth * 0.75}rem"
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
  </button>

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
    gap: 0.4rem;
    width: 100%;
    padding: 0.2rem 0.75rem 0.2rem 0.5rem;
    border: 0;
    background: transparent;
    color: #cccccc;
    font: inherit;
    font-size: 0.8rem;
    text-align: left;
    cursor: pointer;
  }

  .row:hover:not(.disabled) {
    background: #252526;
  }

  .row:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }

  .row.active {
    background: #37373d;
    color: #ffffff;
  }

  .row.disabled {
    color: #6b6b6b;
    cursor: default;
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
