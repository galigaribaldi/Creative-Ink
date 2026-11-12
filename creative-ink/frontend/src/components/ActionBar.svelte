<script>
  import { compileDocument, openFolder, saveAs, saveDocument, toggleSplitEditor } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { terminal } from '../lib/terminal.svelte.js'
  import { toggleBottomTab, togglePreview, toggleTool, ui } from '../lib/ui.svelte.js'

  // macOS shows ⌘, Linux shows Ctrl in the tooltips.
  const IS_MAC = navigator.platform.toUpperCase().includes('MAC')
  const MOD = IS_MAC ? '⌘' : 'Ctrl+'
  const SHIFT_MOD = IS_MAC ? '⇧⌘' : 'Ctrl+Shift+'

  let explorerActive = $derived(ui.sidebarOpen && ui.activeTool === 'explorer')
  let searchActive = $derived(ui.sidebarOpen && ui.activeTool === 'search')
  let compiling = $derived(compilation.status === 'compiling')
  let compileFailed = $derived(compilation.status === 'error')
  let terminalOpen = $derived(ui.bottomPanelOpen && ui.bottomTab === 'terminal')
</script>

<nav class="actionbar" aria-label="Barra de acciones">
  <div class="group">
    <button onclick={() => (ui.startOpen = true)} aria-label="Inicio" title="Inicio: proyectos y documentos rápidos">
      <!-- Lucide "house": the launcher (start screen) -->
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8" />
        <path d="M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      </svg>
    </button>

    <button
      class:active={explorerActive}
      onclick={() => toggleTool('explorer')}
      aria-label="Explorador"
      aria-pressed={explorerActive}
      title="Explorador ({MOD}B)"
    >
      <!-- Lucide "folder-tree": the project's files (not a document) -->
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M20 10a1 1 0 0 0 1-1V6a1 1 0 0 0-1-1h-2.5a1 1 0 0 1-.8-.4l-.9-1.2A1 1 0 0 0 15 3h-2a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1Z" />
        <path d="M20 21a1 1 0 0 0 1-1v-3a1 1 0 0 0-1-1h-2.9a1 1 0 0 1-.88-.55l-.42-.85a1 1 0 0 0-.92-.6H13a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1Z" />
        <path d="M3 5a2 2 0 0 0 2 2h3" />
        <path d="M3 3v13a2 2 0 0 0 2 2h3" />
      </svg>
    </button>

    <button
      class:active={searchActive}
      onclick={() => toggleTool('search')}
      aria-label="Buscar en el proyecto"
      aria-pressed={searchActive}
      title="Buscar en el proyecto ({SHIFT_MOD}F)"
    >
      <!-- Lucide "search" -->
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.3-4.3" />
      </svg>
    </button>

    <button onclick={openFolder} aria-label="Abrir carpeta" title="Abrir carpeta ({MOD}O)">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      </svg>
    </button>

    <button onclick={saveDocument} aria-label="Guardar" title="Guardar ({MOD}S)">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M5 3h11l3 3v13a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2z" />
        <path d="M8 3v5h7V3M8 21v-7h8v7" />
      </svg>
    </button>

    <button onclick={saveAs} aria-label="Guardar como…" title="Guardar como… ({SHIFT_MOD}S)">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M13 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h9l3 3v5" />
        <path d="M8 3v5h7V3M17 15v6M14 18h6" />
      </svg>
    </button>

    <button
      class="compile"
      class:busy={compiling}
      onclick={compileDocument}
      disabled={compiling}
      aria-label={compiling ? 'Compilando…' : 'Compilar'}
      title={compiling ? 'Compilando…' : 'Compilar'}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        {#if compiling}
          <path d="M21 12a9 9 0 1 1-6.2-8.56" />
        {:else}
          <path d="M7 4v16l13-8z" />
        {/if}
      </svg>
      {#if compileFailed}<span class="badge" aria-hidden="true"></span>{/if}
    </button>
  </div>

  <div class="group">
    <button
      class:active={ui.splitEditor}
      onclick={toggleSplitEditor}
      aria-label="Dividir el editor"
      aria-pressed={ui.splitEditor}
      title="Dividir el editor: dos archivos lado a lado ({MOD}\)"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 5h16a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1zM12 5v14" />
      </svg>
    </button>

    <button
      class:active={ui.previewVisible}
      onclick={togglePreview}
      aria-label="Vista previa"
      aria-pressed={ui.previewVisible}
      title="Mostrar u ocultar la vista previa ({MOD}⌥P)"
    >
      <!-- Lucide "panel-right" -->
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <rect width="18" height="18" x="3" y="3" rx="2" />
        <path d="M15 3v18" />
      </svg>
    </button>

    <button
      class:active={terminalOpen}
      onclick={() => toggleBottomTab('terminal')}
      aria-label="Terminal CI"
      aria-pressed={terminalOpen}
      title="Terminal CI (Ctrl+` · {MOD}J)"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 5h16a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z" />
        <path d="M7 10l3 2-3 2M12 15h5" />
      </svg>
      {#if terminal.running}<span class="badge busy" aria-hidden="true"></span>{/if}
    </button>

    <button
      onclick={() => (ui.healthOpen = true)}
      aria-label="Estado del sistema"
      title="Estado del sistema"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3 12h4l3-8 4 16 3-8h4" />
      </svg>
    </button>
  </div>
</nav>

<style>
  /* Chrome: same surface as the Sidebar, no border (A3: surfaces separate). */
  .actionbar {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0;
    background: var(--surface-base);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  button {
    position: relative;
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  button:hover:not(:disabled) {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  /* Keyboard only: the one place besides A2's list where the accent shows. */
  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  /* Active tool (open drawer, split editor, terminal): neutral, never the accent. */
  button.active,
  button:active:not(:disabled) {
    color: var(--text-primary);
    background: var(--surface-active);
  }

  button:disabled {
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

  /* Compiling: the icon turns into a quiet spinner (A1: success and progress are neutral). */
  .compile.busy {
    color: var(--text-primary);
  }

  .compile.busy svg {
    animation: spin 0.9s linear infinite;
  }

  .badge {
    position: absolute;
    top: 6px;
    right: 6px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--danger); /* last compilation failed */
  }

  .badge.busy {
    background: var(--text-secondary); /* a terminal command is running */
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .compile.busy svg {
      animation: none;
    }
  }
</style>
