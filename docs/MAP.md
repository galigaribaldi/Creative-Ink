# Creative-ink — Mapa de arquitectura y skills

## 1. Mapa de directorios (objetivo)

```
Creative-Ink/                     ← raíz del repo (metadocumentación)
├── CLAUDE.md                     ← contexto persistente para Claude
├── .claude/settings.json         ← permisos (qué puede y no puede ejecutar Claude)
├── .claude/skills/ui-svelte-architect/ ← skill de diseño UI (SKILL.md + scripts/check-svelte.mjs)
├── .claude/skills/go-wails-ipc-bridge/ ← skill de backend Go/Wails (fachada, eventos, async)
├── .claude/skills/latex-toolchain-orchestrator/ ← skill de compilación con latexmk
├── docs/MAP.md                   ← este archivo
└── creative-ink/                 ← app Wails (generada por `wails init`)
    ├── main.go                   ← arranque de Wails: ventana, assets, Bind(&App{})
    ├── app.go                    ← struct App: ÚNICA fachada IPC expuesta al frontend
    ├── preview.go                ← middleware del AssetServer: sirve el PDF en /__preview/document.pdf
    ├── wails.json                ← configuración del proyecto Wails
    ├── go.mod
    ├── internal/                 ← lógica Go no importable desde fuera del módulo
    │   ├── compiler/
    │   │   ├── orchestrator.go   ← CompileLaTeXContext: latexmk -xelatex con timeout/cancelación, PATH de TeX, errores del .log
    │   │   └── scheduler.go      ← Scheduler: compilación en segundo plano, una a la vez, juntando ráfagas
    │   ├── core/                 ← dominio: proyectos, archivos, (futuro) plugins
    │   │   ├── file_manager.go   ← FileManager: guardado atómico (tmp + rename) y carga de documentos
    │   │   ├── workspace.go      ← ScanTree, ResolveInFolder, DefaultTeXFile, PrepareRepository
    │   │   ├── settings.go       ← elección del primer arranque en <UserConfigDir>/creative-ink/settings.json
    │   │   ├── project_meta.go   ← <carpeta>/.creative-ink-meta: archivo principal del proyecto (ruta relativa)
    │   │   ├── watcher.go        ← FolderWatcher (fsnotify): cambios externos con debounce; TreeSignature
    │   │   └── pdf_snapshots.go  ← PDFSnapshots: copias inmutables y versionadas del PDF para la vista previa
    │   └── system/               ← entorno: health check de LaTeX y detección/instalación de motores
    │       ├── health.go         ← HealthStatus: herramientas de LaTeX, hardware y repositorio
    │       ├── platform.go       ← RepositoryOptions por SO, parser de XDG user-dirs
    │       └── documents_<os>.go ← Documentos reales: darwin, linux (XDG), windows (Known Folders), other
    ├── build/                    ← iconos y metadatos de empaquetado por SO
    └── frontend/                 ← Svelte 5 + Vite 8
        ├── index.html
        ├── src/
        │   ├── main.js           ← monta App.svelte
        │   ├── App.svelte        ← shell en CSS Grid: [ActionBar | Sidebar | Workspace], atajo Cmd/Ctrl+B
        │   ├── components/
        │   │   ├── ActionBar.svelte   ← barra vertical de 44 px: Explorador, Guardar, Compilar, Estado del sistema
        │   │   ├── Sidebar.svelte     ← explorador: cajón de 240 px (siempre montado, `inert` si está cerrado)
        │   │   ├── FileTreeItem.svelte ← nodo recursivo del árbol; solo los `.tex` se pueden abrir
        │   │   ├── Workspace.svelte   ← Editor | Preview (+ CompileLog bajo el editor); solo layout
        │   │   ├── Editor.svelte      ← CodeMirror 6 (one-dark, stex) + autoguardado con debounce
        │   │   ├── CompileLog.svelte  ← panel de errores de LaTeX debajo del editor
        │   │   ├── HealthPanel.svelte ← modal "Estado del sistema" (incluye "Sistema y repositorio")
        │   │   ├── RepositorySetup.svelte ← primer arranque: "Solo para mí" / "Para todos los usuarios"
        │   │   └── Preview.svelte     ← PDF.js en <canvas>, doble buffer, conserva la página al recargar
        │   ├── lib/
        │   │   ├── ui.svelte.js          ← estado visual ($state): sidebarOpen, activeTool, healthOpen. Nunca va a Go
        │   │   ├── compilation.svelte.js ← reflejo de los eventos compile:* (status, logs, pdfTimestamp)
        │   │   ├── workspace.svelte.js   ← reflejo de Go: currentFolderPath, currentFilePath, fileTree
        │   │   ├── actions.js            ← acciones globales (abrir carpeta/archivo, guardar, guardar como, compilar)
        │   │   └── pdf.js        ← configuración de PDF.js (worker local) y loadPdf(url)
        │   └── style.css         ← estilos globales mínimos
        └── wailsjs/              ← GENERADO por Wails: no editar a mano
            ├── go/main/App.js    ← bindings de los métodos de App
            ├── go/models.ts      ← tipos TS de los structs de Go (p. ej. system.HealthStatus)
            └── runtime/          ← EventsOn, EventsEmit, etc.
```

