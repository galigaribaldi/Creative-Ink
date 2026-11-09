// Light / dark theme (.claude/skills/ui-svelte-architect/SKILL.md, A2).
//
// The preference ('system' | 'light' | 'dark') is purely visual: it lives in
// localStorage and never goes to Go. The resolved theme is written to
// <html data-theme="…">, which switches every CSS token in style.css.
// Components never read it to pick colours; only code that paints outside
// CSS (e.g. a canvas) may need `theme.resolved`.

/** @typedef {'system' | 'light' | 'dark'} ThemePreference */
/** @typedef {'light' | 'dark'} ResolvedTheme */

const STORAGE_KEY = 'creative-ink.theme'
/** @type {ThemePreference[]} */
const PREFERENCES = ['system', 'light', 'dark']

const darkQuery = window.matchMedia('(prefers-color-scheme: dark)')

export const theme = $state({
  /** @type {ThemePreference} */
  preference: readPreference(),
  /** @type {ResolvedTheme} */
  resolved: 'dark'
})

/** @returns {ThemePreference} */
function readPreference() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (PREFERENCES.includes(/** @type {ThemePreference} */ (saved))) return /** @type {ThemePreference} */ (saved)
  } catch {
    // Storage unavailable: follow the system.
  }
  return 'system'
}

/** @param {ThemePreference} preference @returns {ResolvedTheme} */
function resolve(preference) {
  if (preference === 'system') return darkQuery.matches ? 'dark' : 'light'
  return preference
}

function apply() {
  theme.resolved = resolve(theme.preference)
  document.documentElement.dataset.theme = theme.resolved
}

let initialized = false

/**
 * Applies the saved theme and follows system changes while the preference is
 * 'system'. Call it once, in main.js, BEFORE mount(): the first frame is then
 * painted with the right tokens and nothing flashes.
 */
export function initTheme() {
  if (initialized) return
  initialized = true
  apply()
  darkQuery.addEventListener('change', () => {
    if (theme.preference === 'system') apply()
  })
}

/**
 * Changes and remembers the preference (e.g. from a future settings menu).
 * @param {ThemePreference} preference
 */
export function setThemePreference(preference) {
  if (!PREFERENCES.includes(preference)) return
  theme.preference = preference
  try {
    localStorage.setItem(STORAGE_KEY, preference)
  } catch {
    // Not persisted; it still applies for this session.
  }
  apply()
}
