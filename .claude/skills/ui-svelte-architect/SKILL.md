---
name: ui-svelte-architect
description: Design bible and Svelte 5 architecture for the Creative-ink UI. Covers the visual system (functional minimalism in the style of Typora/Obsidian, light and dark themes, CSS variable tokens, emerald accent rules, typography, thin-stroke icons, 0.15s transitions) and the structure of components and layouts (Action Bar, Sidebar, Workspace with Editor and PDF Preview, panels, modals), plus frontend state that never overloads the Wails IPC bridge. Use for any Svelte UI/UX work: visual design, colors, themes, styling, layout, new panels or components, frontend state (open files, split views, visibility) or frontend performance (debouncing, re-renders, jank).
---

# UI Svelte Architect: Creative-ink design bible

You are the UI/UX architect of Creative-ink, a native LaTeX editor that runs inside Wails, and you work in Svelte 5.
Goal: an editor-first interface that is fast, light and calm. It must stay out of the way while the user writes,
and it must never push purely visual state through the Go ↔ JS bridge.

This skill has two layers, and both are mandatory:
- **Part A (Design system):** how things look and feel.
- **Part B (Svelte architecture):** how things are built.

Read first: `CLAUDE.md` (rules, gotchas, status) and `docs/MAP.md` (layers, IPC contract).
Those files win over anything in this skill if they disagree.

---

# Part A: Design system

## A1. Visual philosophy: functional minimalism

Inspired by **Typora** and **Obsidian**.

1. **The text canvas is king.** The editor and the PDF page get the space, the contrast and the attention.
   Every other element (bars, trees, panels) is chrome. Chrome is lower in contrast and quieter,
   and it only gets louder while the user interacts with it.
2. **The interface disappears while the user types.**
   - Nothing moves or animates, and the layout never changes, in response to typing.
   - Status text changes inside fixed-width slots, so no layout shift happens.
   - Hover-only affordances stay hidden until hover or focus (e.g. the ☆ main-file toggle).
   - Nothing interrupts the user: no modal for routine events, and errors never steal focus. A failed compile
     marks its tab with a dot; it does not pop a dialog.
3. **Light and native.** No aggressive borders and no heavy shadows. Separate zones by a change of surface
   colour and by spacing first; use a 1px hairline (`--border`) only where a separation is still unclear.
   Shadows are reserved for things that float (menus, modals) and for the PDF sheet, and they are always soft.
4. **Quiet success, visible problems.** A successful state needs no colour (a neutral ✓ or plain text).
   Only warnings and errors use colour (`--warning`, `--danger`).
5. **Every pixel justifies itself.** Prefer a thin Action Bar and retractable drawers over permanent panels.
   Before adding a panel, a badge or a toolbar button, ask whether it can be hidden, merged with something
   else, or shown only when relevant.

## A2. Colour system (CSS variables)

**Always design for both the light and the dark theme.** A component is not done until it has been checked in both.
- Every colour comes from a token. Never write a hex or rgb value inside a component.
- Tokens live in `frontend/src/style.css`.
- The theme is the `data-theme` attribute on `<html>` (`"light"` | `"dark"`).

### Tokens