**Fuera del repo, en el sistema operativo del usuario:**

El repositorio central `My Creative Ink Repository` se crea **la primera vez que se abre la app**. Una pantalla de bienvenida (`RepositorySetup.svelte`) muestra el sistema detectado y las rutas propuestas, y el usuario elige el ámbito:

| SO | "Solo para mí" (`user`) | "Para todos los usuarios" (`shared`) | Cómo se calcula |
|----|-------------------------|--------------------------------------|-----------------|
| macOS | `~/Documents/My Creative Ink Repository` | `/Users/Shared/My Creative Ink Repository` | Rutas fijas del sistema |
| Linux | `<XDG_DOCUMENTS_DIR>/My Creative Ink Repository` (p. ej. `~/Documentos/…`) | **No disponible** (no hay una carpeta compartida estándar sin `sudo`) | `$XDG_CONFIG_HOME/user-dirs.dirs`; si falta, `~/Documents` |
| Windows | `<FOLDERID_Documents>\My Creative Ink Repository` (puede estar en OneDrive) | `<FOLDERID_PublicDocuments>\My Creative Ink Repository` (normalmente `C:\Users\Public\Documents`) | Known Folders (`golang.org/x/sys/windows`), nunca `%USERPROFILE%` a mano |

- **Avisos del diagnóstico:** en macOS, si Documentos se sincroniza con iCloud; en Windows, si Documentos está en OneDrive (cada compilación subiría `.aux`/`.pdf`). En Linux, cuando la carpeta XDG no es `~/Documents`.
- **Por qué en el primer arranque y no en el instalador:** Wails v2 solo genera instalador en Windows (NSIS); en macOS es un `.app` y en Linux un binario. El primer arranque funciona igual en los tres sistemas.
- **La ruta la decide Go:** el frontend solo envía `"user"` o `"shared"` a `SetupRepository`; los ámbitos desconocidos o no disponibles se rechazan.
- **Creación:** `core.PrepareRepository` con `MkdirAll(…, 0755)`. En modo compartido, la carpeta pasa a `0777` + sticky (como `/Users/Shared`: todos pueden crear documentos y solo su dueño puede borrarlos); en Windows mandan las ACL de Documentos públicos. Lleva un `main.tex` inicial con `O_EXCL` solo si no hay ningún `.tex`: **nunca sobrescribe**.
- **Elección guardada** por usuario en `<os.UserConfigDir()>/creative-ink/settings.json` (macOS `~/Library/Application Support`, Linux `~/.config`, Windows `%AppData%`). Si el archivo está corrupto, se vuelve a preguntar.
- El usuario puede abrir cualquier otra carpeta con ⌘O; el repositorio central es solo el punto de partida.

## 2. Reglas de capas

```
 Svelte (UI, sin estado crítico)
        │  bindings (request/response)   ▲ eventos (stream/async)
        ▼                                │
 app.go  ── fachada IPC: valida la entrada, delega y traduce errores
        │
        ▼
 internal/core    internal/system      ← sin dependencias de Wails (testeables)
        │                │
        ▼                ▼
   sistema de archivos   subprocesos (latexmk, kpsewhich, gestores de paquetes)
```

- `app.go` es la **única** puerta de entrada desde el frontend. Los métodos públicos de `App` se exponen como bindings.
- `internal/*` **no importa** `github.com/wailsapp/wails`. Para emitir eventos recibe una interfaz o callback inyectado desde `App`.
- El frontend **nunca** toca el disco ni lanza procesos. Todo pasa por Go.
- `frontend/wailsjs/` se regenera con `wails dev` o `wails build` (el programador los ejecuta).

## 3. Contrato IPC (vivo: actualizar con cada método o evento nuevo)

