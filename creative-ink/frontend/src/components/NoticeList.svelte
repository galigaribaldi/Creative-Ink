<script>
  // Go's default messages (core.LocationNotice): light (info) or strong
  // (warning). Shared by the terminals' location banner and the import
  // summary, so both look and read the same. Only shows them.

  /**
   * folded: one line of text per notice and no steps (the banner).
   * @type {{ notices: import('../lib/workspace.svelte.js').LocationNotice[], folded?: boolean }}
   */
  let { notices, folded = false } = $props()
</script>

<div class="notices">
  {#each notices as n (n.id)}
    <div class="notice notice-{n.level}">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        {#if n.level === 'warning'}
          <!-- Lucide "triangle-alert" -->
          <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />
          <path d="M12 9v4" />
          <path d="M12 17h.01" />
        {:else}
          <!-- Lucide "info" -->
          <circle cx="12" cy="12" r="10" />
          <path d="M12 16v-4" />
          <path d="M12 8h.01" />
        {/if}
      </svg>
      <div class="body">
        <strong>{n.title}</strong>
        <span class="text" class:folded>{n.text}</span>
        {#if !folded && n.steps?.length}<pre>{n.steps.join('\n')}</pre>{/if}
      </div>
    </div>
  {/each}
</div>

<style>
  .notices {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .notice {
    display: flex;
    align-items: flex-start;
    gap: 0.45rem;
  }

  svg {
    flex-shrink: 0;
    width: 14px;
    height: 14px;
    margin-top: 0.1rem;
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .notice-info svg {
    color: var(--text-muted);
  }

  .notice-warning svg,
  .notice-warning strong {
    color: var(--warning);
  }

  .body {
    min-width: 0;
  }

  strong {
    margin-right: 0.4rem;
    font-weight: 600;
  }

  /* Folded: one line per notice. */
  .text.folded {
    display: -webkit-box;
    -webkit-line-clamp: 1;
    line-clamp: 1;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  pre {
    margin: 0.3rem 0 0.1rem;
    color: var(--text-secondary);
    font-family: var(--font-mono);
    white-space: pre-wrap;
  }
</style>
