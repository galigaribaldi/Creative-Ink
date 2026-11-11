<script>
  // Where the open project lives (docs/MAP.md §4.7), above the input of the
  // Terminal CI and the Shell. Go writes the notices (default messages) and
  // decides their level; this only shows them: light (info) for a folder
  // outside the repository without git, strong (warning) with git or a
  // synced folder. Hidden until another folder is opened.
  import { untrack } from 'svelte'
  import { ui } from '../lib/ui.svelte.js'
  import { workspace } from '../lib/workspace.svelte.js'
  import NoticeList from './NoticeList.svelte'

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
    <NoticeList {notices} folded={!expanded} />
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