| Tipo    | Nombre                | Dirección  | Firma / payload | Sección |
|---------|-----------------------|------------|-----------------|---------|
| Binding | `SystemHealthCheck`   | UI → Go    | `() → system.HealthStatus` = `{ hasXeLaTeX, hasLatexmk, hasBiber: boolean; totalRAMGB, freeDiskSpaceGB: number; cpuCores: number; hardwareTier: string; hardwareTierId: 'basic'\|'advanced'\|'unknown'; repository: system.RepositoryOptions; warnings: string[] }`. Incluye el SO y el repositorio (actual y rutas propuestas) | 2, 9 |
| Binding | `GetWorkspace`        | UI → Go    | `() → main.WorkspaceState` = `{ needsSetup, canceled, folderPath, filePath, mainFilePath, tree: core.FileNode, truncated }`. `mainFilePath` es el archivo principal del proyecto (`""` si no hay, o si se borró). Carpeta abierta (al inicio, el repositorio central guardado), documento activo y árbol. **`needsSetup: true`** en el primer arranque, sin crear nada en disco | 9 |
| Binding | `GetRepositoryOptions` | UI → Go   | `() → system.RepositoryOptions` = `{ os, osName, user, shared: RepositoryOption; warnings: string[]; currentScope, currentPath }`, con `RepositoryOption` = `{ scope, path, available, reason }`. Rutas propuestas para este SO | 9 |
| Binding | `SetupRepository`     | UI → Go    | `(scope: 'user'\|'shared') → main.WorkspaceState`. Crea el repositorio en la ruta que calcula Go, guarda la elección y lo abre. Rechaza ámbitos desconocidos o no disponibles | 9 |
| Binding | `RefreshWorkspace`    | UI → Go    | `() → main.WorkspaceState`: vuelve a escanear la carpeta abierta. La UI solo aplica el árbol y el archivo principal (`applyTree`), **nunca** `filePath` | 8, 11 |
| Binding | `OpenFolderDialog`    | UI → Go    | `() → main.WorkspaceState`. Diálogo nativo; `canceled: true` si se cierra. Abre la carpeta y elige `main.tex` o el primer `.tex` | 8 |
| Binding | `OpenFile`            | UI → Go    | `(path) → main.OpenedFile` = `{ path, content }`. Solo `.tex` dentro de la carpeta abierta. Lo hace documento activo y encola su compilación | 8 |
| Binding | `SaveContent`         | UI → Go    | `(path, content) → main.SaveResult` = `{ saved }`. Responde de inmediato y **no compila**: la UI pide `RequestCompile` tras 3 s sin escribir o con Cmd/Ctrl+S. `path` (absoluto, `.tex`, dentro de la carpeta) viaja con cada guardado. Rechaza la promesa si la ruta no es válida o falla la escritura | 8, 12 |
| Binding | `SaveAsDialog`        | UI → Go    | `(content) → main.WorkspaceState`. Diálogo nativo (filtro `*.tex`; agrega `.tex` si falta). Si el destino está fuera de la carpeta, su directorio pasa a ser la carpeta abierta. Lo hace documento activo y compila | 8 |
| Binding | `SetMainFile`         | UI → Go    | `(path) → main.WorkspaceState`. `.tex` existente dentro de la carpeta, o `""` para quitarlo. Se guarda (ruta relativa) en `<carpeta>/.creative-ink-meta` y se recompila el nuevo objetivo | 10 |
| Binding | `RequestCompile`      | UI → Go    | `() → boolean`: encola una compilación del **objetivo** (principal o activo) sin guardar. Es **el único disparador al editar**: inactividad de 3 s, Cmd/Ctrl+S, botones Guardar y Compilar. `false` solo durante el cierre | 7, 12 |
| Binding | `ConfirmClose`        | UI → Go    | `(saved: boolean) → void`: respuesta a `app:before-close` cuando la UI ya volcó CodeMirror a disco. Si `saved` es `false`, Go pregunta con un diálogo nativo antes de cerrar | 12 |
| Evento  | `compile:started`     | Go → UI    | `main.CompileStartedEvent` = `{ file }`: el **objetivo de compilación** (principal o activo). La UI ignora los eventos de otro objetivo | 8, 10 |
| Evento  | `compile:finished`    | Go → UI    | `main.CompileFinishedEvent` = `{ file, success, logs, durationMs, pdfVersion }`. `pdfVersion` es la instantánea publicada (0 si falló la compilación o la copia). No se emite después del cierre | 8, 13 |
| Evento  | `app:before-close`    | Go → UI    | Sin payload. Lo emite `OnBeforeClose` en el primer intento de cerrar la ventana; la UI hace `waitIdle()` y responde con `ConfirmClose`. Go cierra igual a los 2 s si no hay respuesta | 12 |
| Evento  | `workspace:changed`   | Go → UI    | `main.WorkspaceChangedEvent` = `{ folder }`. Se crearon, borraron o renombraron archivos o carpetas fuera de la app (con debounce de 300 ms y **solo si el árbol visible cambió**). La UI llama a `RefreshWorkspace` si `folder` sigue abierta | 11 |
| HTTP    | `/__preview/document.pdf?v=<pdfVersion>` | WebView → Go | `GET` → instantánea **inmutable** del PDF del objetivo de compilación (`core.PDFSnapshots`; se guardan las 3 últimas). Admite `Range` → **206 Partial Content** (`http.ServeContent`: `Accept-Ranges`, `Content-Length`, `Content-Range`). Sin `v`, sirve la más reciente; una versión desconocida da 404 | 4, 13 |

## 4. Motores LaTeX (requisito de producto)
- **Detección (`internal/system/health.go`):** primero `exec.LookPath` y, si falla, `fallbackDirs` (`/Library/TeX/texbin`, `/opt/homebrew/bin`, `/usr/local/bin`, `/usr/bin`) más `/usr/local/texlive/*/bin/*`. Motivo: en macOS, una app abierta desde el Finder o el Dock **no hereda el PATH de la shell**.
- El runner base es `latexmk`, con el motor como flag: `-pdf` (pdflatex), `-xelatex`, `-lualatex`.
- `internal/system` detecta en `PATH`, y en rutas típicas (`/Library/TeX/texbin`, `/usr/bin`, `/usr/local/texlive/*/bin/*`), qué hay instalado.
- La instalación de lo que falte (MacTeX/BasicTeX y `tlmgr` en macOS; `apt`/`dnf`/`pacman` en Linux) se **propone** al usuario y él la confirma.

