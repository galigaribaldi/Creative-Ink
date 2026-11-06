<script>
  import { onDestroy } from 'svelte'
  import ActionBar from './components/ActionBar.svelte'
  import HealthPanel from './components/HealthPanel.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Workspace from './components/Workspace.svelte'
  import { listenCompileEvents } from './lib/compilation.svelte.js'
  import { toggleSidebar, ui } from './lib/ui.svelte.js'

  const ACTIONBAR_WIDTH = '44px'
  const SIDEBAR_WIDTH = '240px'

  // Subscribed during init, before the Editor mounts and triggers its first
  // save, so no compile event can be missed.
  onDestroy(listenCompileEvents())

  /** Cmd/Ctrl+B toggles the sidebar (Cmd/Ctrl+S is handled by CodeMirror). */
  function onkeydown(event) {
    if ((event.metaKey || event.ctrlKey) && !event.shiftKey && event.key.toLowerCase() === 'b') {
      event.preventDefault()
      toggleSidebar()
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
