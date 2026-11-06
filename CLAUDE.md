# Creative-ink — Contexto para Claude

Editor LaTeX de escritorio, nativo, ultraligero y modular, para **macOS y Linux**.
Este repositorio es el **"Repo 1" (Core)**. Los plugins vivirán en repos externos más adelante.

Mapa de arquitectura y skills requeridas: @docs/MAP.md

## Stack
- **Backend:** Go con Wails v2. Tiene control total del sistema de archivos y de los subprocesos (compiladores LaTeX).
- **Frontend:** Svelte 5 (sintaxis runes: `$state`, `onclick`) + Vite 8, JavaScript (template `svelte` de Wails v2.17.1). Es solo interfaz y **no guarda estado crítico**. Montar con `mount()` de `svelte`, no con `new App()`.
- **IPC:** métodos de Go enlazados (bindings `window.go.main.App.*`) y eventos de Wails (`runtime.EventsEmit` / `EventsOn`).
- **Prohibido:** Electron o cualquier runtime pesado equivalente.

## Ubicación del código
- La app Wails vive en `./creative-ink/` (la genera `wails init -n creative-ink -t svelte`).
- La raíz del repo guarda la metadocumentación: `CLAUDE.md`, `docs/` y `.claude/`.

## Visión del MVP (Repo 1)
Un editor LaTeX funcional "al estilo VS Code": explorador de archivos, editor, compilación y vista previa.
- La compilación usa **latexmk** por defecto, con un motor seleccionable: `pdflatex`, `xelatex` o `lualatex`.
- La app debe **detectar** los motores instalados (health check en `internal/system/`) y **ofrecer instalarlos** si faltan. La instalación siempre la confirma el usuario.
- **Editor decidido:** CodeMirror 6 (sin `basicSetup`; las extensiones se eligen a mano en `Editor.svelte`).
- **Visor PDF decidido:** PDF.js (`pdfjs-dist`) en `<canvas>`; el PDF lo sirve el middleware de Go (`preview.go`).
- Decisiones **pendientes** (no las asumas, espera instrucciones): modelo de plugins, migración a TypeScript, licencia y patrón final de IPC.

## Skills del proyecto (`.claude/skills/`)
- **`ui-svelte-architect`**: diseño de layouts y componentes Svelte 5 (ActionBar, Sidebar, Workspace, paneles) y estado del frontend sin sobrecargar el IPC. Úsala para cualquier trabajo de UI. Para verificar: `node .claude/skills/ui-svelte-architect/scripts/check-svelte.mjs` (solo lectura).
- **`go-wails-ipc-bridge`**: backend en Go para Wails: fachada `App`, bindings y eventos, tareas asíncronas sin bloquear el IPC, resultados frente a errores y data races. Úsala para cualquier cambio en `app.go` o `internal/*`.
- **`latex-toolchain-orchestrator`**: compilación con latexmk: resolución de binarios TeX, ejecución segura sin shell, timeouts, selección de motor y extracción de errores de TeX. Úsala para cualquier cambio en `internal/compiler/`.

## Reglas de trabajo (OBLIGATORIAS)
Las hace cumplir `.claude/settings.json`. Aun así, respétalas aunque un permiso falte:
1. **Git:** NO ejecutar `git add`, `git rm`, `git commit`, `git push` ni nada que publique ramas o reescriba historia. Solo se permiten `git checkout`/`switch`/`stash`/`branch`, y siempre **con autorización previa**. Lectura (`status`, `diff`, `log`) libre.
2. **Instalación, instanciación y arranque:** NO ejecutar `wails init/dev/build`, `npm install/run`, `go get/install/mod tidy`, `brew`, `tlmgr`, `sudo`, ni compiladores LaTeX. **El programador ejecuta esos comandos.** Claude escribe el código y le dice exactamente qué comando correr.
3. Sí se permite verificar sin efectos externos: `go build`, `go vet`, `gofmt`, `wails doctor`.
4. Idioma de comunicación: **español**. Código e identificadores: inglés.
5. **Mensajes de commit:** NO agregar la línea `Co-Authored-By` ni ninguna otra atribución a Claude en los mensajes de commit que se propongan. Formato: prefijo convencional (`feat`, `fix`, `build`, `chore`…) y mensaje en español.