## 4.1 Perfil de hardware (Módulo 1, parte 2)
- `internal/system/health.go` usa `runtime.NumCPU()`, `gopsutil/v3/mem` (RAM total, en GB binarios) y `gopsutil/v3/disk` (espacio libre en `/`, en GB decimales como Finder).
- **Tier:** si la RAM es menor a 8 GB → `basic` (solo API en la nube); si es de 8 GB o más → `advanced` (IA local tipo Ollama); si no se pudo leer → `unknown`.
- **Errores:** cada medición es independiente. Si una falla, su campo queda en 0 y se agrega un mensaje en `warnings`; el binding nunca rechaza la promesa por una lectura de hardware.

## 4.2 Editor y autoguardado (Módulos 2 y 4)
- **CodeMirror 6** con un setup mínimo hecho a mano (sin `basicSetup`): números de línea, historial, `defaultKeymap`, `indentWithTab`, `bracketMatching`, modo `stex` (`@codemirror/legacy-modes`) y tema `one-dark`.
- **Dos debounces** que cada tecla reinicia (Sección 12):
  - **1 s → guardar** (`SAVE_DELAY_MS`): el documento se convierte en texto **una sola vez** y se envía con `SaveContent`. Nunca hay más de un guardado en curso; los que llegan mientras tanto se juntan en uno con el contenido más reciente.
  - **3 s → compilar** (`COMPILE_DELAY_MS`): `saveAndCompile()` espera a que el guardado llegue a disco y llama a `RequestCompile()`. Mientras se escribe de corrido, latexmk no se ejecuta.
- **Cmd+S (macOS) / Ctrl+S (Linux y Windows) ya está soportado:** listener global en `App.svelte` con `preventDefault()`. Guarda al instante, sin esperar los debounces, y luego compila. También funciona dentro de CodeMirror (su `keydown` llega a `window`; se quitó el `Mod-s` del keymap del editor para que no haya dos manejadores). Los botones Guardar y Compilar hacen lo mismo. Si el guardado falla, no se compila.
- **Seguridad de datos:** si falla `LoadContent`, el editor **no se monta**, para que el autoguardado no sobrescriba el archivo con un documento vacío.
- **Cierre seguro (`OnBeforeClose`, Sección 12):** protocolo en dos fases sin bloquear (en Windows el hook corre en el hilo de la UI, y `runtime.Quit` vuelve a llamarlo en los tres sistemas):
  1. Primer cierre → `App.beforeClose` lo **previene**, emite `app:before-close` y arma un temporizador de 2 s (`closeFlushTimeout`).
  2. La UI hace `waitIdle()` (vuelca CodeMirror a disco) → `ConfirmClose(saved)` → `closeAllowed` → `runtime.Quit` → `beforeClose` ahora permite cerrar.
  3. Sin respuesta en 2 s, Go cierra igual. Con `saved = false`, un diálogo nativo pregunta "Cerrar sin guardar" / "Cancelar".
  - Un segundo clic en ✕ mientras se vuelca se ignora. `quit` y `confirmQuit` se inyectan para los tests.

## 4.3 Pipeline de compilación (Módulo 3)
- `internal/compiler.CompileLaTeXContext(ctx, workdir, filename)` ejecuta `latexmk -xelatex -interaction=nonstopmode -halt-on-error <filename>` (sin shell). `CompileLaTeX` es el mismo sin cancelación externa.
  - **Resolución de binarios:** `system.FindBinary` encuentra latexmk; su directorio se antepone al `PATH` del subproceso para que latexmk encuentre xelatex y biber aunque la app se abra desde el Finder.
  - **Seguridad:** el nombre de archivo no puede tener directorios ni empezar con `-`. Las ejecuciones van en serie (mutex) y tienen un límite de 60 s.
  - **Logs:** stdout y stderr van al mismo buffer. Si falla, los bloques `! ...` hasta `l.<n>` (más su línea de continuación) salen primero de `<doc>.log` y, si el `.log` no tiene errores, de stdout. Después va la cola de la salida (máximo 32 KB). Un bloque sin `l.<n>` se corta a las 6 líneas o en el siguiente `! `.
- **Flujo asíncrono:** autoguardado → `SaveContent` → `SaveDocument` → responde `{saved}` → `Scheduler.Request()` → goroutine: `compile:started` → `CompileLaTeXContext` → `compile:finished` → `App.svelte` actualiza `pdfTimestamp` → `Preview` recarga el canvas.
  - **Scheduler:** un solo worker. Si llegan peticiones mientras compila, se juntan en **una** compilación más, que lee el archivo más reciente del disco: tres guardados rápidos producen como mucho dos compilaciones. Un `panic` se recupera y se informa como fallo. Al cerrar la app, `OnShutdown` → `Close(3s)` cancela latexmk y descarta lo pendiente.
  - Los eventos se emiten por `App.emit`, inyectable: `runtime.EventsEmit` hace `log.Fatalf` si el contexto es `nil`, así que los tests nunca lo llaman.
