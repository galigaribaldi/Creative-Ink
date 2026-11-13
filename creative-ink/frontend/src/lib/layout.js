// Layout engine of the "Multiventana" mode (docs/MAP.md §4.4, #9, option A).
// The arrangement is a tree of splits; computeLayout() turns it into a
// rectangle per item (fractions of the dock area) and the dividers between
// siblings. The items (up to 4 editor panes and the PDF preview) are drawn
// once and only their rectangles change: moving one never remounts
// CodeMirror or PDF.js. Pure functions: no Svelte, no DOM.

/** @typedef {'p1' | 'p2' | 'p3' | 'p4'} PaneId */
/** @typedef {PaneId | 'pdf'} ItemId */
/**
 * @typedef {{ kind: 'leaf', item: ItemId }} Leaf
 * @typedef {{ kind: 'split', dir: 'row' | 'col', sizes: number[], children: LayoutNode[] }} Split
 * @typedef {Leaf | Split} LayoutNode
 * @typedef {{ x: number, y: number, w: number, h: number }} Rect  fractions of the dock area
 * @typedef {{ key: string, dir: 'row' | 'col', path: number[], index: number, pair: Rect, at: number }} Handle
 *   a divider between the visible children index and index+1 of the split at path; pair is
 *   the area of both; at is the divider's position (fraction of the dock, x for rows, y for cols)
 */

export const PANE_IDS = /** @type {PaneId[]} */ (['p1', 'p2', 'p3', 'p4'])
export const MAX_PANES = PANE_IDS.length

/** One editor and the preview side by side (the first-run arrangement). */
export function defaultLayout() {
  return /** @type {LayoutNode} */ ({
    kind: 'split',
    dir: 'row',
    sizes: [0.5, 0.5],
    children: [
      { kind: 'leaf', item: 'p1' },
      { kind: 'leaf', item: 'pdf' }
    ]
  })
}

/** Items of the tree, in order. @param {LayoutNode} node @returns {ItemId[]} */
export function itemsOf(node) {
  return node.kind === 'leaf' ? [node.item] : node.children.flatMap(itemsOf)
}

/** Editor panes of the tree, in order. @param {LayoutNode} node @returns {PaneId[]} */
export function panesOf(node) {
  return /** @type {PaneId[]} */ (itemsOf(node).filter((i) => i !== 'pdf'))
}

/**
 * Rectangles of the visible items and the dividers between visible
 * siblings. Hidden items (e.g. the preview with ⌥⌘P) give their space to
 * their siblings; the tree keeps their place for when they come back.
 * @param {LayoutNode} root
 * @param {Set<ItemId>} [hidden]
 */
export function computeLayout(root, hidden = new Set()) {
  /** @type {Partial<Record<ItemId, Rect>>} */
  const rects = {}
  /** @type {Handle[]} */
  const handles = []
  const visible = (/** @type {LayoutNode} */ n) => itemsOf(n).some((i) => !hidden.has(i))

  /** @param {LayoutNode} node @param {Rect} r @param {number[]} path */
  function walk(node, r, path) {
    if (node.kind === 'leaf') {
      if (!hidden.has(node.item)) rects[node.item] = r
      return
    }
    const shown = node.children.map((c, i) => ({ c, i })).filter(({ c }) => visible(c))
    const total = shown.reduce((sum, { i }) => sum + node.sizes[i], 0) || 1
    let offset = 0
    shown.forEach(({ c, i }, k) => {
      const share = node.sizes[i] / total
      const child =
        node.dir === 'row'
          ? { x: r.x + offset * r.w, y: r.y, w: share * r.w, h: r.h }
          : { x: r.x, y: r.y + offset * r.h, w: r.w, h: share * r.h }
      offset += share
      walk(c, child, [...path, i])
      const next = shown[k + 1]
      if (next) {
        const nextShare = node.sizes[next.i] / total
        const pairStart = offset - share
        const pair =
          node.dir === 'row'
            ? { x: r.x + pairStart * r.w, y: r.y, w: (share + nextShare) * r.w, h: r.h }
            : { x: r.x, y: r.y + pairStart * r.h, w: r.w, h: (share + nextShare) * r.h }
        handles.push({
          key: `${path.join('.')}:${i}`,
          dir: node.dir,
          path,
          index: i,
          pair,
          at: node.dir === 'row' ? r.x + offset * r.w : r.y + offset * r.h
        })
      }
    })
  }
  walk(root, { x: 0, y: 0, w: 1, h: 1 }, [])
  return { rects, handles }
}

/** Deep copy (the tree is plain data). @template T @param {T} node @returns {T} */
export function cloneLayout(node) {
  return JSON.parse(JSON.stringify(node))
}

/** The split at path. @param {LayoutNode} root @param {number[]} path */
function nodeAt(root, path) {
  let node = root
  for (const i of path) node = /** @type {Split} */ (node).children[i]
  return node
}

/**
 * New tree where the divider between children index and the next visible
 * one (other) of the split at path leaves `ratio` of their joint size to
 * the first.
 * @param {LayoutNode} root @param {number[]} path @param {number} index @param {number} other @param {number} ratio
 */
export function resize(root, path, index, other, ratio) {
  const next = cloneLayout(root)
  const split = /** @type {Split} */ (nodeAt(next, path))
  const total = split.sizes[index] + split.sizes[other]
  split.sizes[index] = total * ratio
  split.sizes[other] = total - split.sizes[index]
  return next
}

