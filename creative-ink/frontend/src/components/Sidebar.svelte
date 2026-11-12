<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { newNamedFile, openFolder, refreshWorkspace } from '../lib/actions.js'
  import { ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'

  const TITLES = { explorer: 'Explorador' }
  const MOD = navigator.platform.toUpperCase().includes('MAC') ? '⌘' : 'Ctrl+'

  let tree = $derived(workspace.fileTree)
</script>

<!-- Always mounted; hidden by the shell's zero-width column. `inert` keeps
     the collapsed drawer out of the tab order and the accessibility tree. -->
<aside class="sidebar" aria-label={TITLES[ui.activeTool]} inert={!ui.sidebarOpen}>
  <div class="content">
    <header>
      <span>{TITLES[ui.activeTool]}</span>
      <div class="tools">
        <button onclick={newNamedFile} aria-label="Nuevo archivo con nombre" title="Nuevo archivo con nombre… ({MOD}N: uno sin título)" disabled={!tree}>
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
            <path d="M14 3v5h5M12 11v6M9 14h6" />
          </svg>
        </button>
        <button onclick={openFolder} aria-label="Abrir carpeta" title="Abrir carpeta">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
          </svg>
        </button>
        <button onclick={refreshWorkspace} aria-label="Actualizar" title="Actualizar" disabled={!tree}>
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7" />
          </svg>
        </button>
      </div>
    </header>

    {#if ui.activeTool === 'explorer'}
      {#if tree}
        <p class="folder" title={tree.path}>{tree.name}</p>
        {#if tree.children?.length}
          <ul class="tree" role="tree" aria-label="Archivos de {tree.name}">
            {#each tree.children as node (node.path)}
              <FileTreeItem {node} depth={0} />
            {/each}
          </ul>
        {:else}
          <p class="hint">La carpeta está vacía.</p>
        {/if}
        {#if workspace.truncated}
          <p class="hint">La carpeta es muy grande: se muestra solo una parte.</p>
        {/if}
      {:else}
        <div class="empty">
          <p>No hay ninguna carpeta abierta.</p>
          <button class="primary" onclick={openFolder}>Abrir carpeta</button>
        </div>
      {/if}

      {#if workspace.error}
        <p class="error" role="alert">{workspace.error}</p>
      {/if}
    {/if}
  </div>
</aside>

<style>
  /* Chrome: --surface-base like the ActionBar; the editor's own surface
     separates it from the Workspace, so no border is needed (A3). */
  .sidebar {
    min-width: 0;
    overflow: hidden;
    background: var(--surface-base);
  }

  /* Fixed inner width: the drawer clips its content while it animates
     instead of reflowing the text on every frame. */
  .content {
    width: 240px;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.5rem 0.4rem 0.9rem;
    font-size: var(--text-xs);
    font-weight: 500;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  .tools {
    display: flex;
    gap: 0.1rem;
  }

  .tools button {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  .tools button:hover:not(:disabled) {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .tools button:active:not(:disabled) {
    background: var(--surface-active);
  }

  .tools button:focus-visible,
  .primary:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .tools button:disabled {
    color: var(--text-disabled);
    cursor: default;
  }

  svg {
    width: var(--icon-size);
    height: var(--icon-size);
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .folder {
    margin: 0;
    padding: 0.2rem 0.9rem;
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tree {
    margin: 0;
    padding: 0 0 0.5rem;
  }

  .empty {
    padding: 1rem 0.9rem;
    font-size: var(--text-md);
    color: var(--text-secondary);
  }

  .empty p {
    margin: 0 0 0.75rem;
  }

  /* The one primary action of the empty state: filled with the accent (A2). */
  .primary {
    width: 100%;
    padding: 0.4rem 0.75rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: var(--accent);
    color: var(--accent-contrast);
    font: inherit;
    font-size: var(--text-md);
    font-weight: 500;
    cursor: pointer;
    transition: background-color var(--transition);
  }

  .primary:hover {
    background: var(--accent-hover);
  }

  .hint,
  .error {
    margin: 0;
    padding: 0.6rem 0.9rem;
    font-size: var(--text-xs);
    line-height: 1.4;
    color: var(--text-muted);
  }

  .error {
    color: var(--danger);
  }
</style>