- **Visor PDF:** PDF.js (`pdfjs-dist` 6.5) en `<canvas>`, igual en macOS y Linux (se descartó el iframe porque WebKitGTK no muestra PDFs).
  - **Worker local** con `pdf.worker.min.mjs?url`, que Vite empaqueta como asset; Wails lo sirve con MIME `text/javascript`. Funciona sin conexión.
  - **Carga por rangos (Sección 13, sustituye a la carga completa):** `loadPdf(url)` → `getDocument({ url, rangeChunkSize: 65536, disableAutoFetch: true, disableStream: true, worker })`. PDF.js descarga solo los bloques de la página que dibuja.
    - `disableAutoFetch` **solo funciona con `disableStream: true`** (documentación de pdfjs-dist 6); con `disableStream: false` se descargaría el archivo entero por streaming.
    - **Transporte (verificado en el código):** en macOS y Linux la app corre en `wails://`, que no es http(s), así que PDF.js usa XMLHttpRequest (`PDFNetworkStream`); en Windows (`http://wails.localhost`) usa `fetch`. El puente de Wails reenvía `Range` y devuelve `206` en macOS (`NSHTTPURLResponse`), en Windows (WebView2) y en Linux con WebKitGTK ≥ 2.36 (tag `webkit2_41`). **En Linux sin tags (`webkit2_legacy`) no pasan cabeceras:** PDF.js no ve `Accept-Ranges` y descarga el archivo entero, sin errores.
    - **Instantáneas inmutables:** al cargar por rangos, PDF.js lee trozos *después* de abrir el documento, y latexmk reescribe `<doc>.pdf` en el mismo archivo en la siguiente compilación (se mezclarían bytes de dos versiones). Al terminar con éxito, el worker copia el PDF en streaming (sin RAM extra) a `core.PDFSnapshots` (carpeta temporal privada, 3 versiones, se borra al cerrar). La copia ocurre en la misma goroutine que latexmk, así que nunca coincide con una compilación. `compile:finished` lleva `pdfVersion` y la UI carga `?v=<pdfVersion>`.
  - **Recarga:** `pdfVersion` cambia → `$effect` → `reload()`. Cada carga recibe un número de secuencia y las viejas se descartan. La página se dibuja en un canvas fuera de pantalla y se copia al visible de una vez (sin parpadeo). Si algo falla, el canvas conserva el último PDF válido.
  - Al redimensionar el panel, `ResizeObserver` vuelve a dibujar con un debounce de 150 ms. Se tiene en cuenta `devicePixelRatio` para pantallas Retina.
  - **Ciclo de vida (pdfjs-dist 6, verificado en el código fuente):**
    - `PDFDocumentProxy` **no tiene `destroy()`** en la v6. Un documento se libera con `doc.loadingTask.destroy()` (es asíncrono); `releasePdf()` lo envuelve sin lanzar errores.
    - **Un solo `PDFWorker` compartido**, que se pasa en `getDocument({ worker })`. Sin él, cada `getDocument` crea su propio Web Worker. Con él, `loadingTask.destroy()` libera el documento (páginas, fuentes y estado en el worker) pero conserva el hilo.
    - `disableWorkerRendering: true`, explícito: la v6 intentaría un segundo *renderer worker* por documento.
    - **Propiedad:** solo existe un documento vivo (`pdfDoc`). Uno nuevo se promueve (y se libera el anterior) o se libera en el `finally`, sin excepción.
    - Antes de liberar un documento, se cancela su render (`renderTask.cancel()`), como exige PDF.js.
  - **Retención de página:** `currentPageNumber` (por defecto 1) se conserva entre compilaciones y se ajusta a `numPages` si el PDF nuevo tiene menos páginas. Todavía no hay botones de navegación.
  - **Si WebKitGTK antiguo falla con la build moderna:** usar `pdfjs-dist/legacy/build/pdf.mjs`.
- **Test de integración opcional:** `CREATIVE_INK_LATEX_TEST=1 go test ./internal/compiler -run Integration -v` (lo ejecuta el programador).

