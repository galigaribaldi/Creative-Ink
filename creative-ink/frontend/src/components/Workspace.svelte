<script>
  import { compilation } from '../lib/compilation.svelte.js'
  import { ui } from '../lib/ui.svelte.js'
  import { compileTarget } from '../lib/workspace.svelte.js'
  import BottomPanel from './BottomPanel.svelte'
  import Editor from './Editor.svelte'
  import Preview from './Preview.svelte'
  import TrustBanner from './TrustBanner.svelte'
</script>

<!-- Layout only: the main Editor and the Preview are never re-keyed or put
     inside {#if}, so CodeMirror and the PDF.js canvas keep their state across
     UI changes. The second pane (split editor, ⌘\) is mounted only while
     split: closing it saves its edits first (actions.js toggleSplitEditor). -->
<div class="workspace" class:split={ui.splitEditor}>
  <section class="pane editor-pane" aria-label="Editor">
    <!-- Untrusted .latexmkrc: compiling with -norc; offers to trust it. -->
    <TrustBanner />
    <div class="editors">
      <div class="editor-host">
        <Editor pane="primary" />
      </div>
      {#if ui.splitEditor}
        <div class="editor-host secondary" aria-label="Segundo editor">
          <Editor pane="secondary" />
        </div>
      {/if}
    </div>
    <!-- "Errores LaTeX" | "Terminal". Always mounted; hides itself. -->
    <BottomPanel />
  </section>

  <section class="pane" aria-label="Vista previa">
    <Preview
      compileState={compilation.status}
      pdfVersion={compilation.pdfVersion}
      documentKey={compileTarget()}
    />
  </section>
</div>

<style>
  .workspace {
    height: 100%;
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  /* Two editors share two thirds of the width; the preview keeps one. */
  .workspace.split {
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
  }

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .editor-pane {
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--border);
  }

  .editors {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(0, 1fr);
  }

  .editor-host {
    min-width: 0;
    min-height: 0;
  }

  .editor-host.secondary {
    border-left: 1px solid var(--border);
  }
</style>
