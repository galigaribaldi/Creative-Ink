<script>
  import { compileDocument, saveAs, saveDocument, setEditorMode } from '../lib/actions.js'
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

  /** @type {{ id: import('../lib/ui.svelte.js').EditorMode, label: string, hint: string }[]} */
  const MODES = [
    { id: 'pdf', label: 'Solo PDF', hint: 'Oculta el código: para presentar (Beamer) o enseñar un artículo' },
    { id: 'minimal', label: 'Minimalista', hint: 'Un panel de código y la vista previa' },
    { id: 'multi', label: 'Multiventana', hint: `Pestañas y dos paneles (${MOD}\\ divide)` }
  ]
  let currentMode = $derived(MODES.find((m) => m.id === ui.editorMode) ?? MODES[1])

  /** @param {KeyboardEvent} event */
  function onmenukeydown(event) {
    if (event.key === 'Escape') ui.modeMenuOpen = false
  }
</script>

<svelte:window onclick={() => (ui.modeMenuOpen = false)} onkeydown={onmenukeydown} />

<!-- Lucide icons of the three editor modes (ISC licence, lucide.dev). -->
{#snippet modeIcon(/** @type {string} */ id)}
  <svg viewBox="0 0 24 24" aria-hidden="true">
    {#if id === 'pdf'}
      <!-- "presentation" -->
      <path d="M2 3h20" /><path d="M21 3v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V3" /><path d="m7 21 5-5 5 5" />
    {:else if id === 'minimal'}
      <!-- "columns-2" -->
      <rect width="18" height="18" x="3" y="3" rx="2" /><path d="M12 3v18" />
    {:else}
      <!-- "panels-top-left": tabs above, panes below -->
      <rect width="18" height="18" x="3" y="3" rx="2" /><path d="M3 9h18" /><path d="M9 21V9" />
    {/if}
  </svg>
{/snippet}

<nav class="actionbar" aria-label="Barra de acciones">
  <div class="group">
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

    <!-- The launcher: projects, recents, import, "Abrir otra carpeta…" (⌘O opens a folder directly). -->
    <button onclick={() => (ui.startOpen = true)} aria-label="Proyectos" title="Proyectos: inicio, recientes, importar y abrir carpeta ({MOD}O abre una carpeta)">
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
    <!-- Editor mode: one button with the current mode's icon; a side menu
         offers the three (docs/MAP.md §4.4, #9). -->
    <div class="mode-wrap">
      <button
        class:active={ui.modeMenuOpen}
        onclick={(e) => (e.stopPropagation(), (ui.modeMenuOpen = !ui.modeMenuOpen))}
        aria-label="Modo del editor: {currentMode.label}"
        aria-haspopup="menu"
        aria-expanded={ui.modeMenuOpen}
        title="Modo del editor: {currentMode.label}"
      >
        {@render modeIcon(currentMode.id)}
      </button>
      {#if ui.modeMenuOpen}
        <div class="mode-menu" role="menu" aria-label="Modo del editor">
          {#each MODES as mode (mode.id)}
            <button
              role="menuitemradio"
              aria-checked={mode.id === ui.editorMode}
              class:current={mode.id === ui.editorMode}
              onclick={(e) => (e.stopPropagation(), void setEditorMode(mode.id))}
            >
              {@render modeIcon(mode.id)}
              <span class="text">
                <span class="label">{mode.label}</span>
                <span class="hint">{mode.hint}</span>
              </span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

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

  /* Mode menu: a side flyout next to the bar (raised surface, A3). */
  .mode-wrap {
    position: relative;
  }

  .mode-menu {
    position: absolute;
    top: 0;
    left: calc(100% + 6px);
    z-index: 30;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    width: 17rem;
    padding: 0.3rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
  }

  .mode-menu button {
    width: 100%;
    height: auto;
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    padding: 0.45rem 0.55rem;
    text-align: left;
  }

  .mode-menu button.current {
    background: var(--surface-active);
  }

  .mode-menu svg {
    flex-shrink: 0;
    margin-top: 0.1rem;
  }

  .mode-menu .text {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .mode-menu .label {
    color: var(--text-primary);
    font-size: var(--text-md);
  }

  .mode-menu .current .label {
    font-weight: 600;
  }

  .mode-menu .hint {
    color: var(--text-muted);
    font-size: var(--text-xs);
    line-height: 1.35;
    white-space: normal;
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