```css
/* frontend/src/style.css: theme-independent tokens */
:root {
  --font-ui: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;
  --font-mono: ui-monospace, 'SF Mono', 'Cascadia Mono', 'JetBrains Mono', Menlo, Consolas, 'DejaVu Sans Mono',
    'Liberation Mono', monospace;
  --text-xs: 11px;      /* badges, meta lines */
  --text-sm: 12px;      /* status bars, tree, tabs */
  --text-md: 13px;      /* UI body, dialogs */
  --text-lg: 15px;      /* dialog titles */
  --text-editor: 14px;  /* CodeMirror */
  --line-editor: 1.6;

  --radius-sm: 4px;     /* buttons, inputs, tree rows, badges */
  --radius-md: 6px;     /* cards, menus, modals, the preview sheet */

  --duration: 0.15s;
  --ease: ease;
  --transition: var(--duration) var(--ease);

  --icon-size: 18px;
  --icon-stroke: 1.5;
}

/* Dark theme */
:root[data-theme='dark'] {
  color-scheme: dark;
  --surface-base: #181818;    /* app background, ActionBar, Sidebar, status bars */
  --surface-editor: #202020;  /* CodeMirror, terminal, preview viewer */
  --surface-raised: #262626;  /* menus, modals, popovers */
  --surface-hover: rgb(255 255 255 / 0.05);
  --surface-active: rgb(255 255 255 / 0.08); /* pressed or selected without accent */
  --border: rgb(255 255 255 / 0.07);         /* hairlines */
  --border-strong: rgb(255 255 255 / 0.14);  /* inputs */

  --text-primary: #e0e0e0;
  --text-secondary: #a0a0a0;
  --text-muted: #858585;      /* ≥ 4.5:1 on --surface-base */
  --text-disabled: #5a5a5a;

  --accent: #50c878;          /* emerald */
  --accent-hover: #66d48b;
  --accent-contrast: #0b1f14; /* text/icons ON the accent (white fails contrast here) */
  --accent-soft: rgb(80 200 120 / 0.14);

  --warning: #e5b454;
  --danger: #f07178;
  --selection: rgb(130 170 255 / 0.25);      /* text selection: never the accent */

  --shadow-overlay: 0 4px 16px rgb(0 0 0 / 0.35);
  --shadow-page: 0 1px 4px rgb(0 0 0 / 0.45); /* the PDF sheet */
  --scrim: rgb(0 0 0 / 0.5);                  /* modal backdrop */

  /* LaTeX syntax (CodeMirror): calm, and never the accent */
  --syntax-command: #82aaff;  /* \section, \begin */
  --syntax-bracket: #c792ea;  /* { } [ ] */
  --syntax-string: #e0c08c;   /* arguments, BibTeX values */
  --syntax-number: #f78c6c;
  --syntax-math: #89ddff;     /* $…$ */
  --syntax-comment: #8a8a8a;  /* % … (italic) */
}

/* Light theme */
:root[data-theme='light'] {
  color-scheme: light;
  --surface-base: #f9f9f9;    /* ice grey */
  --surface-editor: #ffffff;
  --surface-raised: #ffffff;  /* + --border, since white on white needs an edge */
  --surface-hover: rgb(0 0 0 / 0.04);
  --surface-active: rgb(0 0 0 / 0.06);
  --border: rgb(0 0 0 / 0.08);
  --border-strong: rgb(0 0 0 / 0.16);

  --text-primary: #111111;
  --text-secondary: #555555;
  --text-muted: #6b6b6b;
  --text-disabled: #a8a8a8;

  --accent: #1f7a4d;          /* deep emerald: 5.3:1 on white */
  --accent-hover: #19663f;
  --accent-contrast: #ffffff;
  --accent-soft: rgb(31 122 77 / 0.1);

  --warning: #9a6700;
  --danger: #c62828;
  --selection: rgb(31 95 191 / 0.18);

  --shadow-overlay: 0 4px 16px rgb(0 0 0 / 0.08);
  --shadow-page: 0 1px 3px rgb(0 0 0 / 0.12);
  --scrim: rgb(0 0 0 / 0.25);

  --syntax-command: #1f5fbf;
  --syntax-bracket: #7b3fb5;
  --syntax-string: #8a5a00;
  --syntax-number: #b5490f;
  --syntax-math: #0b7285;
  --syntax-comment: #757575;
}
```

**Alternative accent: royal blue.** Switching is a token change only, never a component change:
- dark: `--accent: #7c95ff`, `--accent-hover: #93a8ff`, `--accent-contrast: #0b1433`, `--accent-soft: rgb(124 149 255 / 0.16)`;
- light: `--accent: #2747c9`, `--accent-hover: #1f3aa8`, `--accent-contrast: #ffffff`, `--accent-soft: rgb(39 71 201 / 0.1)`.

**Legacy aliases.** These keep existing components working during the migration (A6):
`--bg: var(--surface-base); --fg: var(--text-primary);`. The current `--accent` name is kept.

### Where the accent may appear (and only there)

The accent is a signal. When it appears everywhere, it stops meaning anything. Allowed uses:

1. **Primary action buttons:** the one main action of a screen or dialog (e.g. "Nuevo proyecto", "Crear").
   They are filled with `--accent` and their text is `--accent-contrast`. Secondary actions are neutral
   (transparent background, `--border-strong`, `--text-primary`).
2. **The active item in the file tree:**
   - the open file: `--accent-soft` background, `--text-primary` text, a 2px `--accent` inset mark on the left;
   - in the split editor, the focused pane, with the same thin mark.
3. **The SyncTeX highlight:** a box in the PDF with an `--accent` border, an `--accent-soft` fill and a fade-out.
4. **Exception, accessibility:** the keyboard focus ring (`:focus-visible { outline: 2px solid var(--accent);
   outline-offset: 1px }`). It only appears with the keyboard, never on mouse clicks.

