<script>
  // "Buscar" in the sidebar (docs/MAP.md §4.4, #9): text across the
  // project's files, like VS Code. Go searches (SearchProject); a click
  // opens the file and selects the match.
  import { tick, untrack } from 'svelte'
  import { openMatch, scheduleSearch, search } from '../lib/search.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'

  /** @type {HTMLInputElement | undefined} */
  let input = $state()
  /** Files whose matches the user folded, by path. @type {Record<string, boolean>} */
  let folded = $state({})

  // ⇧⌘F, or the tool was just opened: focus the field.
  $effect(() => {
    void search.focusNonce
    if (ui.sidebarOpen && ui.activeTool === 'search') {
      untrack(() =>
        tick().then(() => {
          input?.focus()
          input?.select()
        })
      )
    }
  })

  // Another project: its files, not the old results.
  let lastFolder = ''
  $effect(() => {
    const folder = workspace.currentFolderPath
    untrack(() => {
      if (folder !== lastFolder) {
        lastFolder = folder
        if (search.text) scheduleSearch()
      }
    })
  })

  /** @param {'caseSensitive' | 'wholeWord' | 'regex'} key */
  function toggle(key) {
    search[key] = !search[key]
    scheduleSearch()
  }

  let summary = $derived(
    search.error
      ? ''
      : !search.text.trim()
        ? ''
        : search.total === 0
          ? search.busy
            ? 'Buscando…'
            : 'Sin resultados'
          : `${search.total} ${search.total === 1 ? 'resultado' : 'resultados'} en ${search.files.length} ${search.files.length === 1 ? 'archivo' : 'archivos'}`
  )
</script>

<div class="search">
  <div class="field">
    <input
      bind:this={input}
      bind:value={search.text}
      oninput={scheduleSearch}
      onkeydown={(e) => e.key === 'Enter' && scheduleSearch()}
      placeholder="Buscar en el proyecto"
      aria-label="Buscar en los archivos del proyecto"
      disabled={!workspace.currentFolderPath}
      autocomplete="off"
      spellcheck="false"
    />
    <div class="toggles">
      <button class:on={search.caseSensitive} aria-pressed={search.caseSensitive} onclick={() => toggle('caseSensitive')} title="Distinguir mayúsculas y minúsculas">Aa</button>
      <button class:on={search.wholeWord} aria-pressed={search.wholeWord} onclick={() => toggle('wholeWord')} title="Solo palabras completas">ab</button>
      <button class:on={search.regex} aria-pressed={search.regex} onclick={() => toggle('regex')} title="Expresión regular">.*</button>
    </div>
  </div>

  {#if !workspace.currentFolderPath}
    <p class="hint">Abre una carpeta para buscar en sus archivos.</p>
  {:else if search.error}
    <p class="error" role="alert">{search.error}</p>
  {:else if summary}
    <p class="hint" role="status">{summary}</p>
  {/if}

  {#if search.files.length}
    <ul class="results" aria-label="Resultados">
      {#each search.files as file (file.path)}
        <li>
          <button class="file" onclick={() => (folded[file.path] = !folded[file.path])} aria-expanded={!folded[file.path]} title={file.rel}>
            <span class="chevron" class:open={!folded[file.path]} aria-hidden="true">›</span>
            <span class="name">{file.rel.split('/').pop()}</span>
            <span class="dir">{file.rel.includes('/') ? file.rel.slice(0, file.rel.lastIndexOf('/')) : ''}</span>
            <span class="count">{file.matches.length}</span>
          </button>
          {#if !folded[file.path]}
            <ul>
              {#each file.matches as m (m.line * 100000 + m.column)}
                {@const chars = [...m.preview]}
                <li>
                  <button class="match" onclick={() => openMatch(file, m)} title="Línea {m.line}, columna {m.column}">
                    <span class="line">{m.line}</span>
                    <span class="preview">{chars.slice(0, m.start).join('')}<mark>{chars.slice(m.start, m.end).join('')}</mark>{chars.slice(m.end).join('')}</span>
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </li>
      {/each}
    </ul>
    {#if search.truncated}
      <p class="hint">Hay más resultados: afina la búsqueda para verlos todos.</p>
    {/if}
  {/if}
</div>

<style>
  .search {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .field {
    position: relative;
    display: flex;
    margin: 0 0.5rem 0.4rem;
  }

  input {
    flex: 1;
    min-width: 0;
    padding: 0.3rem 5.2rem 0.3rem 0.5rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-editor);
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-sm);
  }

  input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }

  /* Aa / ab / .* inside the field, like VS Code. */
  .toggles {
    position: absolute;
    top: 50%;
    right: 0.25rem;
    display: flex;
    gap: 0.1rem;
    transform: translateY(-50%);
  }

  .toggles button {
    min-width: 1.5rem;
    padding: 0.05rem 0.2rem;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    cursor: pointer;
    transition: color var(--transition), background-color var(--transition), border-color var(--transition);
  }

  .toggles button:hover {
    color: var(--text-primary);
  }

  .toggles button.on {
    border-color: var(--border-strong);
    background: var(--surface-active);
    color: var(--text-primary);
  }

  .toggles button:focus-visible,
  .results button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }

  .hint,
  .error {
    margin: 0;
    padding: 0.2rem 0.9rem 0.5rem;
    font-size: var(--text-xs);
    line-height: 1.4;
    color: var(--text-muted);
  }

  .error {
    color: var(--danger);
  }

  .results,
  .results ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .results button {
    width: 100%;
    display: flex;
    align-items: baseline;
    gap: 0.4rem;
    padding: 0.15rem 0.5rem;
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

  .results button:hover {
    background: var(--surface-hover);
    color: var(--text-primary);
  }

  .chevron {
    flex-shrink: 0;
    display: inline-block;
    width: 0.6rem;
    color: var(--text-muted);
    transition: transform var(--transition);
  }

  .chevron.open {
    transform: rotate(90deg);
  }

  .name {
    flex-shrink: 0;
    color: var(--text-primary);
  }

  .dir {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  .count {
    flex-shrink: 0;
    padding: 0 0.35rem;
    border-radius: var(--radius-sm);
    background: var(--surface-active);
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  .results .match {
    padding-left: 1.5rem;
  }

  .line {
    flex-shrink: 0;
    min-width: 2.2rem;
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    text-align: right;
  }

  .preview {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  /* The match: a neutral highlight, never the accent (A2). */
  mark {
    border-radius: 2px;
    background: var(--selection);
    color: var(--text-primary);
  }
</style>
