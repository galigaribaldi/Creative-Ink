<script>
  import { untrack } from 'svelte'
  import { compilation } from '../lib/compilation.svelte.js'
  import { terminal } from '../lib/terminal.svelte.js'
  import { closeBottomPanel, ui } from '../lib/ui.svelte.js'
  import CompileLog from './CompileLog.svelte'
  import TerminalPanel from './TerminalPanel.svelte'

  const TABS = [
    { id: 'problems', label: 'Errores LaTeX' },
    { id: 'terminal', label: 'Terminal' }
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

  /** @param {'problems' | 'terminal'} tab */
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
        {#if tab.id === 'terminal' && terminal.running}<span class="dot dot-busy" aria-label="ejecutando"></span>{/if}
      </button>
    {/each}
    <button class="close" onclick={closeBottomPanel} aria-label="Cerrar panel" title="Cerrar panel">✕</button>
  </div>

  <div id="tabpanel-problems" role="tabpanel" aria-labelledby="tab-problems" class="content" hidden={ui.bottomTab !== 'problems'}>
    <CompileLog logs={compilation.logs} {failed} />
  </div>
  <div id="tabpanel-terminal" role="tabpanel" aria-labelledby="tab-terminal" class="content" hidden={ui.bottomTab !== 'terminal'}>
    <TerminalPanel active={ui.bottomPanelOpen && ui.bottomTab === 'terminal'} />
  </div>
</section>

<style>
  .panel {
    height: 35%;
    min-height: 120px;
    display: flex;
    flex-direction: column;
    border-top: 1px solid #333;
    background: #181818;
  }

  .panel.closed {
    display: none;
  }

  .tabs {
    display: flex;
    align-items: stretch;
    gap: 0.1rem;
    padding: 0 0.4rem;
    border-bottom: 1px solid #2b2b2b;
  }

  .tabs button {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.3rem 0.6rem;
    border: 0;
    border-bottom: 1px solid transparent;
    background: transparent;
    color: #8a8a8a;
    font: inherit;
    font-size: 0.7rem;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    cursor: pointer;
  }

  .tabs button:hover {
    color: #e5e5e5;
  }

  .tabs button.selected {
    color: #e5e5e5;
    border-bottom-color: var(--accent);
  }

  .tabs button:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
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
    background: #f87171;
  }

  .dot-busy {
    background: #fbbf24;
  }

  .content {
    flex: 1;
    min-height: 0;
  }

  .content[hidden] {
    display: none;
  }
</style>
