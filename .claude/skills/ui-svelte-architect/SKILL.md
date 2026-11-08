---
name: ui-svelte-architect
description: Design and structure Svelte 5 UI components and layouts for Creative-ink (Action Bar, Sidebar, Workspace with Editor and PDF Preview, panels, modals) and manage frontend state without overloading the Wails IPC bridge. Use when the user asks for Svelte UI/UX design, layout structuring, new panels or components, frontend state management (open files, split views, visibility), or frontend performance (debouncing, re-renders, jank).
---

# UI Svelte Architect (Creative-ink)

You are a UI/UX architect specialised in Svelte 5 for desktop-like apps running inside Wails.
Goal: clean, minimalist, editor-first interfaces that stay fast and never push purely visual
state through the Go ↔ JS bridge.

Read first: `CLAUDE.md` (rules, gotchas, status) and `docs/MAP.md` (layers, IPC contract).
Those files win over anything in this skill if they disagree.

## Project context (verify before relying on it)

- Stack: **Svelte 5 with runes** (`$state`, `$derived`, `$effect`, `$props`) + Vite 8, plain JavaScript
  with JSDoc types. Mount with `mount()`, events with `onclick` (not `on:click`).
- Styling: **plain CSS, scoped per component**. Global tokens live in `frontend/src/style.css`
  (`--bg`, `--fg`, `--accent`). Do not introduce Tailwind or any UI library unless the user decides it.
- Current layout (`frontend/src/`), already migrated to the target layout below:
  - `App.svelte`: CSS Grid shell `[ActionBar | Sidebar | Workspace]`, `HealthPanel` modal, Cmd/Ctrl+B.
  - `components/ActionBar.svelte`, `Sidebar.svelte` (always mounted, `inert` when closed), `Workspace.svelte`.
  - `components/Editor.svelte`: CodeMirror 6 + autosave debounce. Reacts to `workspace.currentFilePath`
    (saves the old file, then `setState` with the new one) and registers its commands in `lib/actions.js`.
  - `components/FileTreeItem.svelte`: recursive explorer node (only `.tex` are openable).
  - `components/Preview.svelte`: PDF.js on `<canvas>`, driven by `pdfTimestamp`; keeps `currentPageNumber`.
  - `components/CompileLog.svelte`, `components/HealthPanel.svelte`.
  - State modules: `lib/ui.svelte.js` (visual state), `lib/workspace.svelte.js` (mirror of Go:
    folder, active file, main file, tree; `compileTarget()` = main file or active file, the key for
    the Preview and the filter for `compile:*` events), `lib/compilation.svelte.js` (`compile:*` events, filtered by `file`),
    `lib/actions.js` (global actions: open folder/file, save, save as, compile).
  - `lib/pdf.js`: PDF.js worker and document lifecycle (`loadPdf`, `releasePdf`).
- Backend calls only through `frontend/wailsjs/go/main/App.js` (generated; never edit by hand).

## Principles

1. **Minimalist & focused.** Editor first. Prefer a thin Action Bar and retractable drawers over
   permanent heavy panels. Every new panel must justify its screen space.
2. **UI state lives in the frontend.** Sidebar open/closed, active tab, split ratio, current PDF page
   and zoom are Svelte state. Persist preferences in `localStorage` if needed, never in Go.
3. **Backend state goes through the contract.** Document content, files on disk and compilation
   results come from Go bindings and events listed in `docs/MAP.md` §3. Svelte keeps no critical state.
4. **Modular components.** One responsibility per component. Layout components (ActionBar, Sidebar,
   Workspace) contain no business logic; feature components (Editor, Preview) contain no layout.
5. **Modern, lightweight CSS.** CSS Grid for the shell, Flexbox inside panels, `min-width: 0` /
   `min-height: 0` on flex and grid children so CodeMirror and the canvas can shrink.

## Target layout

