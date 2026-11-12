<script>
  import { untrack } from 'svelte'
  import { compilation } from '../lib/compilation.svelte.js'
  import { terminal } from '../lib/terminal.svelte.js'
  import { closeBottomPanel, ui } from '../lib/ui.svelte.js'
  import CompileLog from './CompileLog.svelte'
  import TerminalPanel from './TerminalPanel.svelte'

  const TABS = [
    { id: 'problems', label: 'Errores LaTeX' },
    { id: 'terminal', label: 'Terminal CI', context: 'ci' },
    { id: 'shell', label: 'Shell', context: 'shell' }
  ]

  let failed = $derived(compilation.status === 'error' && compilation.logs !== '')

  // A compile error opens the panel on its tab, unless the user already has
  // the panel open (then only the tab's red dot tells). If the error opened
  // it, a later successful compilation closes it again.
  $effect(() => {
    const status = compilation.status
    untrack(() => {
      if (status === 'error' && failed && !ui.bottomPanelOpen) {
        ui.bottomTab = 'problems'
        ui.bottomPanelOpen = true
        ui.bottomAutoOpened = true
      } else if (status === 'success' && ui.bottomAutoOpened) {
        closeBottomPanel()
      }
    })
  })

  /** @param {import('../lib/ui.svelte.js').BottomTab} tab */
  function select(tab) {
    ui.bottomTab = tab
    ui.bottomAutoOpened = false
  }
</script>

<!-- Always mounted (hidden when closed): the terminal keeps its scroll and
     the half-typed command when the panel or the tab changes. -->
<section class="panel" class:closed={!ui.bottomPanelOpen} aria-label="Panel inferior">
  <div class="tabs" role="tablist">
    {#each TABS as tab (tab.id)}
      <button
        role="tab"
        id="tab-{tab.id}"
        aria-selected={ui.bottomTab === tab.id}
        aria-controls="tabpanel-{tab.id}"
        class:selected={ui.bottomTab === tab.id}
        onclick={() => select(tab.id)}
      >
        {tab.label}
        {#if tab.id === 'problems' && failed}<span class="dot dot-error" aria-label="hay errores"></span>{/if}
        {#if tab.context && terminal.runningContext === tab.context}<span class="dot dot-busy" aria-label="ejecutando"></span>{/if}
      </button>
    {/each}
    <button class="close" onclick={closeBottomPanel} aria-label="Cerrar panel" title="Cerrar panel">✕</button>
  </div>

  <div id="tabpanel-problems" role="tabpanel" aria-labelledby="tab-problems" class="content" hidden={ui.bottomTab !== 'problems'}>
    <CompileLog logs={compilation.logs} {failed} />
  </div>
  <!-- Two separate contexts (own prompt, history and runs): Terminal CI
       adds the `ci` prefix itself; Shell sends the line as typed. -->
  <div id="tabpanel-terminal" role="tabpanel" aria-labelledby="tab-terminal" class="content" hidden={ui.bottomTab !== 'terminal'}>
    <TerminalPanel context="ci" active={ui.bottomPanelOpen && ui.bottomTab === 'terminal'} />
  </div>
  <div id="tabpanel-shell" role="tabpanel" aria-labelledby="tab-shell" class="content" hidden={ui.bottomTab !== 'shell'}>
    <TerminalPanel context="shell" active={ui.bottomPanelOpen && ui.bottomTab === 'shell'} />
  </div>
</section>

<style>
  /* Tabs on the chrome surface; the content (log, terminal) on the editor's
     surface, like a text canvas. One hairline separates it from the editor. */
  /* The height is dragged on the divider above it (Workspace.svelte). */
  .panel {
    height: var(--bottom-h, 35%);
    min-height: 120px;
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--border);
    background: var(--surface-editor);
  }

  .panel.closed {
    display: none;
  }

  .tabs {
    display: flex;
    align-items: center;
    gap: 0.15rem;
    padding: 0.2rem 0.4rem;
    background: var(--surface-base);
  }

  .tabs button {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.6rem;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-xs);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    cursor: pointer;
    transition: background-color var(--transition), color var(--transition);
  }

  .tabs button:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  /* Active tab: neutral (A2: the accent is not for tabs). */
  .tabs button.selected {
    color: var(--text-primary);
    background: var(--surface-active);
  }

  .tabs button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .tabs .close {
    margin-left: auto;
    text-transform: none;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .dot-error {
    background: var(--danger);
  }

  .dot-busy {
    background: var(--text-secondary);
  }

  .content {
    flex: 1;
    min-height: 0;
  }

  .content[hidden] {
    display: none;
  }
</style>
