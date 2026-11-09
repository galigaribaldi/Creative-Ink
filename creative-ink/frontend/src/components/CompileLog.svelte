<script>
  // Content of the "Errores LaTeX" tab of the bottom panel.
  /** @type {{ logs: string, failed: boolean }} */
  let { logs, failed } = $props()
</script>

{#if failed && logs}
  <pre class="log" role="alert" aria-label="Errores de compilación de LaTeX">{logs}</pre>
{:else}
  <p class="clean">✓ Sin errores de compilación.</p>
{/if}

<style>
  /* The log is content: primary text in the editor's font. The tab's dot
     and the role already say it is an error, so it is not painted red. */
  .log {
    height: 100%;
    margin: 0;
    padding: 0.5rem 0.75rem;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--text-primary);
    white-space: pre-wrap;
    word-break: break-word;
  }

  /* Quiet success (A1). */
  .clean {
    margin: 0;
    padding: 0.75rem;
    font-size: var(--text-md);
    color: var(--text-muted);
  }
</style>
