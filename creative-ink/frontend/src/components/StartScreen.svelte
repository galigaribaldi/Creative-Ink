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
          <button class="primary" onclick={() => run(createQuickDocument)} disabled={busy}>Crear documento rápido</button>
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
    background: rgba(0, 0, 0, 0.6);
  }

  .panel {
    width: min(760px, 100%);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 1.5rem 1.75rem;
    border: 1px solid #3c3c3c;
    border-radius: 8px;
    background: #1e1e1e;
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
  }

  h1 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 500;
  }

  .repo {
    margin: 0.25rem 0 0;
    font-family: ui-monospace, 'SF Mono', Menlo, monospace;
    font-size: 0.7rem;
    color: #6b6b6b;
    overflow-wrap: anywhere;
  }

  .close {
    padding: 0.2rem 0.5rem;
    border: 0;
    background: transparent;
    color: #8a8a8a;
    font: inherit;
    cursor: pointer;
  }

  .warning,
  .stale p {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.45;
    color: #fbbf24;
  }

  .stale {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.6rem 0.8rem;
    border: 1px solid #5c4a1f;
    border-radius: 6px;
    background: #1f1a10;
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
    border: 1px solid #3c3c3c;
    border-radius: 6px;
    background: #252526;
  }

  .title {
    font-weight: 600;
  }

  .detail,
  .hint {
    font-size: 0.75rem;
    line-height: 1.4;
    color: #9d9d9d;
  }

  .hint {
    color: #6b6b6b;
  }

  input {
    padding: 0.4rem 0.5rem;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    background: #1b1b1b;
    color: #e5e5e5;
    font: inherit;
    font-size: 0.85rem;
  }

  input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .primary {
    margin-top: auto;
    padding: 0.45rem 0.75rem;
    border: 0;
    border-radius: 4px;
    background: var(--accent);
    color: #fff;
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .primary:disabled {
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
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #9d9d9d;
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
    border-radius: 4px;
    background: transparent;
    color: #cccccc;
    font: inherit;
    font-size: 0.8rem;
    text-align: left;
    cursor: pointer;
  }

  li button:hover:not(:disabled) {
    background: #2a2d2e;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .when {
    flex-shrink: 0;
    color: #6b6b6b;
    font-size: 0.7rem;
  }

  code {
    font-size: 0.7rem;
    color: #bdbdbd;
  }

  footer {
    display: flex;
    justify-content: flex-end;
  }

  .link {
    flex-shrink: 0;
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .muted {
    margin: 0;
    color: #6b6b6b;
  }

  .error {
    margin: 0;
    font-size: 0.8rem;
    color: #f87171;
  }

  @media (max-width: 620px) {
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
