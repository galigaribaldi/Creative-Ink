<script>
  import FileTreeItem from './FileTreeItem.svelte'
  import { tick } from 'svelte'
  import { createInExplorer, openFolder, refreshWorkspace } from '../lib/actions.js'
  import { ui } from '../lib/ui.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  const TITLES = { explorer: 'Explorador' }

  let tree = $derived(workspace.fileTree)
  let canCreate = $derived(!!tree && !workspace.singleFile)

  // ── New file / folder: a name field above the tree (no native dialog) ──
  let newName = $state('')
  let createError = $state('')
  let creating = $state(false)
  /** @type {HTMLInputElement | undefined} */
  let nameInput = $state()
  let where = $derived(
    ui.treeCreate ? (ui.treeCreate.parent === tree?.path ? tree.name : baseName(ui.treeCreate.parent)) : ''
  )

  /** @param {'file' | 'folder'} kind @param {string} [parent] */
  async function startCreate(kind, parent = tree?.path ?? '') {
    ui.treeMenu = null
    if (!canCreate || !parent) return
    ui.treeCreate = { kind, parent }
    newName = ''
    createError = ''
    await tick()
    nameInput?.focus()
  }

  function cancelCreate() {
    ui.treeCreate = null
    createError = ''
  }

  async function submitCreate() {
    if (!ui.treeCreate || !newName.trim() || creating) return
    creating = true
    const error = await createInExplorer(ui.treeCreate.kind, ui.treeCreate.parent, newName)
    creating = false
    if (error) createError = error
    else cancelCreate()
  }

  /** @param {KeyboardEvent} event */
  function onnamekeydown(event) {
    if (event.key === 'Enter') {
      event.preventDefault()
      void submitCreate()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      cancelCreate()
    }
  }

  // The right-click menu closes on any click elsewhere or Escape.
  /** @param {KeyboardEvent} event */
  function onwindowkeydown(event) {
    if (event.key === 'Escape' && ui.treeMenu) ui.treeMenu = null
  }
</script>

<svelte:window onclick={() => (ui.treeMenu = null)} onkeydown={onwindowkeydown} />

<!-- Always mounted; hidden by the shell's zero-width column. `inert` keeps
     the collapsed drawer out of the tab order and the accessibility tree. -->
<aside class="sidebar" aria-label={TITLES[ui.activeTool]} inert={!ui.sidebarOpen}>
  <div class="content">
    <header>
      <span>{TITLES[ui.activeTool]}</span>
      <div class="tools">
        <button onclick={() => startCreate('file')} aria-label="Nuevo archivo" title="Nuevo archivo en la carpeta (clic derecho en una carpeta: dentro de ella)" disabled={!canCreate}>
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
            <path d="M14 3v5h5M12 11v6M9 14h6" />
          </svg>
        </button>
        <button onclick={() => startCreate('folder')} aria-label="Nueva carpeta" title="Nueva carpeta (clic derecho en una carpeta: dentro de ella)" disabled={!canCreate}>
          <!-- Lucide "folder-plus" -->
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 10v6M9 13h6" />
            <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
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
        {#if ui.treeCreate}
          <div class="create">
            <label>
              <span>{ui.treeCreate.kind === 'file' ? 'Nuevo archivo' : 'Nueva carpeta'} en <strong>{where}</strong></span>
              <input
                bind:this={nameInput}
                bind:value={newName}
                onkeydown={onnamekeydown}
                onblur={() => !newName.trim() && cancelCreate()}
                disabled={creating}
                placeholder={ui.treeCreate.kind === 'file' ? 'capitulo-3.tex (sin extensión: .tex)' : 'Coloquio-2027-1'}
                autocomplete="off"
                spellcheck="false"
              />
            </label>
            <small>Enter crea · Esc cancela · se normaliza con «-» y sin acentos</small>
            {#if createError}<p class="error" role="alert">{createError}</p>{/if}
          </div>
        {/if}
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

{#if ui.treeMenu}
  {@const menu = ui.treeMenu}
  <!-- Right-click menu of the explorer (FileTreeItem.svelte opens it). -->
  <div class="context-menu" role="menu" style:left="{menu.x}px" style:top="{menu.y}px">
    <button role="menuitem" onclick={(e) => (e.stopPropagation(), startCreate('file', menu.parent))}>Nuevo archivo aquí</button>
    <button role="menuitem" onclick={(e) => (e.stopPropagation(), startCreate('folder', menu.parent))}>Nueva carpeta aquí</button>
  </div>
{/if}

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

  /* Name field of a new file or folder, above the tree. */
  .create {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    margin: 0 0.5rem 0.5rem;
  }

  .create label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  .create input {
    padding: 0.25rem 0.4rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-editor);
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-sm);
  }

  .create input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }

  .create small {
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  /* Raised surface, overlay shadow, medium radius (A3). */
  .context-menu {
    position: fixed;
    z-index: 25;
    display: flex;
    flex-direction: column;
    min-width: 11rem;
    padding: 0.25rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
  }

  .context-menu button {
    padding: 0.3rem 0.6rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--transition);
  }

  .context-menu button:hover,
  .context-menu button:focus-visible {
    background: var(--surface-hover);
    outline: none;
  }
</style>