## Gotchas técnicos
- **Svelte 5 (runes):** en `<!-- svelte-ignore a, b -->` los códigos se separan con **coma**. Después de un espacio, el compilador trata el resto como comentario.
- **Svelte 5:** no llames `state` (ni `props`, `derived`, `effect`) a una prop o variable si usas la runa correspondiente: `$state` se interpretaría como un *store*.
- **pdfjs-dist 6:** `PDFDocumentProxy.destroy()` **no existe**. Se libera con `doc.loadingTask.destroy()` (vía `releasePdf()` en `src/lib/pdf.js`). Usa siempre el `PDFWorker` compartido: sin él, cada documento crea un Web Worker nuevo.
- **Template de Wails v2.17.1:** venía con `vite-plugin-svelte@6` (no acepta Vite 8), con `new App()` (Svelte 4) y con un `jsconfig` obsoleto. Ya está corregido.

## Estado actual (actualizar al cerrar cada sección)
- [x] **Sección 1 — Infraestructura y puente IPC** (validada por el programador con `wails dev` el 2026-11-04)
  - [x] `wails init` (lo ejecutó el programador; Wails v2.17.1)
  - [x] Limpieza del template (se quitaron `Greet`, el logo, la fuente Nunito y los estilos; `main.js` se migró a `mount()`)
  - [x] `internal/core/` y `internal/system/` (solo `doc.go`)
  - [x] `App.SystemHealthCheck() string` → `"Creative-ink Core: Puente IPC Activo"`
  - [x] `App.svelte`: fondo oscuro, título y botón que llama al binding y muestra la respuesta
  - [x] Bindings regenerados: `frontend/wailsjs/go/main/App.js` ya exporta `SystemHealthCheck`
  - [x] Fix del template: `@sveltejs/vite-plugin-svelte` subió de `^6` a `^7.3.0` (la v6 no acepta Vite 8 y daba ERESOLVE)
  - [x] Fix del template: en `frontend/jsconfig.json` se quitaron opciones obsoletas de TS 6 (`moduleResolution: Node` → `Bundler`, sin `importsNotUsedAsValues` ni `baseUrl`)
  - [x] Validación del programador: `wails dev` abre la ventana y el botón responde
- [x] **Sección 2 (Módulo 1) — Health check real y perfil de hardware** (validada por el programador con `wails dev` el 2026-11-05)
  - [x] `internal/system/health.go`: `HealthStatus{HasXeLaTeX, HasLatexmk, HasBiber}` con json tags en camelCase, y `CheckHealth()` (LookPath más rutas de respaldo)
  - [x] `internal/system/health_test.go`: tests unitarios (`go test ./internal/...`)
  - [x] `App.SystemHealthCheck() system.HealthStatus`
  - [x] `App.svelte`: tarjetas con ✅ Instalado / ❌ No encontrado y estado de carga
  - [x] Bindings regenerados (`wailsjs/go/models.ts` existe)
  - [x] **Parte 2: perfil de hardware**
    - [x] Programador: `go get github.com/shirou/gopsutil/v3/...` (v3.24.5)
    - [x] `HealthStatus` + `TotalRAMGB`, `CPUCores`, `FreeDiskSpaceGB`, `HardwareTier`, `HardwareTierID` y `Warnings`
    - [x] Tests de tier y de degradación cuando fallan las mediciones
    - [x] `App.svelte`: secciones "Herramientas de LaTeX" y "Recursos de Hardware", con badge de tier
    - [x] Validación del programador: en la UI se ven 48 GB, 12 núcleos, 737.4 GB libres y el tier Avanzado
- [x] **Sección 3 (Módulos 2 y 4) — Núcleo del editor** (validada por el programador con `wails dev` el 2026-11-05)
  - [x] Programador: `cd frontend && npm install @codemirror/view @codemirror/state @codemirror/language @codemirror/theme-one-dark @codemirror/commands @codemirror/legacy-modes`
  - [x] `internal/core/file_manager.go`: `FileManager.SaveDocument(content, path)` con escritura atómica y `LoadDocument`, más tests
  - [x] `App.SaveContent`, `App.LoadContent` y `App.DocumentPath` → `./workspace/main.tex` (ignorado por git)
  - [x] `components/Editor.svelte`: CodeMirror 6 + one-dark + stex, debounce de 1 s, guardados en serie, Cmd+S y barra de estado
  - [x] `components/HealthPanel.svelte`: modal que se abre desde "Ver estado del sistema"
  - [x] `App.svelte`: editor a la izquierda y espacio para la vista previa PDF a la derecha; ventana de 1280×800
  - [x] Validación del programador: escribir → "Guardado" y `workspace/main.tex` en disco sin `.tmp` residual
  - Nota: `npm audit` muestra 2 avisos *low* (`deepmerge` dentro de `@sveltejs/vite-plugin-svelte`). Es una dependencia de desarrollo, no llega al binario y no tiene fix publicado. El aviso de `fsevents` (install scripts) se puede ignorar: es opcional y solo sirve para vigilar archivos en macOS.
