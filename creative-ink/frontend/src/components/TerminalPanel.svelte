<script>
  import { onMount, tick, untrack } from 'svelte'
  import { ListTerminalCommands } from '../../wailsjs/go/main/App.js'
  import { openSystemTerminal } from '../lib/actions.js'
  import LocationBanner from './LocationBanner.svelte'
  import { CI_COMMANDS, cancelCommand, commandHistory, runCommand, terminal, toggleFullOutput } from '../lib/terminal.svelte.js'
  import { baseName, workspace } from '../lib/workspace.svelte.js'

  /**
   * context 'ci' (Terminal CI: the `ci` prefix is implied) or 'shell' (the
   * system shell, as typed). One instance per bottom-panel tab.
   * @type {{ active: boolean, context: import('../lib/terminal.svelte.js').TerminalContext }}
   */
  let { active, context } = $props()
  const isCI = untrack(() => context) === 'ci' // a panel never changes context

  /** @type {HTMLInputElement} */
  let input
  /** @type {HTMLDivElement} */
  let output
  let value = $state('')
  // Position while browsing history with ↑/↓ (-1 = editing a new command).
  let historyIndex = -1
  let draft = ''

  let folderName = $derived(baseName(workspace.currentFolderPath) || 'sin carpeta')
  // Only this context's runs; the history is kept apart too.
  let entries = $derived(terminal.entries.filter((e) => e.context === context))
  // `ci` works without a project (help, config); the shell needs a folder.
  let disabled = $derived(!isCI && !workspace.currentFolderPath)
  let busyElsewhere = $derived(terminal.running && terminal.runningContext !== context)

  // Shell: the OS's own terminal, for what this one cannot do (sudo, prompts).
  let nativeError = $state('')
  async function openNative() {
    nativeError = await openSystemTerminal()
  }

  // ── Terminal CI completion, from Go's `ci` registry ─────────────────────
  /** @typedef {{ completion: string, syntax: string, description: string }} Completion */
  /** Until the registry arrives (or if the binding is missing): the local list. */
  /** @type {Completion[]} */
  let completions = $state(CI_COMMANDS.map((c) => ({ completion: c, syntax: c, description: '' })))

  onMount(async () => {
    if (!isCI) return
    try {
      const registry = await ListTerminalCommands()
      const list = registry.flatMap((cmd) =>
        (cmd.subcommands ?? []).map((sc) => ({ completion: sc.completion, syntax: sc.syntax, description: sc.description }))
      )
      if (list.length) completions = list
    } catch {
      // Older bindings: keep the local list.
    }
  })

  /** Completions that extend what is typed (case-insensitive), without duplicates. */
  let matches = $derived.by(() => {
    if (!isCI || !value || value !== value.trimStart()) return []
    const typed = value.toLowerCase()
    const seen = new Set()
    return completions.filter((c) => {
      const ok = c.completion.toLowerCase().startsWith(typed) && c.completion.length > typed.length && !seen.has(c.completion)
      seen.add(c.completion)
      return ok
    })
  })

  // Grey text after the cursor, only when the match is unambiguous.
  let suggestion = $derived(matches.length === 1 ? matches[0].completion.slice(value.length) : '')

  // Several matches: a light menu above the input; Tab cycles through it.
  /** @type {Completion[]} */
  let menu = $state([])
  let menuIndex = $state(-1)

  function closeMenu() {
    menu = []
    menuIndex = -1
  }

  /** @param {string[]} words */
  function commonPrefix(words) {
    let prefix = words[0] ?? ''
    for (const w of words) while (!w.startsWith(prefix)) prefix = prefix.slice(0, -1)
    return prefix
  }

  /** Tab: complete one match, the common prefix of several, or cycle the menu. */
  function complete() {
    if (menu.length) {
      menuIndex = (menuIndex + 1) % menu.length
      value = menu[menuIndex].completion
      return true
    }
    if (matches.length === 1) {
      value = matches[0].completion
      return true
    }
    if (matches.length > 1) {
      const prefix = commonPrefix(matches.map((m) => m.completion))
      if (prefix.length > value.length) value = prefix
      menu = matches.slice(0, 8)
      menuIndex = -1
      return true
    }
    return false
  }

  /** @param {Completion} item */
  function pick(item) {
    value = item.completion
    closeMenu()
    input?.focus()
  }

  let placeholder = $derived(
    terminal.running
      ? busyElsewhere
        ? `Hay un comando en ${terminal.runningContext === 'ci' ? 'la Terminal CI' : 'la Shell'}: espera o detenlo`
        : 'Ejecutando… (Ctrl+C para cancelar)'
      : isCI
        ? "Escribe 'help' o un comando de LaTeX sin 'ci': compile completo, clean all, trust status… (Tab completa)"
        : 'Comando de la shell del sistema, en la carpeta del proyecto…'
  )

  // Focus the input whenever the tab is shown.
  $effect(() => {
    if (active) tick().then(() => input?.focus())
  })

  // Keep the newest output in view.
  $effect(() => {
    const last = entries.at(-1)
    void last?.output
    void last?.status
    void last?.progress
    void last?.full.text
    void entries.length
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
      closeMenu()
      void runCommand(command, context)
    } else if (event.key === 'Tab' && !event.shiftKey && isCI) {
      // Only taken when there is something to complete: otherwise Tab moves focus.
      if (complete()) event.preventDefault()
    } else if (event.key === 'ArrowRight' && suggestion && input.selectionStart === value.length) {
      event.preventDefault()
      value += suggestion
    } else if (event.key === 'Escape' && menu.length) {
      event.preventDefault()
      closeMenu()
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
    const history = commandHistory(context)
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
  <div
    class="output"
    bind:this={output}
    role="log"
    aria-live="polite"
    aria-label={isCI ? 'Salida de la Terminal CI' : 'Salida de la shell del sistema'}
  >
    {#if entries.length === 0}
      {#if isCI}
        <p class="hint">
          <span class="ci-prompt">CI ❯</span> Comandos de LaTeX de Creative-ink, <strong>sin escribir
          <code>ci</code></strong>: <code>compile completo</code>, <code>clean all</code>,
          <code>trust status</code>… <code>help</code> los lista. Tab completa · ↑/↓ historial ·
          <code>clear</code> limpia.
        </p>
      {:else}
        <p class="hint">
          <span class="shell-prompt">$</span> Shell del sistema en <strong>{folderName}</strong>: la línea se
          ejecuta tal cual (p. ej. <code>git status</code>, <code>tlmgr info siunitx</code>). Sin TTY: los
          comandos interactivos (sudo, editores) no funcionan; para ellos, <strong>Terminal del sistema</strong>.
          Límite de 30 s · <code>clear</code> limpia · ↑/↓ historial.
        </p>
      {/if}
    {/if}
    {#each entries as entry (entry.id)}
      {#if entry.kind === 'notice' && entry.notice}
        <!-- Where the folder just opened lives (Go's default messages). -->
        <div class="entry notice-line notice-{entry.notice.level}" role="note">
          <div class="notice-title">{entry.notice.level === 'warning' ? '⚠' : 'ℹ'} {entry.notice.title}</div>
          <div class="notice-text">{entry.notice.text}</div>
          {#if entry.notice.steps?.length}<pre>{entry.notice.steps.join('\n')}</pre>{/if}
        </div>
      {:else}
      {@const info = entry.status === 'running' ? null : summary(entry)}
      <div class="entry">
        <!-- The same prompt as the input: every run is easy to spot. -->
        <div class="command">
          {#if isCI}
            <span class="ci-prompt" aria-hidden="true">CI ❯</span>
          {:else}
            <span class="shell-prompt" aria-hidden="true">$</span>
          {/if}
          {entry.display}
        </div>
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
      {/if}
    {/each}
  </div>

  <LocationBanner />

  {#if nativeError}
    <div class="notice" role="alert">
      <span>{nativeError}</span>
      <button class="link" onclick={() => (nativeError = '')}>Cerrar</button>
    </div>
  {/if}
  <div class="input-row">
    <span class="cwd" title={workspace.currentFolderPath}>{folderName}</span>
    {#if isCI}
      <span class="ci-prompt" aria-hidden="true">CI ❯</span>
    {:else}
      <span class="shell-prompt" aria-hidden="true">$</span>
    {/if}
    <div class="field">
      {#if menu.length}
        <ul class="menu" role="listbox" aria-label="Comandos de la Terminal CI">
          {#each menu as item, i (item.completion)}
            <li role="option" aria-selected={i === menuIndex}>
              <button class:current={i === menuIndex} onmousedown={(e) => e.preventDefault()} onclick={() => pick(item)}>
                <span class="syntax">{item.syntax}</span>
                {#if item.description}<span class="desc">{item.description}</span>{/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
      <!-- Ghost completion: the typed text (invisible) + the rest, in grey. -->
      {#if suggestion}
        <span class="ghost" aria-hidden="true"><span class="typed">{value}</span>{suggestion}</span>
      {/if}
      <input
        bind:this={input}
        bind:value
        {onkeydown}
        oninput={closeMenu}
        onblur={closeMenu}
        {disabled}
        {placeholder}
        aria-label={isCI ? 'Comando de la Terminal CI (sin ci)' : 'Comando de la shell del sistema'}
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
      />
    </div>
    {#if terminal.running && !busyElsewhere}
      <button class="stop" onclick={cancelCommand} title="Cancelar (Ctrl+C)">Detener</button>
    {/if}
    {#if !isCI}
      <button
        class="native"
        onclick={openNative}
        disabled={!workspace.currentFolderPath}
        title="Abrir la terminal del sistema en {folderName} (sudo, comandos interactivos, tu shell)"
      >
        <!-- Lucide "square-terminal" -->
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="m7 11 2-2-2-2" />
          <path d="M11 13h4" />
          <rect width="18" height="18" x="3" y="3" rx="2" />
        </svg>
        Terminal del sistema
      </button>
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

  /* Location notices in the history: light (info) or strong (warning). */
  .notice-line {
    padding: 0.25rem 0 0.25rem 0.6rem;
    border-left: 2px solid var(--border-strong);
    font-family: var(--font-ui);
    font-size: var(--text-xs);
    line-height: 1.45;
    color: var(--text-muted);
  }

  .notice-line.notice-warning {
    border-left-color: var(--warning);
    background: var(--warning-soft);
    color: var(--text-secondary);
  }

  .notice-title {
    font-weight: 600;
    color: var(--text-secondary);
  }

  .notice-warning .notice-title {
    color: var(--warning);
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

  /* The system shell keeps the traditional, neutral prompt. */
  .shell-prompt {
    flex-shrink: 0;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .field {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
  }

  /* Completion menu: a light popover above the input (A3: raised surface,
     overlay shadow, medium radius). */
  .menu {
    position: absolute;
    left: 0;
    bottom: calc(100% + 4px);
    z-index: 2;
    min-width: 20rem;
    max-width: 100%;
    margin: 0;
    padding: 0.25rem;
    list-style: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface-raised);
    box-shadow: var(--shadow-overlay);
  }

  .menu button {
    width: 100%;
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    padding: 0.2rem 0.5rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-primary);
    font: inherit;
    text-align: left;
    cursor: pointer;
    transition: background-color var(--transition);
  }

  .menu button:hover,
  .menu button.current {
    background: var(--surface-hover);
  }

  .menu .desc {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    font-family: var(--font-ui);
    font-size: var(--text-xs);
  }

  /* Drawn exactly under the input's text (same font and padding). */
  .ghost {
    position: absolute;
    inset: 0;
    padding: 0.2rem 0;
    overflow: hidden;
    white-space: pre;
    color: var(--text-disabled);
    font: inherit;
    pointer-events: none;
  }

  .ghost .typed {
    color: transparent;
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
    position: relative;
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

  /* Secondary action (A2), like Detener, with its thin-stroke icon (A5). */
  .native {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.1rem 0.6rem;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font-family: var(--font-ui);
    font-size: var(--text-xs);
    white-space: nowrap;
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  .native:hover:not(:disabled) {
    background: var(--surface-hover);
    color: var(--text-primary);
  }

  .native:disabled {
    color: var(--text-disabled);
    cursor: default;
  }

  .native:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .native svg {
    width: 14px;
    height: 14px;
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .notice {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    padding: 0.25rem 0.75rem;
    border-top: 1px solid var(--border);
    color: var(--danger);
    font-family: var(--font-ui);
    font-size: var(--text-xs);
  }
</style>