Not allowed:
- links (use `--text-secondary`, underlined on hover);
- the active tab or tool in the ActionBar (use `--text-primary` + `--surface-active`);
- text selection (use `--selection`);
- success states (neutral);
- syntax colours;
- decorative borders.

### Contrast
- Body text: at least **4.5:1** against its surface. Large text, icons and UI boundaries: at least **3:1**.
- The values above already pass. When you add a token, check it with a contrast calculator in both themes.
- Never place text on the accent without `--accent-contrast`. White on dark-theme emerald fails (about 2:1).

### Theme switching
- `lib/theme.svelte.js` resolves the theme. It reads the preference `'system' | 'light' | 'dark'`, stored in
  `localStorage` (it is purely visual, so it never goes to Go). For `'system'` it follows
  `matchMedia('(prefers-color-scheme: dark)')`, including live changes. It then sets `document.documentElement.dataset.theme`.
- Apply the theme **in `main.js`, before `mount()`**, so the first frame is already painted with the right
  theme and nothing flashes.
- Components never read the theme to choose colours: the tokens already did. The only JS consumers are
  CodeMirror (A3) and anything painted on a canvas.

## A3. Components and typography

- **UI typography:** `font-family: var(--font-ui)`. It renders in the system's own face (SF on macOS,
  Segoe UI on Windows, Ubuntu/Cantarell on Linux), so the app feels native.
  - Sizes come from the `--text-*` scale only.
  - Weights: 400 for text, 500 for titles and labels, 600 only for the dialog title. Never 700+ in chrome.
- **Editor typography (CodeMirror):**
  - `font-family: var(--font-mono)`, `font-size: var(--text-editor)`, `line-height: var(--line-editor)`;
  - `font-variant-ligatures: none`, so `--`, `''` and `<<` read as typed, which matters in LaTeX;
  - generous inner padding (`.cm-content`: 16px or more on the sides; it grows with the pane width);
  - the gutter has no border and uses muted numbers (`--text-muted`), with `--text-secondary` on the active line;
  - the active line is barely visible (`--surface-hover`); the selection is `--selection`;
  - the editor theme is built from the tokens: `EditorView.theme({...})` with `var(--…)` values, plus a
    `HighlightStyle` mapped to the `--syntax-*` tokens. Since it uses variables, **one theme serves both modes**.
    That theme replaces `@codemirror/theme-one-dark`, which is dark-only. `HighlightStyle` needs `tags` from
    `@lezer/highlight`, which today is only a transitive dependency: ask the user to run
    `npm install @lezer/highlight` in `creative-ink/frontend/` before importing it.
- **Shapes:**
  - `--radius-sm` (4px) for buttons, inputs, tree rows, badges and tabs;
  - `--radius-md` (6px) for cards, menus, modals and the preview sheet;
  - never more than 6px; no pills, except for count badges.
- **Borders:** 1px `--border`, used sparingly. Inputs use `--border-strong` and, on keyboard focus, the focus
  ring. There are no borders between ActionBar, Sidebar and Workspace: the change of surface is enough
  (`--surface-base` against `--surface-editor`).
- **Shadows:** only `--shadow-overlay` (menus, modals, popovers) and `--shadow-page` (the PDF sheet).
  Panels, buttons and cards have none.
- **Buttons:**
  - minimum hit area 28 × 28px (32px in the ActionBar);
  - icon buttons are transparent at rest, `--surface-hover` on hover and `--surface-active` while pressed or active;
  - disabled: `--text-disabled`, no hover and `cursor: default`.
- **Lists and the tree:**
  - rows are 22–24px high with `--text-sm` and indent by depth;
  - hover is `--surface-hover`, and the active row follows A2 rule 2;
  - folder chevrons rotate on expand (transition A4);
  - file-type badges use muted colours, never the accent.
- **Status bars:** `--surface-base`, `--text-sm`, `--text-muted`. Warnings use `--warning`. Text truncates with an
  ellipsis and never wraps. Each item keeps a stable width so typing never shifts them.
- **Dialogs and modals:**
  - `--surface-raised`, `--radius-md`, `--shadow-overlay`, over a `--scrim` backdrop;
  - one primary action, and Esc closes;
  - the title is `--text-lg`/600 and the body `--text-md`.