- [ ] **Sección 4 (Módulo 3) — Pipeline de compilación** (código listo; falta que el programador valide con `wails dev`)
  - [x] `internal/compiler/orchestrator.go`: `CompileLaTeX(workdir, filename) (string, error)`, más tests unitarios y un test de integración opcional
  - [x] `system.FindBinary` (exportado) para resolver latexmk con las rutas de respaldo
  - [x] `App.SaveContent` → `SaveResult{Saved, CompileSuccess, CompileLogs}` (compila solo si el guardado salió bien)
  - [x] `preview.go`: middleware de Wails que sirve `/__preview/document.pdf` (más `preview_test.go`)
  - [x] `Preview.svelte`: iframe con `?t=pdfTimestamp`, estado y log de errores; mantiene el último PDF válido si hay error
  - [x] `Editor.svelte`: `onsavestart`/`onsaved` y compilación inicial al cargar
  - [x] Validación del programador: el PDF se genera y se muestra
- [ ] **Sección 5 — Visor PDF.js multiplataforma** (rama `feat/visor-pdfjs`; código listo, falta validar con `wails dev`)
  - [x] Programador: `npm install pdfjs-dist` (6.5.438)
  - [x] `src/lib/pdf.js`: worker local con `?url` y `loadPdf(url)` (fetch → `getDocument({data})`)
  - [x] `Preview.svelte`: página 1 en `<canvas>`, doble buffer, cargas con secuencia, `ResizeObserver` y soporte HiDPI
  - [x] `CompileLog.svelte`: log de LaTeX debajo del editor; se vuelve a abrir con cada error nuevo
  - [x] Fix: liberación de documentos con `loadingTask.destroy()` + `PDFWorker` compartido (antes cada compilación dejaba vivo un worker y su documento)
  - [x] Retención de página: `currentPageNumber` se conserva entre compilaciones (ajustado a `numPages`)
  - [ ] Validación del programador: reiniciar `wails dev` y confirmar que ya no aparece el aviso de `destroy`
- [ ] **Sección 6 — Guardado y compilación desacoplados** (deuda técnica #1; código listo, falta validar con `wails dev`)
  - [x] `internal/compiler/scheduler.go`: `Scheduler` con un solo worker, ráfagas juntadas, `recover` de panics y `Close` con cancelación
  - [x] `CompileLaTeXContext` (cancelable) y errores leídos primero del `.log`; extractor robusto (no se traga el siguiente `!`, incluye la continuación de `l.<n>`)
  - [x] `App.SaveContent` → `SaveResult{Saved}` inmediato; eventos `compile:started` y `compile:finished` (`compiler.Result`); `OnShutdown` cancela latexmk
  - [x] Tests: planificador, fachada (`app_test.go`) y parseo de logs; `go test -race ./...` limpio
  - [x] Frontend: `App.svelte` escucha los eventos con `EventsOn` (y se da de baja en `onDestroy`); `Editor.svelte` ya no reporta compilación
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 7 — Layout ActionBar | Sidebar | Workspace** (código listo; falta validar con `wails dev`)
  - [x] `lib/ui.svelte.js` (estado visual con `$state`), `lib/compilation.svelte.js` (eventos `compile:*`) y `lib/actions.js` (guardar y compilar sin pasar props)
  - [x] `ActionBar.svelte`, `Sidebar.svelte` y `Workspace.svelte`; `App.svelte` como grid `[44px | 0/240px | 1fr]` con transición de 120 ms y Cmd/Ctrl+B
  - [x] Binding `RequestCompile()` para compilar sin guardar (más test)
  - [x] `check-svelte.mjs` ahora también compila los módulos `.svelte.js`
  - [ ] Validación del programador: reiniciar `wails dev`
