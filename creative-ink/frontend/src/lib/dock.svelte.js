// Dragging editor panes and the preview around the dock (docs/MAP.md §4.4,
// #9, phase 3). The grip (⠿) of a pane's tab strip or of the preview's bar
// starts it; Workspace.svelte publishes the items' rectangles here and draws
// the drop hint. Hit-testing uses those rectangles, not the DOM (while
// dragging, body.resizing disables pointer events everywhere).
import { placeItem } from './layout.js'
import { panes, setLayout } from './panes.svelte.js'
import { ui } from './ui.svelte.js'

/** @typedef {import('./layout.js').ItemId} ItemId */
/** @typedef {'left' | 'right' | 'top' | 'bottom' | 'center'} Zone */

/** Within this share of an item's edge the drop goes to that side; else the center (swap). */
const EDGE = 0.28

export const dock = $state({
  /** Item being dragged ('' = none). @type {'' | ItemId} */
  dragging: '',
  /** Item under the pointer and where it would go. @type {'' | ItemId} */
  target: '',
  /** @type {'' | Zone} */
  zone: '',
  /** Set by Workspace.svelte: the visible items' rectangles (dock fractions). @type {Partial<Record<ItemId, import('./layout.js').Rect>>} */
  rects: {},
  /** Set by Workspace.svelte. @type {HTMLElement | null} */
  el: null
})

/** Which item and zone a point of the screen falls on. @param {number} x @param {number} y */
function hit(x, y) {
  const d = dock.el?.getBoundingClientRect()
  if (!d) return { target: /** @type {''} */ (''), zone: /** @type {''} */ ('') }
  const fx = (x - d.left) / d.width
  const fy = (y - d.top) / d.height
  for (const [id, r] of Object.entries(dock.rects)) {
    if (!r || fx < r.x || fx > r.x + r.w || fy < r.y || fy > r.y + r.h) continue
    const dx = (fx - r.x) / r.w
    const dy = (fy - r.y) / r.h
    const edges = /** @type {[Zone, number][]} */ ([
      ['left', dx],
      ['right', 1 - dx],
      ['top', dy],
      ['bottom', 1 - dy]
    ])
    const [side, dist] = edges.reduce((a, b) => (b[1] < a[1] ? b : a))
    return { target: /** @type {ItemId} */ (id), zone: dist < EDGE ? side : /** @type {Zone} */ ('center') }
  }
  return { target: /** @type {''} */ (''), zone: /** @type {''} */ ('') }
}

/**
 * Starts dragging an item from its grip (Multiventana only).
 * @param {ItemId} item @param {PointerEvent} event
 */
export function startDockDrag(item, event) {
  if (ui.editorMode !== 'multi' || event.button !== 0) return
  event.preventDefault()
  event.stopPropagation()
  dock.dragging = item
  dock.target = ''
  dock.zone = ''
  document.body.classList.add('resizing', 'docking')

  const onmove = (/** @type {PointerEvent} */ e) => {
    const h = hit(e.clientX, e.clientY)
    const self = h.target === item
    dock.target = self ? '' : h.target
    dock.zone = self ? '' : h.zone
  }
  const finish = (/** @type {boolean} */ apply) => {
    window.removeEventListener('pointermove', onmove)
    window.removeEventListener('pointerup', onup)
    window.removeEventListener('keydown', onkey, true)
    document.body.classList.remove('resizing', 'docking')
    if (apply && dock.target && dock.zone) setLayout(placeItem(panes.layout, item, dock.target, dock.zone))
    dock.dragging = ''
    dock.target = ''
    dock.zone = ''
  }
  const onup = () => finish(true)
  const onkey = (/** @type {KeyboardEvent} */ e) => {
    if (e.key !== 'Escape') return
    e.preventDefault()
    e.stopPropagation()
    finish(false)
  }
  window.addEventListener('pointermove', onmove)
  window.addEventListener('pointerup', onup)
  window.addEventListener('keydown', onkey, true)
}
