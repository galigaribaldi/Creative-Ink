# Creative-ink — Contexto para Claude

Editor LaTeX de escritorio, nativo, ultraligero y modular, para **macOS, Linux y Windows** (Windows entró en el alcance el 2026-11-07).
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
- **Repositorio central** `My-Creative-Ink-Repository`: en el **primer arranque** el usuario elige "Solo para mí" (sus Documentos) o "Para todos los usuarios" (Documentos compartidos; no existe en Linux). Go calcula la ruta según el sistema operativo y guarda la elección en la configuración del usuario.
- **La raíz del repositorio es un lanzador:** cada sesión empieza en la pantalla de inicio. Un proyecto es una carpeta `<repo>/<Nombre-Normalizado>/`; los documentos rápidos van en `<repo>/tmp-Workspaces/unnamed-tmpN/`. Datos del editor en `<repo>/.creative-ink/`. Diseño completo en `docs/MAP.md` §4.6.
- **Un archivo activo** (sin pestañas, por minimalismo), más un **segundo panel opcional** para ver dos archivos lado a lado (⌘\). Se edita cualquier archivo de texto (`.tex`, `.bib`, `.sty`…). Se compila el **archivo principal** del proyecto si está definido (`.creative-ink-meta`); si no, el último `.tex` activo.
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
- **pdfjs-dist 6.5, renderer worker:** se pinta en un Web Worker propio (`GlobalWorkerOptions.rendererSrc`), uno por documento, que termina `loadingTask.destroy()`. No vuelvas a poner `disableWorkerRendering: true`: el pintado volvería al hilo de CodeMirror.
- **Compilación automática apagada por defecto** (`compile.auto`, `internal/config`). Lee y escribe la configuración solo a través del servicio (`GetConfig`/`SetConfig`, `ci config`), nunca de `settings.json` o `.creative-ink-meta` a mano. Las peticiones del temporizador van con `RequestCompile('auto')`; las del usuario, con `'manual'`.
- **Procesos externos que se puedan cancelar** (latexmk, herramientas de `ci`): usa `system.KillTreeOnCancel(cmd)`; si no, cancelar deja huérfanos a xelatex/biber.
- **pdfjs-dist 6:** `PDFDocumentProxy.destroy()` **no existe**. Se libera con `doc.loadingTask.destroy()` (vía `releasePdf()` en `src/lib/pdf.js`). Usa siempre el `PDFWorker` compartido: sin él, cada documento crea un Web Worker nuevo.
- **fsnotify en macOS (kqueue):** abre un descriptor por carpeta **y por archivo** vigilado, y no es recursivo. Respeta `maxWatchedDirs` y los filtros de `isIgnoredEntry`; nunca vigiles carpetas de build ni `node_modules`.
- **pdfjs-dist 6 por rangos:** `disableAutoFetch: true` **exige** `disableStream: true`. Sirve siempre instantáneas inmutables (`?v=`), nunca el `<doc>.pdf` que latexmk reescribe. En Linux sin `-tags webkit2_41`, Wails no pasa cabeceras: no hay rangos (se descarga entero).
- **Terminal integrada (`RunTerminalCommand`):** es la **única** excepción a "nunca `sh -c`" y es ejecución arbitraria por diseño. Pásale solo lo que el usuario escribió en la terminal; nunca construyas comandos a partir de documentos, logs o nombres de archivo, y nunca uses `{@html}` con contenido externo.
- **macOS necesita el menú nativo (`menu.go`):** sin el rol Edición, ⌘C/⌘X/⌘V no funcionan en WKWebView. Los atajos de la app viven en `App.svelte` (`runCommand`) y hacen `preventDefault`, así el menú no los repite. En Linux y Windows no se crea menú.
- **Archivos de texto:** Go solo abre UTF-8 sin bytes nulos y de hasta 8 MB (`FileManager.LoadText`). Nunca abras un archivo que el puente pueda alterar: el autoguardado lo reescribiría.
- **Editor dividido:** un mismo archivo nunca puede estar en los dos paneles (los autoguardados se pisarían). Usa siempre `openFile()` de `actions.js`; no asignes `workspace.currentFilePath` directamente desde otros módulos.
- **Convención de nombres:** todo lo que crea el editor usa `-` en lugar de espacios y sin acentos (`core.Slugify`). Nunca crees carpetas o archivos con espacios.
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
- [ ] **Sección 8 — Carpetas, archivos y explorador** (código listo; falta validar con `wails dev`)
  - [x] Backend: `core/workspace.go` (`ScanTree`, `ResolveInFolder`, `DefaultTeXFile`) más tests; `App` con carpeta y documento dinámicos (`RWMutex`)
  - [x] Bindings `GetWorkspace`, `RefreshWorkspace`, `OpenFolderDialog`, `OpenFile`, `SaveAsDialog`; `SaveContent(path, content)`. Se quitaron `LoadContent` y `DocumentPath`
  - [x] Eventos `compile:*` con `file`; diálogos inyectables; documento inicial en `./workspace` si falta
  - [x] Frontend: `lib/workspace.svelte.js`, `FileTreeItem.svelte`, explorador, botones Abrir carpeta y Guardar como, ⌘O y ⇧⌘S
  - [x] `Editor` cambia de archivo con `setState` sin desmontarse; guarda en el archivo anterior antes de cambiar
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 9 — Repositorio central multiplataforma** (deuda técnica #4; código listo, falta validar con `wails dev`)
  - [x] `core.PrepareRepository(dir, starter, shared)`: `MkdirAll 0755` (`0777` + sticky en modo compartido, salvo Windows), `main.tex` inicial con `O_EXCL` solo si no hay `.tex`
  - [x] `system/platform.go` + `documents_{darwin,linux,windows,other}.go`: sistema operativo y Documentos reales (macOS `~/Documents` y `/Users/Shared`; Linux XDG `user-dirs.dirs`; Windows Known Folders `FOLDERID_Documents` y `FOLDERID_PublicDocuments`), más avisos de iCloud y OneDrive
  - [x] `core/settings.go`: elección guardada en `<UserConfigDir>/creative-ink/settings.json`
  - [x] `App`: sin `./workspace`. Sin configuración → `GetWorkspace` devuelve `needsSetup` y no crea nada. Bindings `GetRepositoryOptions` y `SetupRepository(scope)`; el diagnóstico incluye `repository`
  - [x] Frontend: `RepositorySetup.svelte` (primer arranque) y sección "Sistema y repositorio" en `HealthPanel`
  - [x] `go vet` con `GOOS=linux` y `GOOS=windows` limpio
  - [x] Tests con `HOME` temporal y `t.Chdir` (simula el arranque desde el Finder); `go test -race ./...` limpio
  - [x] `.gitignore` raíz: `*.pdf`, `*.aux`, `*.log`, `*.fls`, `*.fdb_latexmk`, `*.xdv`, `*.synctex.gz` y demás auxiliares de LaTeX
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 10 — Archivo principal para proyectos de varios archivos** (deuda técnica #6; código listo, falta validar con `wails dev`)
  - [x] `core/project_meta.go`: `.creative-ink-meta` con `mainFile` relativo; se ignora si es inválido o no existe
  - [x] `App.mainFile` (bajo `mu`, se recarga al cambiar de carpeta), `compileTarget()` para el Scheduler, `preview.go` y los eventos; guardar cualquier `.tex` recompila el principal
  - [x] Binding `SetMainFile(path)` (`""` lo quita); `WorkspaceState.mainFilePath`
  - [x] Frontend: `compileTarget()` en `workspace.svelte.js`; insignia `★ MAIN` y botón ☆/★ al pasar el ratón en `FileTreeItem`; la vista previa muestra el nombre del PDF
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 11 — Explorador reactivo a cambios externos** (deuda técnica #7; código listo, falta validar con `wails dev`)
  - [x] `github.com/fsnotify/fsnotify v1.9.0` en `go.mod`
  - [x] `core/watcher.go`: `FolderWatcher` recursivo con filtros, debounce de 300 ms, `recover`, límite de 256 carpetas; `TreeSignature`
  - [x] `App`: `restartWatcher` al cambiar de carpeta, `stopWatching` en el cierre, `onFolderChanged` emite `workspace:changed` solo si el árbol cambió
  - [x] Frontend: `listenWorkspaceEvents()` → `refreshWorkspace()` (uno en curso, los demás juntados) y `applyTree()`, que no toca el archivo abierto
  - [x] `go vet`, `go test`, `go test -race` y `GOOS=linux/windows go vet` limpios; tests del watcher estables (5 repeticiones)
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 12 — Atajos, debounce de compilación y cierre seguro** (deudas técnicas #2 y #3; código listo, falta validar con `wails dev`)
  - [x] Cmd+S / Ctrl+S global en `App.svelte` (`preventDefault`): guardar al instante y compilar; se quitó `Mod-s` del keymap de CodeMirror
  - [x] `Editor.svelte`: guardar a 1 s y compilar a 3 s de inactividad (`saveAndCompile` → `RequestCompile`); `SaveContent` ya no compila
  - [x] `OnBeforeClose` → `App.beforeClose`: evento `app:before-close`, binding `ConfirmClose(saved)`, límite de 2 s y diálogo nativo si el guardado falló
  - [x] Tests del protocolo de cierre (confirmación, timeout, cancelar, duplicados); `go test -race` y `GOOS=linux/windows go vet` limpios
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 13 — Visor PDF por rangos HTTP** (deuda técnica #5; código listo, falta validar con `wails dev` y medir con un PDF grande)
  - [x] `lib/pdf.js`: `getDocument({ url, rangeChunkSize: 65536, disableAutoFetch: true, disableStream: true, worker })`; sin `fetch` completo
  - [x] `core/pdf_snapshots.go`: instantáneas inmutables y versionadas (3), copiadas en streaming en la goroutine del worker; `compile:finished.pdfVersion`
  - [x] `preview.go`: `?v=<version>`, `http.ServeContent` con 206 Partial Content (tests de `Range` y de inmutabilidad)
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 14 — Disparador adaptativo y prevención de recarga** (§4.5; código listo, falta validar con `wails dev`)
  - [x] `compilation.svelte.js`: `autoCompileDelayMs = max(3000, 2 × durationMs)` y `autoCompilePaused` si `durationMs > 10000`, por objetivo y en memoria; aviso en la barra de estado del editor
  - [x] `Editor.svelte`: lee el retraso al programar cada compilación; no programa nada mientras esté pausado
  - [x] `App.publishPreview`: `os.Stat` del PDF tras compilar; reutiliza `pdfVersion` si `ModTime` y tamaño no cambiaron (sin copia ni recarga); `compile:finished.pdfModTime`
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 15 — Terminal integrada** (código listo, falta validar con `wails dev`)
  - [x] `core/terminal{,_unix,_windows}.go`: `ExecuteCommand(cmd, workdir)` con `sh -c` / `cmd /c`, 30 s, sin stdin, entorno no interactivo, grupo de procesos, salida limitada a 256 KB; tests (incluye que el timeout mata a los hijos)
  - [x] `system.ToolPATH()`; bindings `RunTerminalCommand` (uno a la vez) y `CancelTerminalCommand`; se cancela al cerrar la app
  - [x] Frontend: `BottomPanel` (pestañas Errores LaTeX | Terminal), `TerminalPanel`, `terminal.svelte.js`; Ctrl+` y ⌘J/Ctrl+J; botón en la ActionBar
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 16 — SyncTeX bidireccional** (código listo, falta validar con `wails dev`)
  - [x] latexmk con `-synctex=1`; `<doc>.synctex(busy)` filtrado del explorador y del watcher
  - [x] `compiler/synctex.go`: `synctex view` / `synctex edit` sin shell, 5 s, parseo del primer registro; sin datos → `found: false` silencioso
  - [x] Bindings `SyncTexForward(line, file)` y `SyncTexBackward(page, x, y)` (solo `.tex` del proyecto), sobre el PDF del objetivo de compilación
  - [x] Frontend: `lib/synctex.svelte.js`; ⌘-clic / Ctrl-clic y ⌘⌥J / Ctrl+Alt+J en el editor → salto de página y recuadro en el PDF; doble clic en el PDF → abre el `.tex` y selecciona la línea
  - [ ] Validación del programador: reiniciar `wails dev` y compilar una vez (para generar `.synctex.gz`)
- [ ] **Sección 17 — Terminal híbrida con comandos `ci`** (código listo, falta validar con `wails dev`)
  - [x] `internal/ci`: `ci help`, `ci config autocompile [on|off]`, `ci compile rapido|borrador|completo|solo <cap>`, `ci clean tikz|all` (traducción del Makefile, sin shell ni make). `ci check` se quitó: las herramientas propias de un proyecto llegarán como plugins
  - [x] `compiler.RunPasses` / `WithCompileLock`: los builds manuales y la copia de la instantánea comparten el candado con latexmk; 10 min de límite
  - [x] `RunTerminalCommand` enruta `ci …` a Go; los builds emiten `compile:started`/`compile:finished`; `settings.json.autoCompile` + `WorkspaceState.autoCompile`
  - [ ] Pendiente de decisión (propuesta en `docs/MAP.md` §4.6): servicio de configuración por capas, confianza de carpetas para `.latexmkrc`, registro de comandos `ci` y host de plugins. Hasta entonces la UI no respeta `autoCompile`, y `--shell-escape` no se ofrece
  - [ ] Validación del programador: reiniciar `wails dev` y probar `ci help` en la terminal
- [ ] **Sección 18 — Estructura del repositorio y pantalla de inicio** (§4.6, paso 1; código listo, falta validar con `wails dev`)
  - [x] `My-Creative-Ink-Repository` con migración automática del nombre con espacios (sin sobrescribir; avisa si existen las dos carpetas)
  - [x] `core/repository.go`: `.creative-ink/{config.json,plugins,templates,guide}` (oculta en Windows), `tmp-Workspaces/unnamed-tmpN`, `Slugify`, plantilla de proyecto, `StaleQuickDocs` (más de 8 h, solo aviso)
  - [x] Bindings `GetStartScreen`, `CreateProject`, `CreateQuickDocument`, `OpenProject` (solo dentro del repositorio o recientes), `RevealQuickDocuments`; `WorkspaceState.needsStart/repositoryPath`; recientes en `settings.json`
  - [x] Plugins por usuario: `<repo>/.creative-ink/plugins` o `<UserConfigDir>/creative-ink/plugins` si el repositorio es compartido
  - [x] Frontend: `StartScreen.svelte` en cada sesión y botón Inicio en la ActionBar
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 19 — Comandos básicos de editor, archivos de texto y editor dividido** (código listo; falta instalar `@codemirror/search` y validar con `wails dev`)
  - [x] `core.IsEditableFile` y `FileManager.LoadText` (8 MB, UTF-8, sin bytes nulos): se abren y editan `.bib`, `.sty`, `.cls`, `.txt`, `.md`…; `OpenFile`/`SaveContent`/`SaveAsDialog` aceptan texto; `SetMainFile` y SyncTeX siguen solo con `.tex`
  - [x] Objetivo de compilación = principal o **último `.tex`** (`App.texDoc`, `WorkspaceState.texFilePath`)
  - [x] Bindings `NewFileDialog` (⌘N, nombre normalizado, nunca reemplaza) y `LoadFile` (segundo panel)
  - [x] `menu.go`: menú nativo en macOS (Archivo, Edición, Buscar, Ver, Ventana) → `menu:command`
  - [x] Frontend: `runCommand` (atajos y menú), ⌘N / ⇧⌘N, búsqueda y reemplazo, ir a la línea, ⌘D, varios cursores, ⌘/ y resaltado por tipo (`lib/languages.js`, modo BibTeX propio)
  - [x] Editor dividido (⌘\, botón en la ActionBar): dos paneles, sin el mismo archivo en ambos; botón "Nuevo archivo" en el explorador
  - [ ] Programador: `cd frontend && npm install @codemirror/search`
  - [ ] Validación del programador: reiniciar `wails dev`
- [ ] **Sección 20 — Escalabilidad: configuración, PDF en un worker, cancelación y terminal en disco** (decisiones del 2026-11-08; código listo, falta validar con `wails dev`)
  - [x] `internal/config`: claves tipadas y capas (por defecto < usuario < proyecto); `GetConfig`, `SetConfig`, `config:changed`; `ci config list|get|set|unset [--project]`. **`compile.auto` apagado por defecto**, con cerrojo en la UI y en Go (`RequestCompile('auto'|'manual')`); al abrir sin compilación automática se muestra el PDF existente (`cached`)
  - [x] PDF.js con renderer worker (`pdf.renderer.min.mjs`) y detector de proyecto grande (`core.AnalyzeProject`: archivos, tamaño, imágenes) → `project:health` y "⚠ Proyecto grande" con tooltip (solo avisa)
  - [x] Scheduler con cancelación acotada (solo automáticas, solo jóvenes, nunca dos seguidas; `canceled`) y `system.KillTreeOnCancel` para latexmk y `ci`
  - [x] Terminal: salida completa en disco (`core.OutputSpool`, `ReadTerminalOutput`, `terminal:started`), progreso de `ci compile` (`terminal:progress`: pasada, página, total) y cola acotada de 256 KB en Go; botón "Salida completa" en la UI
  - [ ] Validación del programador: reiniciar `wails dev` y medir con la tesis (§4.5)
