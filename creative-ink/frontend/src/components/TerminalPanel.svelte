<script>
  import { tick } from 'svelte'
  import { cancelCommand, commandHistory, runCommand, terminal } from '../lib/terminal.svelte.js'
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
  <div class="output" bind:this={output} role="log" aria-live="polite" aria-label="Salida de la terminal">
    {#if terminal.entries.length === 0}
      <p class="hint">
        Ejecuta comandos en <strong>{folderName}</strong> (p. ej. <code>git status</code>,
        <code>tlmgr info siunitx</code>). Sin TTY: los comandos interactivos (sudo, editores) no
        funcionan. Límite de 30 s · <code>clear</code> limpia · ↑/↓ historial.
      </p>
    {/if}
    {#each terminal.entries as entry (entry.id)}
      {@const info = entry.status === 'running' ? null : summary(entry)}
      <div class="entry">
        <div class="command"><span class="prompt">$</span> {entry.command}</div>
        {#if entry.output}<pre>{entry.output}</pre>{/if}
        {#if info}
          <div class="meta meta-{info.kind}">{info.text}</div>
        {:else}
          <div class="meta meta-running">Ejecutando…</div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="input-row">
    <span class="prompt" title={workspace.currentFolderPath}>{folderName} $</span>
    <input
      bind:this={input}
      bind:value
      {onkeydown}
      disabled={!workspace.currentFolderPath}
      placeholder={terminal.running ? 'Ejecutando… (Ctrl+C para cancelar)' : 'Escribe un comando y pulsa Enter'}
      aria-label="Comando de terminal"
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
    font-family: ui-monospace, 'SF Mono', Menlo, monospace;
    font-size: 0.75rem;
  }

  .output {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.4rem 0.75rem;
  }

  .hint {
    margin: 0;
    color: #6b6b6b;
    line-height: 1.5;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  }

  .hint code {
    color: #9d9d9d;
  }

  .entry + .entry {
    margin-top: 0.5rem;
  }

  .command {
    color: #e5e5e5;
  }

  .prompt {
    color: #4ade80;
  }

  pre {
    margin: 0.15rem 0 0;
    color: #bdbdbd;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .meta {
    margin-top: 0.1rem;
    font-size: 0.65rem;
  }

  .meta-ok {
    color: #6b6b6b;
  }

  .meta-running,
  .meta-warn {
    color: #fbbf24;
  }

  .meta-bad {
    color: #f87171;
  }

  .input-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.3rem 0.75rem;
    border-top: 1px solid #2b2b2b;
  }

  .input-row .prompt {
    flex-shrink: 0;
    max-width: 40%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  input {
    flex: 1;
    min-width: 0;
    padding: 0.2rem 0;
    border: 0;
    outline: none;
    background: transparent;
    color: #e5e5e5;
    font: inherit;
  }

  input::placeholder {
    color: #5a5a5a;
  }

  .stop {
    flex-shrink: 0;
    padding: 0.1rem 0.6rem;
    border: 1px solid #5c4a1f;
    border-radius: 4px;
    background: transparent;
    color: #fbbf24;
    font: inherit;
    cursor: pointer;
  }
</style>
