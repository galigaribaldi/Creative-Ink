// CodeMirror theme built ONLY from the design tokens of style.css
// (.claude/skills/ui-svelte-architect/SKILL.md, A2 and A3). Every value is a
// CSS variable, so one theme serves light and dark: switching
// <html data-theme> repaints the editor without reconfiguring or
// remounting it. It replaces @codemirror/theme-one-dark (dark-only).
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { EditorView } from '@codemirror/view'
import { tags as t } from '@lezer/highlight'

const chrome = EditorView.theme({
  '&': {
    height: '100%',
    color: 'var(--text-primary)',
    backgroundColor: 'var(--surface-editor)',
    fontSize: 'var(--text-editor)'
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': {
    fontFamily: 'var(--font-mono)',
    lineHeight: 'var(--line-editor)',
    // `--`, `''` and `<<` must read as typed in LaTeX.
    fontVariantLigatures: 'none'
  },
  // The text canvas is king (A1): generous inner padding.
  '.cm-content': { padding: '16px', caretColor: 'var(--text-primary)' },
  '.cm-line': { padding: '0' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--text-primary)' },

  // Selection: never the accent (A2).
  '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection':
    { backgroundColor: 'var(--selection)' },
  '.cm-selectionMatch': { backgroundColor: 'var(--surface-active)' },
  '.cm-activeLine': { backgroundColor: 'var(--surface-hover)' },
  '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': {
    backgroundColor: 'var(--surface-active)',
    outline: '1px solid var(--border-strong)'
  },
  '.cm-nonmatchingBracket, &.cm-focused .cm-nonmatchingBracket': { color: 'var(--danger)' },
  '.cm-specialChar': { color: 'var(--danger)' },

  // Gutter: no border, muted numbers, the active line a step brighter.
  '.cm-gutters': {
    backgroundColor: 'var(--surface-editor)',
    color: 'var(--text-muted)',
    border: 'none'
  },
  '.cm-lineNumbers .cm-gutterElement': { padding: '0 8px 0 12px' },
  '.cm-activeLineGutter': { backgroundColor: 'transparent', color: 'var(--text-secondary)' },

  // Search panel (⌘F) and "go to line" (⌥⌘G).
  '.cm-panels': {
    backgroundColor: 'var(--surface-base)',
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-ui)',
    fontSize: 'var(--text-sm)'
  },
  '.cm-panels.cm-panels-top': { borderBottom: '1px solid var(--border)' },
  '.cm-panels.cm-panels-bottom': { borderTop: '1px solid var(--border)' },
  '.cm-panel label': { color: 'var(--text-secondary)', fontSize: 'var(--text-sm)' },
  '.cm-textfield': {
    padding: '2px 6px',
    border: '1px solid var(--border-strong)',
    borderRadius: 'var(--radius-sm)',
    backgroundColor: 'var(--surface-editor)',
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-ui)',
    fontSize: 'var(--text-sm)'
  },
  '.cm-textfield:focus-visible': { outline: '2px solid var(--accent)', outlineOffset: '1px' },
  '.cm-button': {
    padding: '2px 8px',
    border: '1px solid var(--border-strong)',
    borderRadius: 'var(--radius-sm)',
    backgroundImage: 'none',
    backgroundColor: 'transparent',
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-ui)',
    fontSize: 'var(--text-sm)',
    transition: 'background-color var(--transition)'
  },
  '.cm-button:hover': { backgroundColor: 'var(--surface-hover)' },
  '.cm-button:active': { backgroundImage: 'none', backgroundColor: 'var(--surface-active)' },
  '.cm-panel.cm-search [name=close]': {
    color: 'var(--text-secondary)',
    borderRadius: 'var(--radius-sm)',
    transition: 'color var(--transition)'
  },
  '.cm-panel.cm-search [name=close]:hover': { color: 'var(--text-primary)' },
  '.cm-searchMatch': {
    backgroundColor: 'var(--selection)',
    outline: '1px solid var(--border-strong)'
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'var(--selection)',
    outline: '1px solid var(--text-secondary)'
  },

  // Floating UI (tooltips, if any extension adds them).
  '.cm-tooltip': {
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-md)',
    backgroundColor: 'var(--surface-raised)',
    color: 'var(--text-primary)',
    boxShadow: 'var(--shadow-overlay)'
  }
})

// Token types of the legacy modes (lib/languages.js) → tags → --syntax-*.
//  stex: tag = \command, atom = environment/package/label argument,
//        keyword = math delimiters, bracket, number, comment, error.
//  BibTeX: keyword = @type, def = citation key, attribute = field,
//          string = value, number, operator, comment, bracket.
const latexHighlight = HighlightStyle.define([
  { tag: [t.tagName, t.attributeName, t.propertyName, t.standard(t.variableName)], color: 'var(--syntax-command)' },
  { tag: [t.bracket, t.brace, t.squareBracket, t.paren, t.operator], color: 'var(--syntax-bracket)' },
  { tag: [t.atom, t.string, t.special(t.string), t.definition(t.variableName)], color: 'var(--syntax-string)' },
  { tag: [t.number, t.bool], color: 'var(--syntax-number)' },
  { tag: [t.keyword, t.modifier], color: 'var(--syntax-math)' },
  { tag: [t.comment, t.lineComment, t.blockComment], color: 'var(--syntax-comment)', fontStyle: 'italic' },
  { tag: t.heading, color: 'var(--text-primary)', fontWeight: '600' },
  { tag: t.meta, color: 'var(--text-muted)' },
  { tag: t.invalid, color: 'var(--danger)' }
])

/** The editor's whole look: chrome + syntax colours, all from tokens. */
export const editorTheme = [chrome, syntaxHighlighting(latexHighlight)]
