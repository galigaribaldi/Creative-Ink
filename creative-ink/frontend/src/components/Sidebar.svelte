<script>
  import { onMount } from 'svelte'
  import { DocumentPath } from '../../wailsjs/go/main/App.js'
  import { ui } from '../lib/ui.svelte.js'

  const TITLES = { explorer: 'Explorador' }

  let docPath = $state('')
  let fileName = $derived(docPath.split(/[\\/]/).pop() || '')
  let folder = $derived(docPath.split(/[\\/]/).slice(-2, -1)[0] || '')

  // One-off read of backend data (not UI state): which document is open.
  onMount(async () => {
    try {
      docPath = await DocumentPath()
    } catch {
      docPath = ''
    }
  })
</script>

<!-- Always mounted; hidden by the shell's zero-width column. `inert` keeps
     the collapsed drawer out of the tab order and the accessibility tree. -->
<aside class="sidebar" aria-label={TITLES[ui.activeTool]} inert={!ui.sidebarOpen}>
  <div class="content">
    <header>{TITLES[ui.activeTool]}</header>

    {#if ui.activeTool === 'explorer'}
      {#if docPath}
        <p class="folder">{folder}</p>
        <ul class="tree">
          <li class="file active" title={docPath}>
            <span class="ext">TeX</span>
            {fileName}
          </li>
        </ul>
      {/if}
      <p class="hint">Abrir carpetas y archivos llegará con el explorador completo.</p>
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
  }

  header {
    padding: 0.6rem 0.9rem;
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #9d9d9d;
  }

  .folder {
    margin: 0;
    padding: 0.2rem 0.9rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: #bdbdbd;
  }

  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .file {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.9rem 0.25rem 1.6rem;
    font-size: 0.8rem;
    color: #d4d4d4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .file.active {
    background: #2a2d2e;
  }

  .ext {
    font-size: 0.6rem;
    font-weight: 700;
    color: #4ade80;
  }

  .hint {
    margin: auto 0 0;
    padding: 0.9rem;
    font-size: 0.7rem;
    line-height: 1.4;
    color: #6b6b6b;
  }
</style>
