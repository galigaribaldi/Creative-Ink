<script>
  import { onMount } from 'svelte'
  import { GetStartScreen, RevealQuickDocuments } from '../../wailsjs/go/main/App.js'
  import { createProject, createQuickDocument, openFolder, openProject } from '../lib/actions.js'
  import { ui } from '../lib/ui.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  /** @type {import('../../wailsjs/go/models').main.StartScreen | null} */
  let screen = $state(null)
  let error = $state('')
  let busy = $state(false)
  let projectName = $state('')
  /** @type {HTMLInputElement | undefined} */
  let nameInput = $state()

  // Closable only when a project is open behind it (otherwise nothing to show).
  let closable = $derived(workspace.currentFolderPath !== '')

  const relative = new Intl.RelativeTimeFormat('es', { numeric: 'auto' })
  /** @param {string} iso */
  function ago(iso) {
    const t = Date.parse(iso)
    if (!t || t < 0) return ''
    const minutes = Math.round((t - Date.now()) / 60000)
    if (Math.abs(minutes) < 60) return relative.format(minutes, 'minute')
    const hours = Math.round(minutes / 60)
    if (Math.abs(hours) < 48) return relative.format(hours, 'hour')
    return relative.format(Math.round(hours / 24), 'day')
  }

  onMount(async () => {
    try {
      screen = await GetStartScreen()
    } catch (err) {
      error = String(err)
    }
  })

  /** @param {() => Promise<string>} action */
  async function run(action) {
    busy = true
    error = await action()
    busy = false
  }

  function onsubmit(event) {
    event.preventDefault()
    if (projectName.trim()) void run(() => createProject(projectName))
  }

  async function reveal() {
    try {
      await RevealQuickDocuments()
    } catch (err) {
      error = String(err)
    }
  }

  function onkeydown(event) {
    if (event.key === 'Escape' && closable) ui.startOpen = false
  }
</script>

<svelte:window {onkeydown} />