## 4.3.1 Carpetas, archivos y explorador (Sección 8)
- **Rutas absolutas en el puente.** Go valida cada una con `core.ResolveInFolder`: dentro de la carpeta abierta y siguiendo symlinks, para que un enlace no pueda escapar. Así, un guardado tardío de una carpeta anterior se **rechaza** en lugar de escribirse en la nueva.
- **Estado en Go:** `App.folder` y `App.docPath` protegidos por `sync.RWMutex`. El compilador toma el documento activo al **empezar** cada compilación (`compilingDoc`, solo en la goroutine del worker) y los eventos llevan ese `file`.
- **Cambio de archivo en el frontend:** cambia `workspace.currentFilePath` → `$effect` en `Editor` → editor de solo lectura → `waitIdle()` (guarda lo pendiente en el archivo **anterior**) → `OpenFile` → `view.setState()`. `setState` no dispara los update listeners (sin autoguardado) y empieza un historial de deshacer nuevo. Si no se pudo guardar, **no cambia de archivo**.
- **Abrir carpeta y Guardar como** también llaman antes a `waitIdle()`. Guardar como usa `adopt(path)`: mismo texto, archivo nuevo, sin recargar (se conservan cursor e historial).
- **Diálogos inyectables** (`pickFolder`, `pickSavePath`), igual que `emit`, para testear sin el runtime de Wails.
- **Decisión de diseño:** un solo archivo activo a la vez, sin pestañas.

## 4.3.2 Proyectos de varios archivos: archivo principal (Sección 10)
- **Objetivo de compilación** = archivo principal si el proyecto tiene uno **y existe en disco**; si no, el documento activo. `App.compileTarget()` lo decide para el Scheduler (`compilingDoc` se toma al empezar cada compilación), para `preview.go` (qué PDF se sirve) y para el `file` de los eventos.
- **Persistencia:** `<carpeta>/.creative-ink-meta` (JSON: `{ version, mainFile }`), con `mainFile` **relativo y con `/`**, así el proyecto se puede mover o compartir entre sistemas. Es un archivo oculto, así que no aparece en el explorador. Se recarga cada vez que cambia la carpeta abierta. Si es inválido (fuera de la carpeta, no `.tex`, borrado), se ignora y se compila el activo.
- **Guardar cualquier `.tex`** de la carpeta recompila el principal (el Scheduler junta las ráfagas).
- **Frontend:** `compileTarget()` en `workspace.svelte.js` (`mainFilePath || currentFilePath`) es la clave del `Preview` y el filtro de los eventos `compile:*`. Al saltar entre capítulos, el objetivo no cambia: el PDF, la página y el log se conservan.
- **UI:** insignia `★ MAIN` en el árbol. Al pasar el ratón por un `.tex` aparece ☆ ("Establecer como principal"); en el principal, ★ lo quita. La vista previa muestra qué PDF se ve (`· main.pdf`).

## 4.3.3 Explorador reactivo a cambios externos (Sección 11)
- **`core.FolderWatcher`** (`fsnotify` v1.9, sin Wails) vigila la carpeta abierta y sus subcarpetas con los mismos filtros que `ScanTree` (`isIgnoredEntry`). `fsnotify` no es recursivo: las carpetas nuevas se añaden al vuelo.
- **Solo cambios de estructura:** `Create`, `Remove` y `Rename`. Se ignoran las escrituras (autoguardado, latexmk), los auxiliares de LaTeX, el `.tmp` del guardado atómico y los archivos ocultos (`.creative-ink-meta`).
- **Debounce de 300 ms** (`WatchDebounce`) en la goroutine del watcher, que además recupera los panics. Si hay *overflow* de eventos, se refresca igualmente.
- **Sin eventos falsos:** `App.onFolderChanged` vuelve a escanear y compara `core.TreeSignature` con la anterior. Solo emite `workspace:changed` si el árbol visible cambió (kqueue informa `Remove`+`Create` cuando el guardado atómico reemplaza `main.tex`).
- **Ciclo de vida:** `App.restartWatcher()` al cambiar de carpeta (vigila siempre la carpeta *actual*, aunque dos cambios coincidan), `stopWatching()` en `OnShutdown`. Los callbacks de un watcher viejo se descartan. En los tests, `watchFolder` se inyecta.
- **Límites:** como mucho 256 carpetas vigiladas (`maxWatchedDirs`) y 6 niveles. En macOS, kqueue abre un descriptor por carpeta **y por archivo** vigilado; Go ≥ 1.19 sube el límite blando de descriptores al duro al arrancar.
- **Frontend:** `listenWorkspaceEvents()` (en `actions.js`, se da de baja en `onDestroy`) → `refreshWorkspace()`, con un solo refresco en curso y los demás juntados en uno. `applyTree()` no cambia el archivo abierto: un refresco durante un cambio de archivo no puede devolver el editor al archivo anterior.