- **Icons:** thin stroke in the **Lucide / Feather** style:
  - `viewBox="0 0 24 24"`, `fill="none"`, `stroke="currentColor"`, `stroke-width="var(--icon-stroke)"` (1.5),
    `stroke-linecap="round"`, `stroke-linejoin="round"`;
  - rendered at `--icon-size` (18px);
  - colour `--text-secondary` at rest and `--text-primary` on hover or when active;
  - inline SVG only, no icon font. Do not install `lucide-*` packages without the user's decision. If you
    copy Lucide paths (ISC licence) or Feather paths (MIT), keep the licence notice in a comment where they are used.

## A4. Motion

- **Every interactive change animates with `0.15s ease` (`var(--transition)`):** hover, press, focus, opening
  and closing menus and drawers, switching tabs, expanding tree folders, showing hover affordances.
- List the properties explicitly. **Never `transition: all`**: it animates layout by accident and costs frames.
  ```css
  .button {
    transition: background-color var(--transition), color var(--transition),
      border-color var(--transition), opacity var(--transition);
  }
  ```
- **Menus and popovers:** opacity 0 → 1 and `translateY(2px)` → 0. Avoid `transform: scale` on text, because it blurs it.
- **Drawers:** the shell animates `--sidebar-w` / `grid-template-columns` over `var(--duration)`. The Preview's
  `ResizeObserver` is debounced (150 ms), so the PDF re-renders once after the drawer settles.
- **Never animate:** anything that depends on typing (A1 rule 2); the editor's text; the PDF canvas size frame
  by frame; layout properties of large areas (`width` or `height` of panels) outside the drawer.
