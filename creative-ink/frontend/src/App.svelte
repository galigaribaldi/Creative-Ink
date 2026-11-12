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
  import { listenConfigEvents } from './lib/config.svelte.js'
  import { guardSystemDrops, importer, listenDropEvents, listenImportEvents } from './lib/import.svelte.js'
  import ImportDialog from './components/ImportDialog.svelte'
  import Toast from './components/Toast.svelte'
  import { listenTerminalEvents } from './lib/terminal.svelte.js'
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
  // `ci config …` in the terminal updates the UI at once (config:changed).
  onDestroy(listenConfigEvents(() => workspace.currentFolderPath))
  // Terminal runs: id of the spooled output and build progress.
  onDestroy(listenTerminalEvents())
  onDestroy(listenImportEvents())

  // ⌘+ / ⌘- / ⌘0: the text tokens of style.css scale with --zoom.
  $effect(() => {
    document.documentElement.style.setProperty('--zoom', String(ui.zoom))
  })
  onDestroy(listenDropEvents())
  onDestroy(guardSystemDrops())
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
   * Cmd/Ctrl+N untitled document in this window · Cmd/Ctrl+Shift+N new window ·
   * Cmd/Ctrl+Alt+N quick document
   * Cmd/Ctrl+S save + compile · Cmd/Ctrl+Shift+S save as
   * Cmd/Ctrl+B sidebar · Cmd/Ctrl+O open folder · Cmd/Ctrl+\ split editor
   * Ctrl+` or Cmd/Ctrl+J terminal (` is a dead key on Spanish layouts)
   *
   * preventDefault also keeps the macOS menu (menu.go) from running the same
   * command again: WKWebView only passes unhandled keys to the menu.
   */
  function onkeydown(event) {
    if (!(event.metaKey || event.ctrlKey)) return
    // ⌥⌘N quick document, ⌥⌘P preview (event.code: ⌥ turns the key into
    // another character on macOS).
    if (event.altKey) {
      const alt = event.shiftKey ? '' : { KeyN: 'new-quick-doc', KeyP: 'toggle-preview' }[event.code]
      if (alt) {
        event.preventDefault()
        runCommand(alt)
      }
      return
    }
    // ⌘+ / ⌘- / ⌘0: zoom of the UI and editor text ("=" is "+" without
    // Shift on US layouts; "+" has its own key on Spanish ones).
    const zoom = { '+': 'zoom-in', '=': 'zoom-in', '-': 'zoom-out', '0': 'zoom-reset' }[event.key]
    if (zoom) {
      event.preventDefault()
      runCommand(zoom)
      return
    }
    const key = event.key.toLowerCase()
    const shift = event.shiftKey
    let command = ''
    if ((event.ctrlKey && event.code === 'Backquote') || (key === 'j' && !shift)) command = 'toggle-terminal'
    else if (key === 'n') command = shift ? 'new-window' : 'new-file'
    else if (key === 's') command = shift ? 'save-as' : 'save' // no "Save page" from the webview
    else if (key === 'f' && shift) command = 'search-project' // ⌘F stays CodeMirror's find
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

<!-- "Importar proyecto…" and drops from the system ("¿Qué desea hacer?"). -->
{#if importer.preview}
  {#key importer.preview.id}
    <ImportDialog preview={importer.preview} />
  {/key}
{/if}

<Toast />

<style>
  .shell {
    height: 100%;
    display: grid;
    grid-template-columns: var(--actionbar-w) var(--sidebar-w) minmax(0, 1fr);
    background: var(--surface-base);
    /* Short slide: Preview's ResizeObserver is debounced (150 ms), so the
       PDF re-renders once after the drawer settles, not on every frame.
       --duration is 0s with prefers-reduced-motion (style.css). */
    transition: grid-template-columns var(--transition);
  }

  .main {
    min-width: 0;
    min-height: 0;
  }
</style>