## 4.4 Deuda técnica conocida (análisis del 2026-11-05)
1. ~~**Guardar y compilar están acoplados en `SaveContent`.**~~ **Resuelto el 2026-11-06:** `SaveContent` responde de inmediato y la compilación va por el `Scheduler` con eventos (ver §4.3).
2. ~~**Se compila con cada pausa de 1 s.**~~ **Resuelto el 2026-11-07:** guardar (1 s) y compilar (3 s) tienen debounces separados; `SaveContent` ya no compila. **Cmd+S / Ctrl+S ya está soportado** en la UI: guarda al instante y compila (ver §4.2).
3. ~~**No se guarda al cerrar la ventana.**~~ **Resuelto el 2026-11-07:** `OnBeforeClose` con protocolo en dos fases (`app:before-close` → `waitIdle()` → `ConfirmClose`), un límite de 2 s y un diálogo nativo si el guardado falló (ver §4.2).
4. ~~**La carpeta *inicial* depende del directorio de trabajo** (`./workspace`).~~ **Resuelto el 2026-11-07:** la app abre un repositorio central en `~/Documents/My Creative Ink Repository`, una ruta **absoluta** construida con `os.UserHomeDir()` y `filepath.Join` (válida en macOS y Linux). Se crea con `MkdirAll(0755)` si no existe y lleva un `main.tex` inicial si no hay ningún `.tex`, sin sobrescribir nunca. Funciona igual desde `wails dev` y desde el Finder. **Ampliado el mismo día:** la ruta depende del SO y de la elección del primer arranque ("Solo para mí" o "Para todos los usuarios"); en Linux se respeta XDG (`~/Documentos`) y en Windows, Known Folders (ver §1). *Pendiente menor:* recordar la última carpeta abierta y permitir cambiar el repositorio desde la UI (hoy hay que borrar `settings.json`).
5. ~~**El PDF se carga entero en memoria (`fetch` → `data`).**~~ **Resuelto el 2026-11-07:** PDF.js carga por **rangos HTTP** (`getDocument({ url, rangeChunkSize, disableAutoFetch, disableStream: true })`) desde instantáneas inmutables (`?v=<pdfVersion>`) que sirve `preview.go` con `206 Partial Content`. *Pendiente:* medirlo con un PDF grande en la app (ver §4.5) y, en Linux, compilar con `-tags webkit2_41` para tener rangos.
6. ~~**Proyectos de varios archivos:** se compila el `.tex` **activo**.~~ **Resuelto el 2026-11-07:** cada carpeta puede tener un **archivo principal** (`SetMainFile`, guardado en `.creative-ink-meta`). Se compila y se muestra ese archivo mientras se edita cualquier capítulo (ver §4.3.2). Texto original: Si editas un capítulo que se incluye con `\input`, se compila el capítulo y no el documento principal. Falta definir un "archivo principal" por carpeta.
7. ~~**El árbol no se actualiza solo** (no hay `fsnotify`).~~ **Resuelto el 2026-11-07:** `core.FolderWatcher` (fsnotify) avisa con `workspace:changed`, con debounce y solo si el árbol visible cambió, y la UI refresca el explorador sola (ver §4.3.3). El botón Actualizar se mantiene como respaldo.
8. **Escalabilidad de la compilación automática y la vista previa con documentos grandes** (tesis, libros, muchas imágenes o TikZ). Analizado el 2026-11-07, **sin implementar todavía**: ver §4.5.

## 4.5 Escalabilidad con documentos grandes (análisis del 2026-11-07, pendiente)

> Solo análisis: no hay nada implementado. Antes de decidir, hay que **medir** con un documento de prueba representativo (ver al final).

### Problema
La compilación automática (3 s sin escribir) funciona bien con documentos pequeños. Con documentos grandes (cientos de páginas, decenas de imágenes, figuras TikZ, bibliografía con biber) puede convertirse en un **cuello de botella** de CPU, disco y memoria, y también en la vista previa (decodificar y dibujar el PDF).

