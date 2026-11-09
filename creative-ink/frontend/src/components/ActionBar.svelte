<script>
  import { compileDocument, openFolder, saveAs, saveDocument, toggleSplitEditor } from '../lib/actions.js'
  import { compilation } from '../lib/compilation.svelte.js'
  import { terminal } from '../lib/terminal.svelte.js'
  import { toggleBottomTab, toggleTool, ui } from '../lib/ui.svelte.js'

  // macOS shows ⌘, Linux shows Ctrl in the tooltips.
  const IS_MAC = navigator.platform.toUpperCase().includes('MAC')
  const MOD = IS_MAC ? '⌘' : 'Ctrl+'
  const SHIFT_MOD = IS_MAC ? '⇧⌘' : 'Ctrl+Shift+'

  let explorerActive = $derived(ui.sidebarOpen && ui.activeTool === 'explorer')
  let compiling = $derived(compilation.status === 'compiling')
  let compileFailed = $derived(compilation.status === 'error')
  let terminalOpen = $derived(ui.bottomPanelOpen && ui.bottomTab === 'terminal')
</script>

<nav class="actionbar" aria-label="Barra de acciones">
  <div class="group">
    <button onclick={() => (ui.startOpen = true)} aria-label="Inicio" title="Inicio: proyectos y documentos rápidos">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 11l8-7 8 7M6 9.5V20h12V9.5" />
      </svg>
    </button>

    <button
      class:active={explorerActive}
      onclick={() => toggleTool('explorer')}
      aria-label="Explorador"
      aria-pressed={explorerActive}
      title="Explorador ({MOD}B)"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
        <path d="M14 3v5h5M9 13h6M9 17h6" />
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
      class:active={terminalOpen}
      onclick={() => toggleBottomTab('terminal')}
      aria-label="Terminal"
      aria-pressed={terminalOpen}
      title="Terminal (Ctrl+` · {MOD}J)"
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
  .actionbar {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0;
    border-right: 1px solid #2b2b2b;
    background: #181818;
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
    border-left: 2px solid transparent;
    border-radius: 4px;
    background: transparent;
    color: #8a8a8a;
    cursor: pointer;
  }

  button:hover:not(:disabled) {
    color: #e5e5e5;
    background: #242424;
  }

  button:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }

  button.active {
    color: #e5e5e5;
    border-left-color: var(--accent);
    border-radius: 0 4px 4px 0;
  }

  button:disabled {
    cursor: default;
  }

  svg {
    width: 20px;
    height: 20px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .compile.busy {
    color: #fbbf24;
  }

  .compile.busy svg {
    animation: spin 0.9s linear infinite;
  }

  .badge.busy {
    background: #fbbf24;
  }

  .badge {
    position: absolute;
    top: 6px;
    right: 6px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #f87171;
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
