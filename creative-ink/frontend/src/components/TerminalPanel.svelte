<script>
  import { tick } from 'svelte'
  import { cancelCommand, commandHistory, runCommand, terminal, toggleFullOutput } from '../lib/terminal.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  /** @type {{ active: boolean }} */
  let { active } = $props()

  /** @type {HTMLInputElement} */
  let input
  /** @type {HTMLDivElement} */
  let output
  let value = $state('')
  // Position while browsing history with ↑/↓ (-1 = editing a new command).
  let historyIndex = -1
  let draft = ''

  let folderName = $derived(baseName(workspace.currentFolderPath) || 'sin carpeta')

  // Focus the input whenever the tab is shown.
  $effect(() => {
    if (active) tick().then(() => input?.focus())
  })

  // Keep the newest output in view.
  $effect(() => {
    const last = terminal.entries.at(-1)
    void last?.output
    void last?.status
    void last?.progress
    void last?.full.text
    void terminal.entries.length
    tick().then(() => {
      if (output) output.scrollTop = output.scrollHeight
    })
  })

  /** @param {KeyboardEvent} event */
  function onkeydown(event) {
    if (event.key === 'Enter') {
      event.preventDefault()
      const command = value
      value = ''
      historyIndex = -1
      void runCommand(command)
    } else if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      event.preventDefault()
      browseHistory(event.key === 'ArrowUp' ? -1 : 1)
    } else if (event.ctrlKey && event.key.toLowerCase() === 'c' && terminal.running) {
      event.preventDefault()
      cancelCommand()
    }
  }

  /** @param {-1 | 1} step */
  function browseHistory(step) {
    const history = commandHistory()
    if (!history.length) return
    if (historyIndex === -1) {
      if (step === 1) return
      draft = value
      historyIndex = history.length
    }
    historyIndex += step
    if (historyIndex >= history.length) {
      historyIndex = -1
      value = draft
      return
    }
    historyIndex = Math.max(0, historyIndex)
    value = history[historyIndex]
  }

  /**
   * "Pasada 2/4 · xelatex · página 143 de 212 · 34 s" and the overall share.
   * @param {import('../lib/terminal.svelte.js').BuildProgress} p
   */
  function progressInfo(p) {
    const parts = [`Pasada ${p.pass}/${p.passes}`, p.tool]
    if (p.page) parts.push(p.pages ? `página ${p.page} de ${p.pages}` : `página ${p.page}`)
    parts.push(`${Math.round(p.elapsedMs / 1000)} s`)
    const passShare = p.pages && p.page ? Math.min(1, p.page / p.pages) : 0
    return { text: parts.join(' · '), percent: Math.round(((p.pass - 1 + passShare) / p.passes) * 100) }
  }

  function kb(/** @type {number} */ bytes) {
    return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${Math.ceil(bytes / 1024)} KB`
  }

  /** @param {import('../lib/terminal.svelte.js').TerminalEntry} entry */
  function summary(entry) {
    const seconds = `${(entry.durationMs / 1000).toFixed(1)} s`
    if (entry.status === 'error') return { text: 'no se pudo ejecutar', kind: 'bad' }
    if (entry.timedOut) return { text: `detenido: superó 30 s`, kind: 'warn' }
    if (entry.canceled) return { text: `cancelado · ${seconds}`, kind: 'warn' }
    if (entry.exitCode !== 0) return { text: `código ${entry.exitCode} · ${seconds}`, kind: 'bad' }
    return { text: seconds, kind: 'ok' }
  }
</script>

<div class="terminal">
  <div class="output" bind:this={output} role="log" aria-live="polite" aria-label="Salida de la terminal CI">
    {#if terminal.entries.length === 0}
      <p class="hint">
        <span class="ci-prompt">CI ❯</span> Terminal de Creative-ink en <strong>{folderName}</strong>.
        <code>ci help</code> lista los comandos de LaTeX (compilar, limpiar, configuración, confianza);
        cualquier otra línea va a la shell del sistema (p. ej. <code>git status</code>,
        <code>tlmgr info siunitx</code>). Sin TTY: los comandos interactivos (sudo, editores) no
        funcionan. Límite de 30 s · <code>clear</code> limpia · ↑/↓ historial.
      </p>
    {/if}
    {#each terminal.entries as entry (entry.id)}
      {@const info = entry.status === 'running' ? null : summary(entry)}
      <div class="entry">
        <!-- The same prompt as the input: every run is easy to spot. -->
        <div class="command"><span class="ci-prompt" aria-hidden="true">CI ❯</span> {entry.command}</div>
        {#if entry.output}<pre>{entry.output}</pre>{/if}
        {#if entry.status === 'running' && entry.progress && !entry.progress.done}
          {@const p = progressInfo(entry.progress)}
          <div class="progress" role="status">
            <span>{p.text}</span>
            <span class="bar" aria-hidden="true"><span style:width="{p.percent}%"></span></span>
          </div>
        {/if}
        {#if entry.full.open}
          {#if entry.full.truncated}
            <div class="meta">… se muestran los últimos 256 KB de {kb(entry.full.size)}</div>
          {/if}
          <pre class="full">{entry.full.text}</pre>
          {#if entry.full.error}<div class="meta meta-bad">{entry.full.error}</div>{/if}
        {/if}
        <div class="meta-row">
          {#if info}
            <span class="meta meta-{info.kind}">{info.text}</span>
          {:else}
            <span class="meta meta-running">Ejecutando…</span>
          {/if}
          {#if entry.runId}
            <button class="link" onclick={() => toggleFullOutput(entry)}>
              {entry.full.open ? 'Ocultar salida completa' : 'Salida completa'}
            </button>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div class="input-row">
    <span class="cwd" title={workspace.currentFolderPath}>{folderName}</span>
    <span class="ci-prompt" aria-hidden="true">CI ❯</span>
    <input
      bind:this={input}
      bind:value
      {onkeydown}
      disabled={!workspace.currentFolderPath}
      placeholder={terminal.running
        ? 'Ejecutando… (Ctrl+C para cancelar)'
        : "Escribe 'ci help' para comandos de LaTeX, o usa la shell del sistema..."}
      aria-label="Comando de la terminal CI"
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
    />
    {#if terminal.running}
      <button class="stop" onclick={cancelCommand} title="Cancelar (Ctrl+C)">Detener</button>
    {/if}
  </div>
</div>

<style>
  .terminal {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--surface-editor);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .output {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.4rem 0.75rem;
  }

  .hint {
    margin: 0;
    color: var(--text-muted);
    line-height: 1.5;
    font-family: var(--font-ui);
    font-size: var(--text-sm);
  }

  .hint code {
    color: var(--text-secondary);
    font-family: var(--font-mono);
  }

  .entry + .entry {
    margin-top: 0.5rem;
  }

  .command {
    color: var(--text-primary);
  }

  /* The Terminal CI's own prompt (A2, accent rule 4): sets it apart from
     the system terminal and marks where every run starts in the history. */
  .ci-prompt {
    flex-shrink: 0;
    color: var(--accent);
    font-family: var(--font-mono);
    font-weight: 600;
    white-space: nowrap;
  }

  /* The folder it runs in: present but quiet, before the prompt. */
  .cwd {
    flex-shrink: 1;
    min-width: 0;
    max-width: 40%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }

  pre {
    margin: 0.15rem 0 0;
    color: var(--text-secondary);
    font-family: var(--font-mono);
    white-space: pre-wrap;
    word-break: break-word;
  }

  .meta {
    margin-top: 0.1rem;
    font-size: var(--text-xs);
  }

  .meta-row {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
  }

  /* Links are not accent-coloured (A2): secondary text, underlined on hover. */
  .link {
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    font-size: var(--text-xs);
    text-decoration: underline transparent;
    text-underline-offset: 2px;
    cursor: pointer;
    transition: color var(--transition), text-decoration-color var(--transition);
  }

  .link:hover {
    color: var(--text-primary);
    text-decoration-color: currentColor;
  }

  .link:focus-visible,
  .stop:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .progress {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin-top: 0.2rem;
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  .bar {
    flex: 0 0 120px;
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

  pre.full {
    max-height: 40vh;
    overflow-y: auto;
    padding: 0.3rem 0.5rem;
    border-left: 2px solid var(--border-strong);
    color: var(--text-muted);
  }

  .meta-ok {
    color: var(--text-muted);
  }

  .meta-running {
    color: var(--text-secondary);
  }

  .meta-warn {
    color: var(--warning);
  }

  .meta-bad {
    color: var(--danger);
  }

  .input-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.3rem 0.75rem;
    border-top: 1px solid var(--border);
  }


  input {
    flex: 1;
    min-width: 0;
    padding: 0.2rem 0;
    border: 0;
    outline: none;
    background: transparent;
    color: var(--text-primary);
    font: inherit;
  }

  input::placeholder {
    color: var(--text-disabled);
  }

  /* Secondary action (A2): neutral, outlined. */
  .stop {
    flex-shrink: 0;
    padding: 0.1rem 0.6rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    font-size: var(--text-xs);
    cursor: pointer;
    transition: background-color var(--transition), border-color var(--transition);
  }

  .stop:hover {
    background: var(--surface-hover);
  }
</style>