### Cómo se comporta hoy (verificado en el código)
| Pieza | Comportamiento actual | Por qué escala mal |
|-------|-----------------------|--------------------|
| Disparador (`Editor.svelte`) | `RequestCompile` tras 3 s fijos de inactividad (`COMPILE_DELAY_MS`) | Si una compilación tarda 40 s, cada pausa de 3 s al escribir encola otra. El usuario nunca deja de tener latexmk corriendo |
| Scheduler (`scheduler.go`) | Un solo worker; una petición nueva **no cancela** la compilación en curso, solo deja una pendiente (`pending = true`) | Encadena compilaciones casi sin pausa: la CPU queda ocupada de forma continua (ventiladores, batería) y el resultado ya está viejo cuando llega |
| latexmk (`orchestrator.go`) | Compilación completa con `-xelatex`; latexmk repite pasadas para referencias, índices y biber | xelatex es de un solo hilo y vuelve a procesar todas las imágenes y las figuras TikZ en cada pasada |
| Límite de tiempo | `compileTimeout = 60 s`, fijo | Un documento grande puede superarlo **siempre**: la compilación se corta y el usuario solo ve "superó el límite" |
| Prioridad del proceso | latexmk corre con prioridad normal | Compite con la UI (WebView, CodeMirror) por la CPU |
| Vista previa (`pdf.js`, `Preview.svelte`) | ~~`fetch` del PDF completo~~ → desde la Sección 13, **carga por rangos** de una instantánea | Ya no se copia el PDF entero en RAM (deuda #5 resuelta). Siguen pendientes: copiar la instantánea en disco en cada compilación (coste de E/S proporcional al tamaño) y decodificar las imágenes de la página visible en el hilo principal (`disableWorkerRendering: true`) |
| Recarga sin cambios | `pdfTimestamp = Date.now()` en cada éxito, aunque latexmk dijera "All targets up-to-date" | Se vuelve a descargar y analizar un PDF idéntico |

Ya existe un dato útil: `compile:finished` incluye `durationMs`, todavía sin usar.

### Opciones para más adelante (no decididas)
1. **Disparador adaptativo.** Ajustar el debounce según `durationMs` (por ejemplo, `max(3 s, 2 × última duración)`). Por encima de un umbral (por ejemplo 15 s), **desactivar la compilación automática** y dejar solo Cmd/Ctrl+S, avisando en la UI ("Documento grande: compila con ⌘S"). Podría guardarse por proyecto en `.creative-ink-meta`.
2. **Cancelar compilaciones viejas.** Si llega una petición y la compilación en curso supera cierto tiempo, cancelarla y empezar de nuevo. *Riesgo:* un latexmk cortado a mitad puede dejar `.aux` inconsistentes; latexmk suele recuperarse en la pasada siguiente, pero hay que probarlo.
3. **Límite de tiempo configurable** por proyecto en lugar de 60 s fijos, o un límite más amplio cuando la compilación la pide el usuario (Cmd+S) frente a la automática.
4. **Modo borrador para la vista previa automática** (requiere decisión del usuario, porque cambia los flags de latexmk; ver la skill `latex-toolchain-orchestrator`):
   - `graphicx` con la opción `draft` en las compilaciones automáticas (marcos en lugar de imágenes), y compilación completa con Cmd+S. Ejemplo: `latexmk -usepretex='\PassOptionsToPackage{draft}{graphicx}'`.
   - Con archivo principal, `\includeonly{<capítulo activo>}` para compilar solo el capítulo que se edita.
   - Externalizar TikZ (`\usetikzlibrary{external}`) para que las figuras se compilen una vez y se reutilicen.
5. **Prioridad baja para latexmk:** `nice` en macOS/Linux y `BELOW_NORMAL_PRIORITY_CLASS` en Windows, para que la UI siga fluida mientras compila.
6. **Valores por defecto según el hardware:** usar el tier de `SystemHealthCheck` (`basic` / `advanced`) y `cpuCores` para elegir debounce y modo (borrador por defecto en equipos `basic`).
7. **Vista previa más barata:**
   - No recargar si el PDF no cambió: incluir la fecha de modificación (o un hash) del PDF en `compile:finished`, o usar `ETag`/`If-None-Match` en `preview.go`.
   - Cargar por rangos (`PDFDataRangeTransport`, deuda #5) para no copiar el PDF entero.
   - Dibujar en el worker (`GlobalWorkerOptions.rendererSrc` + `OffscreenCanvas`) para que las imágenes pesadas no bloqueen el hilo principal.
8. **Visibilidad:** emitir el progreso de latexmk (pasada actual, biber) como eventos, para que una compilación larga no parezca colgada.

### Cómo medir antes de decidir
- **Documento de prueba:** unas 200 páginas, unas 50 imágenes PNG/JPEG de varios MB, algunas figuras TikZ, bibliografía con biber y archivo principal con capítulos en `\input`.
- **Qué medir:**
  - `durationMs` de la primera compilación y de las incrementales (cambiar una palabra en un capítulo);
  - uso de CPU mientras se escribe con pausas de 3–5 s;
  - tiempo de `loadPdf` y de `renderPage`;
  - memoria del WebView tras 20 compilaciones.
- **Dónde:** en un equipo `basic` (8 GB o menos) y en uno `advanced`, en macOS y en Linux.

## 5. Skills / conocimiento requerido

| Área | Qué hay que dominar |
|------|---------------------|
| Wails v2 | `wails.Run`, `options.App`, `Bind`, ciclo de vida (`startup(ctx)`, `shutdown`), `runtime.EventsEmit/On`, generación de `wailsjs/`, `wails doctor` |
| Go | `context`, `os/exec` con cancelación, streaming de stdout/stderr, `fsnotify` (futuro), errores envueltos, `internal/` |
| Svelte 5 + Vite | Runes (`$state`, `$derived`, `$effect`), eventos `onclick`, `mount()`, import de bindings desde `../wailsjs/go/main/App.js` |
| LaTeX toolchain | `latexmk` (`-pdf/-xelatex/-lualatex`, `-interaction=nonstopmode`, `-synctex=1`, `-outdir`), parseo del `.log`, `kpsewhich`, `tlmgr` |
| Multiplataforma | macOS: WKWebView y Xcode CLT. Linux: WebKitGTK (`libwebkit2gtk-4.0` o `4.1` con el tag `webkit2_41`), XDG user dirs. Windows: WebView2, Known Folders, instalador NSIS. Archivos `_<os>.go` con build tags; verificar con `GOOS=<os> go vet ./...` |

## 6. Comandos que ejecuta el PROGRAMADOR (nunca Claude)

| Acción | Comando (desde `creative-ink/`) |
|--------|---------------------------------|
| Desarrollo con hot reload | `wails dev` |
| Build de producción | `wails build` |
| Dependencias del frontend | `cd frontend && npm install` |
| Dependencias de Go | `go mod tidy` |
