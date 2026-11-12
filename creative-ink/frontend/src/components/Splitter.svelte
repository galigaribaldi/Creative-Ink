<script>
  // A draggable divider (docs/MAP.md §4.4, #9, block A): between the two
  // editors, between the editors and the PDF preview, and above the bottom
  // panel. 4px hit area with a 1px hairline; pointer, keyboard (arrows,
  // Home) and double click (back to the default). It only reports ratios:
  // the parent owns them (ui.svelte.js, localStorage).

  /**
   * orientation: 'vertical' divides columns (col-resize), 'horizontal' rows.
   * ratio: the parent's value. It is the share of the element BEFORE the
   * divider, or of the one after it with invert (preview on the left, the
   * bottom panel below the editors).
   * @type {{
   *   orientation: 'vertical' | 'horizontal',
   *   ratio: number,
   *   invert?: boolean,
   *   label: string,
   *   container: () => HTMLElement | undefined,
   *   onchange: (ratio: number, persist: boolean) => void,
   *   onreset: () => void
   * }}
   */
  let { orientation, ratio, invert = false, label, container, onchange, onreset } = $props()

  const KEY_STEP = 0.05
  let dragging = $state(false)

  /** Pointer position → ratio in the parent's terms. @param {PointerEvent} e @param {DOMRect} rect */
  function ratioAt(e, rect) {
    const r = orientation === 'vertical' ? (e.clientX - rect.left) / rect.width : (e.clientY - rect.top) / rect.height
    return invert ? 1 - r : r
  }

  /** @param {PointerEvent} event */
  function onpointerdown(event) {
    if (event.button !== 0) return
    const el = container()
    if (!el) return
    event.preventDefault() // no text selection while dragging
    const rect = el.getBoundingClientRect()
    dragging = true
    // While dragging: the cursor stays the same everywhere and CodeMirror or
    // the PDF canvas do not take the pointer (style.css, body.resizing).
    document.body.classList.add('resizing', orientation === 'vertical' ? 'resizing-col' : 'resizing-row')
    const onmove = (/** @type {PointerEvent} */ e) => onchange(ratioAt(e, rect), false)
    const onup = (/** @type {PointerEvent} */ e) => {
      dragging = false
      document.body.classList.remove('resizing', 'resizing-col', 'resizing-row')
      window.removeEventListener('pointermove', onmove)
      window.removeEventListener('pointerup', onup)
      onchange(ratioAt(e, rect), true) // remembered across sessions
    }
    window.addEventListener('pointermove', onmove)
    window.addEventListener('pointerup', onup)
  }

  /** ←/→ (or ↑/↓) move it, Home resets it. @param {KeyboardEvent} event */
  function onkeydown(event) {
    const keys = orientation === 'vertical' ? { ArrowLeft: -1, ArrowRight: 1 } : { ArrowUp: -1, ArrowDown: 1 }
    const dir = keys[/** @type {keyof typeof keys} */ (event.key)]
    if (dir) onchange(ratio + (invert ? -dir : dir) * KEY_STEP, true)
    else if (event.key === 'Home') onreset()
    else return
    event.preventDefault()
  }
</script>

<!-- A focusable separator is an ARIA widget: it takes the keyboard. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="splitter {orientation}"
  class:dragging
  role="separator"
  aria-orientation={orientation}
  aria-label={label}
  aria-valuemin="0"
  aria-valuemax="100"
  aria-valuenow={Math.round(ratio * 100)}
  tabindex="0"
  title="Arrastra para cambiar el tamaño · doble clic para restablecer"
  {onpointerdown}
  {onkeydown}
  ondblclick={onreset}
></div>

<style>
  .splitter {
    position: relative;
    flex-shrink: 0;
    background: transparent;
    transition: background-color var(--transition);
    touch-action: none;
  }

  .vertical {
    width: 4px;
    cursor: col-resize;
  }

  .horizontal {
    height: 4px;
    cursor: row-resize;
  }

  /* The 1px hairline in the middle. */
  .splitter::before {
    content: '';
    position: absolute;
    background: var(--border);
  }

  .vertical::before {
    inset: 0 auto 0 50%;
    width: 1px;
  }

  .horizontal::before {
    inset: 50% 0 auto 0;
    height: 1px;
  }

  .splitter:hover,
  .dragging {
    background: var(--surface-hover);
  }

  .splitter:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }
</style>