- **Continuous progress** (e.g. the terminal's build bar) may use a linear transition on `width`. It is
  feedback, not an interaction.
- **Respect `prefers-reduced-motion: reduce`:** set the durations to 0. Define `--duration: 0s` in a media
  query in `style.css`, so every component follows the user's setting automatically.

## A5. Design review checklist (run it before saying a UI change is done)

1. Checked in **both themes** (in the real app, with the system appearance switched, or with `data-theme` forced in devtools).
2. **No hard-coded colours, sizes or durations**: only tokens. In the component, a search for `#[0-9a-f]` finds nothing.
3. The **accent** appears only in the places allowed by A2.
4. Text contrast is ≥ 4.5:1 and icons/boundaries ≥ 3:1 in both themes.
5. Every hover/active/open/tab change has a `var(--transition)` on explicit properties; nothing animates while typing.
6. Icons are 24-unit, 1.5 stroke, round, `currentColor`, 18px.
7. No new borders or shadows beyond A3. Separation comes from surfaces and spacing.
8. The editor canvas did not lose space or contrast.
9. Keyboard: focus ring visible, Esc closes, icon buttons have `aria-label` + `title`.

## A6. Current state of the code (visual debt, as of 2026-11-08)

The app predates this design bible. **Do not restyle everything in one go**, and only change the app when the
user asks for UI work. When a task touches a component, migrate *that* component to the tokens in the same change.
- `style.css` only defines `--bg #1e1e1e`, `--fg #d4d4d4` and `--accent #3b82f6` (blue), with `color-scheme: dark`.
  The full token set of A2, `lib/theme.svelte.js` and the light theme do not exist yet.
- The components hard-code more than 150 colour values: `#fbbf24`, `#9d9d9d`, `#6b6b6b`, `#f87171`, `#4ade80`, `#3c3c3c`, `#252526`…
- CodeMirror uses `@codemirror/theme-one-dark`, which is dark-only.
- The SyncTeX highlight is amber (it should be the accent). Successes are green (they should be neutral). Some
  links use the accent (they should be `--text-secondary`).
- Transitions: 100 ms (tree chevron), 120 ms (drawer), 200 ms (terminal bar), and none on most hovers.
- `Preview.svelte` uses a heavy page shadow (`0 2px 12px rgb(0 0 0 / 0.5)`). It should be `--shadow-page`.
- `main.go` sets the Wails window `BackgroundColour` to a fixed dark grey (30, 30, 30), so the light theme
  would flash dark at startup. Fixing that is a Go change (`go-wails-ipc-bridge`): match the background to the
  theme, or use `options.Mac.Appearance`/WebView transparency.

Suggested migration order when the user asks for it:
1. tokens + `lib/theme.svelte.js` + legacy aliases;
2. shell (ActionBar, Sidebar, status bars);
3. CodeMirror theme;
4. Preview (sheet, SyncTeX highlight);
5. dialogs (StartScreen, RepositorySetup, HealthPanel);
6. terminal and bottom panel.

---

# Part B: Svelte architecture

## B1. Project context (verify before relying on it)

- Stack: **Svelte 5 with runes** (`$state`, `$derived`, `$effect`, `$props`) + Vite 8, plain JavaScript
  with JSDoc types. Mount with `mount()`, events with `onclick` (not `on:click`).
- Styling: **plain CSS, scoped per component**, consuming the global tokens of A2 from `frontend/src/style.css`.
  Do not introduce Tailwind or any UI library unless the user decides it.
- Current layout (`frontend/src/`), already migrated to the target layout below:
  - `App.svelte`: CSS Grid shell `[ActionBar | Sidebar | Workspace]`, `HealthPanel` modal, global shortcuts.
  - `components/ActionBar.svelte`, `Sidebar.svelte` (always mounted, `inert` when closed), `Workspace.svelte`.
  - `components/Editor.svelte`: CodeMirror 6 + autosave debounce, one instance per pane.
    - `pane="primary"` reacts to `workspace.currentFilePath`, `pane="secondary"` to `workspace.secondaryFilePath`
      (split editor).
    - It saves the old file, then calls `setState` with the new one.
    - It registers its commands in `lib/actions.js`.
    - It only schedules automatic compiles when `config.autoCompile` is on.
  - `components/FileTreeItem.svelte`: recursive explorer node. Text files are openable (`.tex`, `.bib`…);
    only `.tex` can be the main file.
  - `components/Preview.svelte`: PDF.js on `<canvas>`, driven by `pdfVersion`; keeps `currentPageNumber`.
    Pages are painted in PDF.js' renderer worker (`lib/pdf.js`); never set `disableWorkerRendering`.
  - `components/StartScreen.svelte`, `RepositorySetup.svelte`, `HealthPanel.svelte`, `BottomPanel.svelte`
    (`CompileLog` | `TerminalPanel`).
  - State modules:
    - `lib/ui.svelte.js`: visual state;
    - `lib/workspace.svelte.js`: mirror of Go (folder, the file of each pane, main file, tree, project warnings).
      `compileTarget()` = the main file or the last `.tex` of the main pane; it is the Preview's key and the
      filter for `compile:*` events;
    - `lib/compilation.svelte.js`: `compile:*` events, filtered by `file`;
    - `lib/config.svelte.js`: mirror of the config service;
    - `lib/terminal.svelte.js`, `lib/synctex.svelte.js`;
    - `lib/actions.js`: global actions and `runCommand()`.
  - `lib/pdf.js`: PDF.js workers and document lifecycle (`loadPdf`, `releasePdf`).
  - `lib/languages.js`: syntax highlighting per file type.
- Backend calls only through `frontend/wailsjs/go/main/App.js` (generated; never edit by hand).

## B2. Principles

1. **Minimalist & focused.** Editor first (see A1). Every new panel must justify its screen space.
2. **UI state lives in the frontend.** Sidebar open/closed, active tab, split ratio, current PDF page, zoom
   and **theme** are Svelte state. Persist preferences in `localStorage` if needed, never in Go.
3. **Backend state goes through the contract.** Document content, files on disk, settings and compilation
   results come from the Go bindings and events listed in `docs/MAP.md` §3. Svelte keeps no critical state.
4. **Modular components.** One responsibility per component. Layout components (ActionBar, Sidebar,
   Workspace) contain no business logic; feature components (Editor, Preview) contain no layout.
5. **Modern, lightweight CSS.** CSS Grid for the shell, Flexbox inside panels, `min-width: 0` /
   `min-height: 0` on flex and grid children so CodeMirror and the canvas can shrink. Tokens for every value (A2).

## B3. Target layout

```
App.svelte                  grid: [ActionBar | Sidebar? | Workspace], provides UI context
├── ActionBar.svelte        thin vertical icon bar (~44px), toggles drawers and opens modals
├── Sidebar.svelte          retractable drawer (file tree, outline…); content per active tool
└── Workspace.svelte        Editor (| Editor) | Preview (+ BottomPanel under the editors)
    ├── Editor.svelte       ×2 when split
    └── Preview.svelte
```

New drawers add a `SidebarTool` id in `lib/ui.svelte.js`, a button in `ActionBar` and a branch in
`Sidebar`. Never move the primary `Editor` or the `Preview` out of `Workspace`, and never wrap them in `{#if}`.

## B4. Steps for architecting a layout

1. **Analyse the requirements.** List the zones, what each shows, and which data each needs.
   For each piece of data, decide whether it is *UI state* (frontend) or *backend data* (IPC).
2. **Define the state.**
   - Global UI state goes in a rune module, e.g. `src/lib/ui.svelte.js`:
     ```js
     export const ui = $state({ sidebarOpen: false, activeTool: 'files', splitRatio: 0.5 })
     ```
     Share it through `setContext`/`getContext` (or by importing the module) instead of prop drilling.
   - Derived values use `$derived`. Side effects (IPC, DOM APIs) use `$effect`, with `untrack` for the parts
     that must not become dependencies.
   - Classic `svelte/store` (`writable`, `derived`) is still valid, but prefer runes for new code.
3. **Structure the components** as in the target layout. Pass callbacks as props
   (`onsaved`, `onclose`), not `createEventDispatcher` (that is Svelte 4).
4. **Design the visuals with Part A.** Surfaces, tokens, the accent only where allowed, `--radius-*`,
   thin icons and `var(--transition)`, in both themes. Example shell:
   ```css
   .shell {
     display: grid;
     grid-template-columns: 44px var(--sidebar-w, 0px) 1fr;
     height: 100%;
     background: var(--surface-base);
     transition: grid-template-columns var(--transition);
   }
   ```
   Animate the drawer by changing `--sidebar-w`, or overlay it with `transform` if it must not resize the
   workspace at all.
5. **Verify.**
   - Run the checker (B5) and fix every error and warning.
   - Go through the design checklist (A5).
   - Then tell the user to restart `wails dev` (you must not run it) and describe exactly what they should see,
     **in both themes**.

## B5. Verification

From the repo root:

```bash
node .claude/skills/ui-svelte-architect/scripts/check-svelte.mjs
```

The checker:
- compiles every `.svelte` component and `.svelte.js` rune module under `creative-ink/frontend/src` with the
  project's own Svelte compiler, and prints errors and warnings;
- changes nothing and needs no install;
- does not check the visuals: A5 does;
- does not replace testing in the real app.

## B6. Gotchas

- **IPC overload.** Never send purely visual state (sidebar open, hovered item, current page, theme) to Go.
  Batch and debounce anything that must cross the bridge (see the autosave in `Editor.svelte`).
- **Prop drilling.** If state passes through more than one intermediate component, move it to
  context or to a `.svelte.js` module.
- **Do not remount CodeMirror.** Toggling it with `{#if}` destroys the `EditorView` (history,
  selection, scroll). Hide panels with CSS (`hidden`, `display: none`, or a zero-width column).
  Never re-key its parent. A theme change must not remount it either: tokens are CSS variables, so the
  editor repaints by itself.
- **The PDF.js canvas.** Resizing the Preview triggers its debounced `ResizeObserver` re-render. Avoid
  layouts that resize it on every animation frame for long periods. Never destroy documents directly:
  ownership rules are in `lib/pdf.js` (`releasePdf`, shared `PDFWorker`). The canvas is painted by PDF.js;
  the paper stays white in both themes (the sheet is the document, not chrome).
- **Rune name clashes.** Do not name a prop or variable `state`, `derived`, `effect` or `props` in a
  component that uses that rune: `$state` would be parsed as a store subscription.
- **`svelte-ignore`.** In runes mode, codes are comma-separated:
  `<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->`.
- **Accessibility.**
  - Icon-only buttons need `aria-label` and `title`. Modals need `role="dialog"`, `aria-modal` and closing with Escape.
  - A purely informative element (e.g. the "⚠ Proyecto grande" note) uses `role="note"` + `title`, and gets no
    `tabindex` (the checker warns about it).
- **Shortcuts and file opening.**
  - Global shortcuts live in `App.svelte` (`<svelte:window onkeydown>`): Mod-n new file, Mod-Shift-n quick
    document, Mod-s save + compile, Mod-Shift-s save as, Mod-o open folder, Mod-b sidebar, Mod-j terminal,
    Mod-\ split editor.
  - They all go through `runCommand()` in `lib/actions.js`, which the native macOS menu (`menu:command`) also
    uses. Do not rebind them in CodeMirror's keymap.
  - Open files with `openFile()`, and never assign `workspace.currentFilePath` elsewhere: it keeps a file out of
    both panes at once.
- **Window background.** The Wails window paints `BackgroundColour` (set in `main.go`) before the WebView. If
  it does not match the theme, the app flashes at startup (see A6).
- **Project rules.** You cannot run `npm install`, `npm run`, `wails dev` or `wails build`.
  If a component needs a new dependency (e.g. `@lezer/highlight` for the editor theme), stop and give the
  user the exact command, to run in `creative-ink/frontend/`.