<div class="backdrop">
  <div class="panel" role="dialog" aria-modal="true" aria-labelledby="start-title">
    <header>
      <div>
        <h1 id="start-title">¿Cómo quieres trabajar?</h1>
        {#if screen}<p class="repo" title={screen.repositoryPath}>{screen.repositoryPath}</p>{/if}
      </div>
      {#if closable}
        <button class="close" onclick={() => (ui.startOpen = false)} aria-label="Cerrar" title="Cerrar (Esc)">✕</button>
      {/if}
    </header>

    {#if screen}
      {#each screen.warnings as warning}
        <p class="warning" role="alert">⚠️ {warning}</p>
      {/each}

      {#if screen.staleQuickDocs.length}
        <div class="stale" role="status">
          <p>
            Tienes <strong>{screen.staleQuickDocs.length}</strong>
            {screen.staleQuickDocs.length === 1 ? 'documento rápido' : 'documentos rápidos'} sin modificar desde
            hace más de {screen.staleAfterHours} h:
            {screen.staleQuickDocs.map((d) => d.name).join(', ')}. El editor no los borra: revísalos y elimina los
            que ya no necesites.
          </p>
          <button class="link" onclick={reveal}>Mostrar tmp-Workspaces</button>
        </div>
      {/if}

      <div class="choices">
        <form class="choice" {onsubmit}>
          <span class="title">Nuevo proyecto</span>
          <span class="detail">Una carpeta propia con plantilla (main.tex como principal, capítulos, figuras y bibliografía). Para trabajos largos.</span>
          <input
            bind:this={nameInput}
            bind:value={projectName}
            placeholder="Nombre del proyecto"
            aria-label="Nombre del proyecto"
            disabled={busy}
          />
          <span class="hint">Los espacios se convierten en «-» y se quitan los acentos del nombre de la carpeta.</span>
          <button type="submit" class="primary" disabled={busy || !projectName.trim()}>Crear proyecto</button>
        </form>

        <div class="choice">
          <span class="title">Documento rápido</span>
          <span class="detail">Un solo archivo en <code>tmp-Workspaces/unnamed-tmpN</code> con compilación ligera. Para notas y documentos cortos.</span>
          <button class="secondary" onclick={() => run(createQuickDocument)} disabled={busy}>Crear documento rápido</button>
        </div>
      </div>

      <div class="lists">
        {#if screen.projects.length}
          <section>
            <h2>Proyectos</h2>
            <ul>
              {#each screen.projects as p (p.path)}
                <li>
                  <button onclick={() => run(() => openProject(p.path))} disabled={busy} title={p.path}>
                    <span class="name">{p.name}</span><span class="when">{ago(p.modified)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if screen.quickDocs.length}
          <section>
            <h2>Documentos rápidos</h2>
            <ul>
              {#each screen.quickDocs as d (d.path)}
                <li>
                  <button onclick={() => run(() => openProject(d.path))} disabled={busy} title={d.path}>
                    <span class="name">{d.name}</span><span class="when">{ago(d.modified)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if screen.recent.length}
          <section>
            <h2>Recientes</h2>
            <ul>
              {#each screen.recent as path (path)}
                <li>
                  <button onclick={() => run(() => openProject(path))} disabled={busy} title={path}>
                    <span class="name">{baseName(path)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      </div>
    {:else if !error}
      <p class="muted">Cargando…</p>
    {/if}

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <footer>
      <button class="link" onclick={openFolder} disabled={busy}>Abrir otra carpeta…</button>
    </footer>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 15;
    display: grid;
    place-items: center;
    padding: 1rem;
    background: var(--scrim);
  }

  .panel {
    width: min(760px, 100%);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 1.5rem 1.75rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
    font-size: var(--text-md);
    color: var(--text-primary);
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
  }

  h1 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .repo {
    margin: 0.25rem 0 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }

  .close {
    padding: 0.2rem 0.5rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font: inherit;
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  .close:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .warning,
  .stale p {
    margin: 0;
    font-size: var(--text-md);
    line-height: 1.45;
    color: var(--warning);
  }

  .stale {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.6rem 0.8rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-base);
  }

  .choices {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  .choice {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    margin: 0;
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-base);
  }

  .title {
    font-weight: 500;
  }

  .detail,
  .hint {
    font-size: var(--text-sm);
    line-height: 1.4;
    color: var(--text-secondary);
  }

  .hint {
    color: var(--text-muted);
  }

  input {
    padding: 0.4rem 0.5rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-editor);
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-md);
    transition: border-color var(--transition);
  }

  input::placeholder {
    color: var(--text-disabled);
  }

  /* The focus ring is the only accent on inputs (A2 exception). */
  input:focus {
    outline: none;
  }

  input:focus-visible,
  .primary:focus-visible,
  .secondary:focus-visible,
  .close:focus-visible,
  .link:focus-visible,
  li button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  /* The one primary action (A2, accent rule 1). */
  .primary {
    margin-top: auto;
    padding: 0.45rem 0.75rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: var(--accent);
    color: var(--accent-contrast);
    font: inherit;
    font-size: var(--text-md);
    font-weight: 500;
    cursor: pointer;
    transition: background-color var(--transition), opacity var(--transition);
  }

  .primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  /* Secondary actions: neutral, outlined. */
  .secondary {
    margin-top: auto;
    padding: 0.45rem 0.75rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-md);
    cursor: pointer;
    transition: background-color var(--transition), opacity var(--transition);
  }

  .secondary:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  .primary:disabled,
  .secondary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .lists {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
  }

  h2 {
    margin: 0 0 0.4rem;
    font-size: var(--text-xs);
    font-weight: 500;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 180px;
    overflow-y: auto;
  }

  li button {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.3rem 0.5rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  li button:hover:not(:disabled) {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .when {
    flex-shrink: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }

  footer {
    display: flex;
    justify-content: flex-end;
  }

  /* Links are not accent-coloured (A2). */
  .link {
    flex-shrink: 0;
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    font-size: var(--text-md);
    text-decoration: underline transparent;
    text-underline-offset: 2px;
    cursor: pointer;
    transition: color var(--transition), text-decoration-color var(--transition);
  }

  .link:hover:not(:disabled) {
    color: var(--text-primary);
    text-decoration-color: currentColor;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .error {
    margin: 0;
    font-size: var(--text-md);
    color: var(--danger);
  }

  @media (max-width: 620px) {
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
