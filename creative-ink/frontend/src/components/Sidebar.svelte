<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { newFile, openFolder, refreshWorkspace } from '../lib/actions.js'
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
        <button onclick={newFile} aria-label="Nuevo archivo" title="Nuevo archivo… ({MOD}N)" disabled={!tree}>
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
  .sidebar {
    min-width: 0;
    overflow: hidden;
    border-right: 1px solid #2b2b2b;
    background: #1b1b1b;
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
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #9d9d9d;
  }

  .tools {
    display: flex;
    gap: 0.1rem;
  }

  .tools button {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: #8a8a8a;
    cursor: pointer;
  }

  .tools button:hover:not(:disabled) {
    color: #e5e5e5;
    background: #2a2a2a;
  }

  .tools button:disabled {
    opacity: 0.4;
    cursor: default;
  }

  svg {
    width: 16px;
    height: 16px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .folder {
    margin: 0;
    padding: 0.2rem 0.9rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: #bdbdbd;
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
    font-size: 0.8rem;
    color: #9d9d9d;
  }

  .empty p {
    margin: 0 0 0.75rem;
  }

  .primary {
    width: 100%;
    padding: 0.4rem 0.75rem;
    border: 0;
    border-radius: 4px;
    background: var(--accent);
    color: #fff;
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .hint,
  .error {
    margin: 0;
    padding: 0.6rem 0.9rem;
    font-size: 0.7rem;
    line-height: 1.4;
    color: #6b6b6b;
  }

  .error {
    color: #f87171;
  }
</style>
