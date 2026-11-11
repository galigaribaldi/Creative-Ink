<script>
  // Import summary (docs/MAP.md §4.7, phase 2). Go analysed the folder and
  // wrote the notices; this asks for the name, the main file and the
  // history, and offers the two ways: copy into the repository or open in
  // place. With git (or for a collection) opening in place is the
  // recommended, primary action.
  import { untrack } from 'svelte'
  import { cancelImport, closeImport, copyImport, importer, openInPlace } from '../lib/import.svelte.js'
  import NoticeList from './NoticeList.svelte'

  /** @type {{ preview: import('../lib/import.svelte.js').ImportPreview }} */
  let { preview } = $props()

  const initial = untrack(() => preview) // a new analysis remounts this dialog
  let name = $state(initial.proposedName)
  let mainFile = $state(initial.mainCandidates[0] ?? '')
  let includeGit = $state(initial.hasGit) // decided on 2026-11-10: .git is copied
  let busy = $state(false)

  let underGit = $derived(preview.hasGit || preview.gitRoot !== '')
  let copyFirst = $derived(preview.canCopy && !underGit)
  let percent = $derived(
    importer.progress.totalBytes ? Math.min(100, Math.round((importer.progress.bytes / importer.progress.totalBytes) * 100)) : 0
  )

  function size(/** @type {number} */ bytes) {
    if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1).replace('.', ',')} GB`
    if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1).replace('.', ',')} MB`
    return `${Math.ceil(bytes / 1024)} KB`
  }

  async function copy() {
    if (!name.trim()) return
    busy = true
    await copyImport({ name, mainFile, includeGit })
    busy = false
  }

  async function inPlace() {
    busy = true
    await openInPlace()
    busy = false
  }

  function onkeydown(/** @type {KeyboardEvent} */ event) {
    if (event.key === 'Escape' && !importer.copying) {
      event.stopPropagation()
      closeImport()
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="backdrop">
  <div class="panel" role="dialog" aria-modal="true" aria-labelledby="import-title">
    <header>
      <h1 id="import-title">Importar proyecto</h1>
      <button class="close" onclick={closeImport} disabled={importer.copying} aria-label="Cerrar" title="Cerrar (Esc)">✕</button>
    </header>

    <p class="source" title={preview.source}><code>{preview.source}</code></p>

    <NoticeList notices={preview.notices} />

    {#if preview.canCopy}
      <form onsubmit={(e) => (e.preventDefault(), copy())}>
        <label>
          <span>Nombre del proyecto en My-Creative-Ink-Repository</span>
          <input bind:value={name} disabled={importer.copying} required autocomplete="off" spellcheck="false" />
          <small>Se normaliza con «-» y sin acentos, como todo lo que crea el editor.</small>
        </label>
        {#if preview.mainCandidates.length > 1}
          <label>
            <span>Archivo principal</span>
            <select bind:value={mainFile} disabled={importer.copying}>
              {#each preview.mainCandidates as c (c)}<option value={c}>{c}</option>{/each}
            </select>
          </label>
        {/if}
        {#if preview.hasGit}
          <label class="check">
            <input type="checkbox" bind:checked={includeGit} disabled={importer.copying} />
            <span>Incluir el historial git (.git · {size(preview.gitBytes)})</span>
          </label>
        {/if}
      </form>
    {:else if preview.copyBlocked}
      <p class="blocked">No se puede copiar: {preview.copyBlocked}.</p>
    {/if}

    {#if importer.copying}
      <div class="progress" role="status">
        <span>
          Copiando… {importer.progress.files} de {importer.progress.totalFiles} archivos ·
          {size(importer.progress.bytes)} de {size(importer.progress.totalBytes)}
        </span>
        <span class="bar" aria-hidden="true"><span style:width="{percent}%"></span></span>
      </div>
    {/if}

    {#if importer.error}<p class="error" role="alert">{importer.error}</p>{/if}

    <footer>
      {#if importer.copying}
        <button class="secondary" onclick={cancelImport}>Cancelar la copia</button>
      {:else}
        <button class="link" onclick={closeImport} disabled={busy}>Cancelar</button>
        {#if copyFirst}
          <button class="secondary" onclick={inPlace} disabled={busy}>Abrir en su ubicación</button>
          <button class="primary" onclick={copy} disabled={busy || !name.trim()}>Copiar a Creative-ink</button>
        {:else}
          {#if preview.canCopy}
            <button class="secondary" onclick={copy} disabled={busy || !name.trim()}>Copiar de todos modos</button>
          {/if}
          <button class="primary" onclick={inPlace} disabled={busy}>Abrir en su ubicación</button>
        {/if}
      {/if}
    </footer>
  </div>
</div>

<style>
  /* Above the start screen (z-index 15). */
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--scrim);
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    width: 36rem;
    max-width: calc(100vw - 2rem);
    max-height: calc(100vh - 2rem);
    overflow: auto;
    padding: 1.5rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--text-primary);
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  h1 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .source {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }

  code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  label span {
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  label.check {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
  }

  small {
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  input:not([type='checkbox']),
  select {
    padding: 0.35rem 0.5rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-editor);
    color: var(--text-primary);
    font: inherit;
  }

  input:focus-visible,
  select:focus-visible,
  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .blocked {
    margin: 0;
    color: var(--text-secondary);
  }

  .error {
    margin: 0;
    color: var(--danger);
  }

  .progress {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  .bar {
    height: 4px;
    border-radius: 2px;
    background: var(--surface-active);
    overflow: hidden;
  }

  /* Continuous progress: a linear width transition is allowed (A4). */
  .bar span {
    display: block;
    height: 100%;
    background: var(--text-secondary);
    transition: width var(--duration) linear;
  }

  footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.75rem;
  }

  button {
    border-radius: var(--radius-sm);
    font: inherit;
    cursor: pointer;
    transition: background-color var(--transition), border-color var(--transition), color var(--transition);
  }

  button:disabled {
    cursor: default;
    opacity: 0.55;
  }

  /* The one primary action (A2, accent rule 1). */
  .primary {
    padding: 0.4rem 0.9rem;
    border: 1px solid var(--accent);
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
  }

  .primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .secondary {
    padding: 0.4rem 0.9rem;
    border: 1px solid var(--border-strong);
    background: transparent;
    color: var(--text-primary);
  }

  .secondary:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  .link,
  .close {
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-secondary);
  }

  .link:hover:not(:disabled),
  .close:hover:not(:disabled) {
    color: var(--text-primary);
  }
</style>
