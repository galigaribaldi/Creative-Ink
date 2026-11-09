<script>
  import { onDestroy, untrack } from 'svelte'
  import ActionBar from './components/ActionBar.svelte'
  import HealthPanel from './components/HealthPanel.svelte'
  import RepositorySetup from './components/RepositorySetup.svelte'
  import StartScreen from './components/StartScreen.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Workspace from './components/Workspace.svelte'
  import {
    initWorkspace,
    listenAppEvents,
    listenMenuEvents,
    listenWorkspaceEvents,
    runCommand
  } from './lib/actions.js'
  import { listenCompileEvents, resetCompilation } from './lib/compilation.svelte.js'
  import { ui } from './lib/ui.svelte.js'
  import { compileTarget, workspace } from './lib/workspace.svelte.js'

  const ACTIONBAR_WIDTH = '44px'
  const SIDEBAR_WIDTH = '240px'

  // Subscribed during init, before the first file is opened (opening a file
  // queues its compilation), so no compile event can be missed.
  onDestroy(listenCompileEvents())
  // External changes (a pasted image, a new chapter) refresh the explorer.
  onDestroy(listenWorkspaceEvents())
  // Closing the window flushes CodeMirror first (Go's OnBeforeClose).
  onDestroy(listenAppEvents())
  // Native menu clicks (macOS): same commands as the shortcuts below.
  onDestroy(listenMenuEvents())
  void initWorkspace()

  // A different compile target (another file, or the main file changed): the
  // previous status and log no longer apply. Switching chapters while a main
  // file is set keeps the target, so nothing is reset.
  $effect(() => {
    compileTarget()
    untrack(resetCompilation)
  })

  /**
   * Global shortcuts (they also fire inside CodeMirror: its keydown bubbles).
   * Editing commands (undo, find, ⌘D, ⌘/…) belong to CodeMirror's keymap.
   * Cmd/Ctrl+N new file · Cmd/Ctrl+Shift+N quick document
   * Cmd/Ctrl+S save + compile · Cmd/Ctrl+Shift+S save as
   * Cmd/Ctrl+B sidebar · Cmd/Ctrl+O open folder · Cmd/Ctrl+\ split editor
   * Ctrl+` or Cmd/Ctrl+J terminal (` is a dead key on Spanish layouts)
   *
   * preventDefault also keeps the macOS menu (menu.go) from running the same
   * command again: WKWebView only passes unhandled keys to the menu.
   */
  function onkeydown(event) {
    if (!(event.metaKey || event.ctrlKey) || event.altKey) return
    const key = event.key.toLowerCase()
    const shift = event.shiftKey
    let command = ''
    if ((event.ctrlKey && event.code === 'Backquote') || (key === 'j' && !shift)) command = 'toggle-terminal'
    else if (key === 'n') command = shift ? 'new-quick-doc' : 'new-file'
    else if (key === 's') command = shift ? 'save-as' : 'save' // no "Save page" from the webview
    else if (key === 'b' && !shift) command = 'toggle-sidebar'
    else if (key === 'o' && !shift) command = 'open-folder'
    // ⇧⌘\ is CodeMirror's "go to matching bracket".
    else if ((key === '\\' || event.code === 'Backslash') && !shift) command = 'split-editor'
    if (!command) return
    event.preventDefault()
    runCommand(command)
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

<!-- First run only: choose where the central repository lives. -->
{#if workspace.needsSetup}
  <RepositorySetup />
{:else if workspace.needsStart || ui.startOpen}
  <!-- Every session: new project, quick document, or an existing one. -->
  <StartScreen />
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
