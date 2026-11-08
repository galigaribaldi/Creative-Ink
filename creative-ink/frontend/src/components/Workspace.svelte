<script>
  import { compilation } from '../lib/compilation.svelte.js'
  import { compileTarget } from '../lib/workspace.svelte.js'
  import CompileLog from './CompileLog.svelte'
  import Editor from './Editor.svelte'
  import Preview from './Preview.svelte'

  let showLog = $derived(compilation.status === 'error' && compilation.logs !== '')
</script>

<!-- Layout only: Editor and Preview are never re-keyed or put inside {#if},
     so CodeMirror and the PDF.js canvas keep their state across UI changes. -->
<div class="workspace">
  <section class="pane editor-pane" aria-label="Editor">
    <div class="editor-host">
      <Editor />
    </div>
    {#if showLog}
      <CompileLog logs={compilation.logs} />
    {/if}
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

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .editor-pane {
    display: flex;
    flex-direction: column;
    border-right: 1px solid #333;
  }

  .editor-host {
    flex: 1;
    min-height: 0;
  }
</style>
