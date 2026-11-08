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
  .log {
    height: 100%;
    margin: 0;
    padding: 0.5rem 0.75rem;
    overflow: auto;
    font-family: ui-monospace, 'SF Mono', Menlo, monospace;
    font-size: 0.75rem;
    line-height: 1.45;
    color: #e5c0c0;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .clean {
    margin: 0;
    padding: 0.75rem;
    font-size: 0.8rem;
    color: #6b6b6b;
  }
</style>
