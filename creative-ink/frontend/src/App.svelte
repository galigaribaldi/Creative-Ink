<script>
  import { onDestroy, untrack } from 'svelte'
  import ActionBar from './components/ActionBar.svelte'
  import HealthPanel from './components/HealthPanel.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Workspace from './components/Workspace.svelte'
  import { initWorkspace, openFolder, saveAs } from './lib/actions.js'
  import { listenCompileEvents, resetCompilation } from './lib/compilation.svelte.js'
  import { toggleSidebar, ui } from './lib/ui.svelte.js'
  import { workspace } from './lib/workspace.svelte.js'

  const ACTIONBAR_WIDTH = '44px'
  const SIDEBAR_WIDTH = '240px'

  // Subscribed during init, before the first file is opened (opening a file
  // queues its compilation), so no compile event can be missed.
  onDestroy(listenCompileEvents())
  void initWorkspace()

  // A different document: the previous one's compile status and log no
  // longer apply (Preview resets its own canvas through documentKey).
  $effect(() => {
    workspace.currentFilePath
    untrack(resetCompilation)
  })

  /**
   * Global shortcuts. Cmd/Ctrl+S is handled by CodeMirror itself.
   * Cmd/Ctrl+B sidebar · Cmd/Ctrl+O open folder · Cmd/Ctrl+Shift+S save as.
   */
  function onkeydown(event) {
    if (!(event.metaKey || event.ctrlKey) || event.altKey) return
    const key = event.key.toLowerCase()
    if (key === 'b' && !event.shiftKey) {
      event.preventDefault()
      toggleSidebar()
    } else if (key === 'o' && !event.shiftKey) {
      event.preventDefault()
      void openFolder()
    } else if (key === 's' && event.shiftKey) {
      event.preventDefault()
      void saveAs()
    }
  }
</script>

<svelte:window {onkeydown} />

<div
  class="shell"
  style:--actionbar-w={ACTIONBAR_WIDTH}
  style:--sidebar-w={ui.sidebarOpen ? SIDEBAR_WIDTH : '0px'}
>
  <ActionBar />
  <Sidebar />
  <main class="main">
    <Workspace />
  </main>
</div>

{#if ui.healthOpen}
  <HealthPanel onclose={() => (ui.healthOpen = false)} />
{/if}

<style>
  .shell {
    height: 100%;
    display: grid;
    grid-template-columns: var(--actionbar-w) var(--sidebar-w) minmax(0, 1fr);
    /* Short slide: Preview's ResizeObserver is debounced (150 ms), so the
       PDF re-renders once after the drawer settles, not on every frame. */
    transition: grid-template-columns 120ms ease-out;
  }

  .main {
    min-width: 0;
    min-height: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .shell {
      transition: none;
    }
  }
</style>
