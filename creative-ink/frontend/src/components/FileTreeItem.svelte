<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { openFile, setMainFile } from '../lib/actions.js'
  import { setDirExpanded, ui } from '../lib/ui.svelte.js'
  import { isTeXPath, workspace } from '../lib/workspace.svelte.js'

  /** @type {{ node: import('../lib/workspace.svelte.js').FileNode, depth: number }} */
  let { node, depth } = $props()

  // Folders start collapsed; the user's choice is kept in ui.expandedDirs.
  let expanded = $derived(ui.expandedDirs[node.path] ?? false)
  // Shown in one of the editor panes (main or the split one).
  let active = $derived(
    !node.isDir && (node.path === workspace.currentFilePath || node.path === workspace.secondaryFilePath)
  )
  let isTeX = $derived(!node.isDir && isTeXPath(node.name))
  // Short type label: TeX, BIB, STY… ("·" for files the editor cannot open).
  let kind = $derived(isTeX ? 'TeX' : node.editable ? (/\.([^.]+)$/.exec(node.name)?.[1] ?? 'txt').slice(0, 3).toUpperCase() : '·')
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
      title={disabled ? `${node.name} (no es un archivo de texto: no se puede editar)` : node.path}
      tabindex={disabled ? -1 : 0}
    >
      {#if node.isDir}
        <span class="chevron" class:open={expanded} aria-hidden="true">›</span>
      {:else}
        <span class="badge" class:tex={isTeX} class:text={node.editable && !isTeX} aria-hidden="true">
          {kind}
        </span>
      {/if}
      <span class="name">{node.name}</span>
      {#if isMain}
        <span class="main-badge" title="Archivo principal: es el que se compila">★ MAIN</span>
      {/if}
    </button>

    {#if isTeX}
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
    min-height: 24px;
    margin: 0 0.35rem;
    padding-right: 0.25rem;
    border-radius: var(--radius-sm);
    transition: background-color var(--transition), box-shadow var(--transition);
  }

  .row:hover:not(.disabled) {
    background: var(--surface-hover);
  }

  /* Active item (A2, accent rule 2): soft accent fill, primary text and a
     2px accent mark on the left, drawn inset so the row does not shift. */
  .row.active,
  .row.active:hover {
    background: var(--accent-soft);
    box-shadow: inset 2px 0 0 var(--accent);
  }

  .entry {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.2rem 0.25rem 0.2rem 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    transition: color var(--transition);
  }

  .row:hover:not(.disabled) .entry,
  .row.active .entry {
    color: var(--text-primary);
  }

  .row.disabled .entry {
    color: var(--text-disabled);
    cursor: default;
  }

  /* Inset: the tree scrolls inside the drawer, an outer ring would be clipped. */
  .entry:focus-visible,
  .main-toggle:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  /* Neutral marker: the main file is information, not an action (A3 badges). */
  .main-badge {
    flex-shrink: 0;
    margin-left: auto;
    padding: 0 0.3rem;
    border-radius: var(--radius-sm);
    background: var(--surface-active);
    color: var(--text-secondary);
    font-size: var(--text-xs);
    font-weight: 600;
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
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-muted);
    font-size: var(--text-sm);
    line-height: 1;
    cursor: pointer;
    opacity: 0;
    transition: opacity var(--transition), color var(--transition), background-color var(--transition);
  }

  .row:hover .main-toggle,
  .main-toggle:focus-visible {
    opacity: 1;
  }

  .main-toggle:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .main-toggle.is-main {
    color: var(--text-primary);
  }

  .chevron {
    width: 0.8rem;
    display: inline-block;
    color: var(--text-muted);
    transition: transform var(--transition);
  }

  .chevron.open {
    transform: rotate(90deg);
  }

  /* File-type badges: muted, never the accent (A3). One width for all, so
     the names line up. */
  .badge {
    flex-shrink: 0;
    width: 1.75rem;
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--text-disabled);
  }

  .badge.tex {
    color: var(--text-secondary);
  }

  .badge.text {
    color: var(--text-muted);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