/** Index of the next visible sibling after index (for a handle). */
export function nextVisible(/** @type {LayoutNode} */ root, /** @type {number[]} */ path, /** @type {number} */ index, hidden = new Set()) {
  const split = /** @type {Split} */ (nodeAt(root, path))
  for (let i = index + 1; i < split.children.length; i++) {
    if (itemsOf(split.children[i]).some((it) => !hidden.has(it))) return i
  }
  return -1
}

/** Path of the leaf holding item ([] for a leaf root), or null. */
function pathOf(/** @type {LayoutNode} */ node, /** @type {ItemId} */ item, /** @type {number[]} */ path = []) {
  if (node.kind === 'leaf') return node.item === item ? path : null
  for (let i = 0; i < node.children.length; i++) {
    const found = pathOf(node.children[i], item, [...path, i])
    if (found) return found
  }
  return null
}

/**
 * New tree with item removed: its share goes to its siblings, and a split
 * left with one child is replaced by it.
 * @param {LayoutNode} root @param {ItemId} item
 */
export function removeItem(root, item) {
  const path = pathOf(root, item)
  if (!path || path.length === 0) return root // not there, or the only item
  const next = cloneLayout(root)
  const parentPath = path.slice(0, -1)
  const index = /** @type {number} */ (path.at(-1))
  const parent = /** @type {Split} */ (nodeAt(next, parentPath))
  parent.children.splice(index, 1)
  parent.sizes.splice(index, 1)
  const sum = parent.sizes.reduce((a, b) => a + b, 0) || 1
  parent.sizes = parent.sizes.map((s) => s / sum)
  if (parent.children.length === 1) {
    const only = parent.children[0]
    if (parentPath.length === 0) return only
    const grand = /** @type {Split} */ (nodeAt(next, parentPath.slice(0, -1)))
    grand.children[/** @type {number} */ (parentPath.at(-1))] = only
  }
  return next
}

/**
 * New tree with item placed on a side of target ('left', 'right', 'top',
 * 'bottom'), sharing target's space. 'center' swaps the two items.
 * @param {LayoutNode} root @param {ItemId} item @param {ItemId} target
 * @param {'left' | 'right' | 'top' | 'bottom' | 'center'} side
 */
export function placeItem(root, item, target, side) {
  if (item === target) return root
  if (side === 'center') {
    const next = cloneLayout(root)
    const a = pathOf(next, item)
    const b = pathOf(next, target)
    if (!a || !b) return root
    const leafA = /** @type {Leaf} */ (nodeAt(next, a))
    const leafB = /** @type {Leaf} */ (nodeAt(next, b))
    ;[leafA.item, leafB.item] = [leafB.item, leafA.item]
    return next
  }
  const next = cloneLayout(itemsOf(root).includes(item) ? removeItem(root, item) : root)
  const path = pathOf(next, target)
  if (!path) return root
  const dir = side === 'left' || side === 'right' ? 'row' : 'col'
  const before = side === 'left' || side === 'top'
  /** @type {Leaf} */
  const leaf = { kind: 'leaf', item }
  if (path.length > 0) {
    const parent = /** @type {Split} */ (nodeAt(next, path.slice(0, -1)))
    const index = /** @type {number} */ (path.at(-1))
    if (parent.dir === dir) {
      // Same direction: one more sibling, half of the target's share.
      const half = parent.sizes[index] / 2
      parent.sizes[index] = half
      parent.children.splice(before ? index : index + 1, 0, leaf)
      parent.sizes.splice(before ? index : index + 1, 0, half)
      return next
    }
    /** @type {Split} */
    const split = { kind: 'split', dir, sizes: [0.5, 0.5], children: before ? [leaf, parent.children[index]] : [parent.children[index], leaf] }
    parent.children[index] = split
    return next
  }
  // The target is the whole tree.
  return /** @type {Split} */ ({ kind: 'split', dir, sizes: [0.5, 0.5], children: before ? [leaf, next] : [next, leaf] })
}

/**
 * A stored tree, if it is well formed: splits with 2+ children and positive
 * sizes, every item at most once, the preview exactly once and 1–4 panes.
 * Anything else (an old version, a hand-edited value) → null.
 * @param {unknown} value @returns {LayoutNode | null}
 */
export function validLayout(value) {
  const seen = new Set()
  /** @param {any} n @returns {boolean} */
  const ok = (n) => {
    if (!n || typeof n !== 'object') return false
    if (n.kind === 'leaf') {
      if (!(n.item === 'pdf' || PANE_IDS.includes(n.item)) || seen.has(n.item)) return false
      seen.add(n.item)
      return true
    }
    return (
      n.kind === 'split' &&
      (n.dir === 'row' || n.dir === 'col') &&
      Array.isArray(n.children) &&
      n.children.length >= 2 &&
      Array.isArray(n.sizes) &&
      n.sizes.length === n.children.length &&
      n.sizes.every((/** @type {unknown} */ s) => typeof s === 'number' && s > 0) &&
      n.children.every(ok)
    )
  }
  if (!ok(value)) return null
  const panes = [...seen].filter((i) => i !== 'pdf').length
  return seen.has('pdf') && panes >= 1 && panes <= MAX_PANES ? /** @type {LayoutNode} */ (value) : null
}