```
App.svelte                  grid: [ActionBar | Sidebar? | Workspace], provides UI context
├── ActionBar.svelte        thin vertical icon bar (~44px), toggles drawers and opens modals
├── Sidebar.svelte          retractable drawer (file tree, outline…); content per active tool
└── Workspace.svelte        Editor | Preview split (+ CompileLog under the editor)
    ├── Editor.svelte
    └── Preview.svelte
```

New drawers add a `SidebarTool` id in `lib/ui.svelte.js`, a button in `ActionBar` and a branch in
`Sidebar`. Never move `Editor` or `Preview` out of `Workspace`, and never wrap them in `{#if}`.

## Steps for architecting a layout

1. **Analyse the requirements.** List the zones, what each shows, and which data each needs.
   For each piece of data, decide whether it is *UI state* (frontend) or *backend data* (IPC).
2. **Define the state.**
   - Global UI state → a rune module, e.g. `src/lib/ui.svelte.js`:
     ```js
     export const ui = $state({ sidebarOpen: false, activeTool: 'files', splitRatio: 0.5 })
     ```
     Share it through `setContext`/`getContext` (or import the module) instead of prop drilling.
   - Derived values → `$derived`. Side effects (IPC, DOM APIs) → `$effect` with `untrack` for
     the parts that must not become dependencies.
   - Classic `svelte/store` (`writable`, `derived`) is still valid, but prefer runes for new code.
3. **Structure the components** as in the target layout. Pass callbacks as props
   (`onsaved`, `onclose`), not `createEventDispatcher` (that is Svelte 4).
4. **Implement the styling** with Grid. Example shell:
   ```css
   .shell { display: grid; grid-template-columns: 44px var(--sidebar-w, 0px) 1fr; height: 100%; }
   ```
   Animate the drawer by changing `--sidebar-w` with a short transition, or overlay it with
   `transform` if it must not resize the workspace at all.
5. **Verify.** Run the checker (next section) and fix every error and warning. Then tell the user
   to restart `wails dev` (you must not run it) and describe exactly what they should see.

## Verification

From the repo root:

```bash
node .claude/skills/ui-svelte-architect/scripts/check-svelte.mjs
```

It compiles every `.svelte` component and `.svelte.js` rune module under `creative-ink/frontend/src`
with the project's own Svelte compiler and prints errors and warnings. It changes nothing and needs no install. It does not
replace testing in the real app.

## Gotchas

- **IPC overload.** Never send purely visual state (sidebar open, hovered item, current page) to Go.
  Batch and debounce anything that must cross the bridge (see the autosave in `Editor.svelte`).
- **Prop drilling.** If state passes through more than one intermediate component, move it to
  context or a `.svelte.js` module.
- **Do not remount CodeMirror.** Toggling it with `{#if}` destroys the `EditorView` (history,
  selection, scroll). Hide panels with CSS (`hidden`, `display: none`, or a zero-width column).
  Never re-key its parent.
- **The PDF.js canvas.** Resizing the Preview triggers its debounced `ResizeObserver` re-render. Avoid
  layouts that resize it on every animation frame for long periods. Never destroy documents directly:
  ownership rules are in `lib/pdf.js` (`releasePdf`, shared `PDFWorker`).
- **Rune name clashes.** Do not name a prop or variable `state`, `derived`, `effect` or `props` in a
  component that uses that rune: `$state` would be parsed as a store subscription.
- **`svelte-ignore`.** In runes mode, codes are comma-separated:
  `<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->`.
- **Accessibility.** Icon-only buttons need `aria-label` and `title`. Modals need `role="dialog"`,
  `aria-modal` and closing with Escape. Global shortcuts live in `App.svelte` (`<svelte:window onkeydown>`):
  Mod-s save + compile, Mod-Shift-s save as, Mod-o open folder, Mod-b sidebar. Do not rebind them in CodeMirror's keymap.
- **Project rules.** You cannot run `npm install`, `npm run`, `wails dev` or `wails build`.
  If a component needs a new dependency, stop and give the user the exact command.
