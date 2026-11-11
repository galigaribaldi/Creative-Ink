<script>
  // Where the open project lives (docs/MAP.md §4.7), above the input of the
  // Terminal CI and the Shell. Go writes the notices (default messages) and
  // decides their level; this only shows them: light (info) for a folder
  // outside the repository without git, strong (warning) with git or a
  // synced folder. Hidden until another folder is opened.
  import { untrack } from 'svelte'
  import { ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'

  let notices = $derived(workspace.location.notices ?? [])
  let strong = $derived(notices.some((n) => n.level === 'warning'))
  let expanded = $state(false)

  // Another folder: show its notices again, folded.
  $effect(() => {
    void workspace.currentFolderPath
    untrack(() => {
      ui.locationBannerHidden = false
      expanded = false
    })
  })
</script>

{#if notices.length && !ui.locationBannerHidden}
  <div class="location" class:strong role="status" aria-label="Ubicación del proyecto">
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
            <span class="text" class:folded={!expanded}>{n.text}</span>
            {#if expanded && n.steps?.length}<pre>{n.steps.join('\n')}</pre>{/if}
          </div>
        </div>
      {/each}
    </div>
    <div class="actions">
      <button class="link" onclick={() => (expanded = !expanded)} aria-expanded={expanded}>
        {expanded ? 'Menos' : 'Detalles'}
      </button>
      <button
        class="close"
        onclick={() => (ui.locationBannerHidden = true)}
        aria-label="Ocultar el aviso de ubicación"
        title="Ocultar hasta abrir otra carpeta (ci status lo vuelve a mostrar)"
      >
        ×
      </button>
    </div>
  </div>
{/if}

<style>
  /* Light by default (A1: no modal, chrome surface, muted text). */
  .location {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    padding: 0.35rem 0.75rem;
    border-top: 1px solid var(--border);
    background: var(--surface-base);
    color: var(--text-secondary);
    font-family: var(--font-ui);
    font-size: var(--text-xs);
    line-height: 1.45;
  }

  /* Strong (git outside the repository, synced folder): warning edge and tint. */
  .location.strong {
    box-shadow: inset 3px 0 0 var(--warning);
    background: var(--warning-soft);
    color: var(--text-primary);
  }

  .notices {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .notice {
    display: flex;
    align-items: flex-start;
    gap: 0.45rem;
  }

  .notice svg {
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

  .body strong {
    margin-right: 0.4rem;
    font-weight: 600;
  }

  /* Folded: one line per notice, the rest with "Detalles". */
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

  .actions {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .link,
  .close {
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-secondary);
    font: inherit;
    cursor: pointer;
    transition: color var(--transition);
  }

  .link {
    text-decoration: underline transparent;
    text-underline-offset: 2px;
  }

  .link:hover,
  .close:hover {
    color: var(--text-primary);
    text-decoration-color: currentColor;
  }

  .close {
    font-size: var(--text-sm);
    line-height: 1;
  }

  .link:focus-visible,
  .close:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
