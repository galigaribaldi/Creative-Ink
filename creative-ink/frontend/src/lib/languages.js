// Syntax highlighting per file type for CodeMirror. Only legacy stream modes
// already installed (@codemirror/legacy-modes) plus a small BibTeX mode:
// no new dependencies. Files without a mode (.txt, .md, .csv) are plain text.
import { StreamLanguage } from '@codemirror/language'
import { lua } from '@codemirror/legacy-modes/mode/lua'
import { python } from '@codemirror/legacy-modes/mode/python'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { stex } from '@codemirror/legacy-modes/mode/stex'
import { yaml } from '@codemirror/legacy-modes/mode/yaml'

/**
 * BibTeX: `@type{key, field = {value} # "value", …}`. Text outside entries
 * is a comment for BibTeX; `%` lines are comments for biber and editors.
 * State: depth 0 = between entries, 1 = inside an entry, ≥ 2 = inside a
 * braced value; `quote` = inside a "…" value (braces nest inside it).
 */
const bibtexMode = {
  name: 'bibtex',
  startState: () => ({ depth: 0, quote: false, quoteDepth: 0, expectKey: false, afterType: false }),
  copyState: (s) => ({ ...s }),
  token(stream, state) {
    if (state.quote) return quotedValue(stream, state)
    if (state.depth >= 2) return bracedValue(stream, state)
    if (stream.eatSpace()) return null

    if (state.depth === 0) {
      if (stream.match(/^@[A-Za-z]+/)) {
        state.afterType = true
        return 'keyword'
      }
      if (state.afterType && (stream.eat('{') || stream.eat('('))) {
        state.afterType = false
        state.depth = 1
        state.expectKey = true
        return 'bracket'
      }
      state.afterType = false
      if (stream.peek() === '%') {
        stream.skipToEnd()
        return 'comment'
      }
      if (!stream.skipTo('@')) stream.skipToEnd()
      return 'comment'
    }

    // depth 1: key, fields, separators.
    if (state.expectKey && stream.match(/^[^,\s{}()]+/)) {
      state.expectKey = false
      return 'def'
    }
    const ch = stream.next()
    if (ch === ',') {
      state.expectKey = false
      return null
    }
    if (ch === '}' || ch === ')') {
      state.depth = 0
      return 'bracket'
    }
    if (ch === '=' || ch === '#') return 'operator'
    if (ch === '{') {
      state.depth = 2
      return bracedValue(stream, state)
    }
    if (ch === '"') {
      state.quote = true
      state.quoteDepth = 0
      return quotedValue(stream, state)
    }
    if (/\d/.test(ch)) {
      stream.eatWhile(/\d/)
      return 'number'
    }
    if (/[A-Za-z]/.test(ch)) {
      stream.eatWhile(/[\w:.-]/)
      return 'attribute'
    }
    return null
  },
  languageData: { commentTokens: { line: '%' } }
}

function bracedValue(stream, state) {
  let ch
  while ((ch = stream.next()) != null) {
    if (ch === '\\') stream.next()
    else if (ch === '{') state.depth++
    else if (ch === '}' && --state.depth === 1) break
  }
  return 'string'
}

function quotedValue(stream, state) {
  let ch
  while ((ch = stream.next()) != null) {
    if (ch === '\\') stream.next()
    else if (ch === '{') state.quoteDepth++
    else if (ch === '}') state.quoteDepth = Math.max(0, state.quoteDepth - 1)
    else if (ch === '"' && state.quoteDepth === 0) {
      state.quote = false
      break
    }
  }
  return 'string'
}

const TEX = StreamLanguage.define(stex)
const LANGUAGES = {
  tex: TEX,
  bib: StreamLanguage.define(bibtexMode),
  lua: StreamLanguage.define(lua),
  python: StreamLanguage.define(python),
  shell: StreamLanguage.define(shell),
  yaml: StreamLanguage.define(yaml)
}

/** Extensions (lower case, without the dot) → language key. */
const BY_EXTENSION = {
  tex: 'tex', ltx: 'tex', sty: 'tex', cls: 'tex', dtx: 'tex', ins: 'tex',
  def: 'tex', cfg: 'tex', clo: 'tex', fd: 'tex', bbx: 'tex', cbx: 'tex', lbx: 'tex',
  bib: 'bib',
  lua: 'lua',
  py: 'python',
  sh: 'shell',
  yaml: 'yaml',
  yml: 'yaml'
}

/**
 * CodeMirror language extension for a file, or [] for plain text.
 * @param {string} path
 */
export function languageFor(path) {
  const ext = /\.([^./\\]+)$/.exec(path)?.[1]?.toLowerCase() ?? ''
  const key = BY_EXTENSION[ext]
  return key ? LANGUAGES[key] : []
}
