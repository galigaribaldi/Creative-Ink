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
    │   │   └── watcher.go        ← FolderWatcher (fsnotify): cambios externos con debounce; TreeSignature
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
| Binding | `SaveContent`         | UI → Go    | `(path, content) → main.SaveResult` = `{ saved }`. Responde de inmediato. `path` (absoluto, `.tex`, dentro de la carpeta) viaja con cada guardado. Compila si `path` es el documento activo o, **si hay archivo principal, al guardar cualquier `.tex`** (un capítulo con `\input` cambia el PDF del principal). Rechaza la promesa si la ruta no es válida o falla la escritura | 8 |
| Binding | `SaveAsDialog`        | UI → Go    | `(content) → main.WorkspaceState`. Diálogo nativo (filtro `*.tex`; agrega `.tex` si falta). Si el destino está fuera de la carpeta, su directorio pasa a ser la carpeta abierta. Lo hace documento activo y compila | 8 |
| Binding | `SetMainFile`         | UI → Go    | `(path) → main.WorkspaceState`. `.tex` existente dentro de la carpeta, o `""` para quitarlo. Se guarda (ruta relativa) en `<carpeta>/.creative-ink-meta` y se recompila el nuevo objetivo | 10 |
| Binding | `RequestCompile`      | UI → Go    | `() → boolean`: encola una compilación del documento activo **sin guardar**; `false` solo durante el cierre | 7 |
| Evento  | `compile:started`     | Go → UI    | `main.CompileStartedEvent` = `{ file }`: el **objetivo de compilación** (principal o activo). La UI ignora los eventos de otro objetivo | 8, 10 |
| Evento  | `compile:finished`    | Go → UI    | `main.CompileFinishedEvent` = `{ file, success, logs, durationMs }`. No se emite después del cierre | 8 |
| Evento  | `workspace:changed`   | Go → UI    | `main.WorkspaceChangedEvent` = `{ folder }`. Se crearon, borraron o renombraron archivos o carpetas fuera de la app (con debounce de 300 ms y **solo si el árbol visible cambió**). La UI llama a `RefreshWorkspace` si `folder` sigue abierta | 11 |
| HTTP    | `/__preview/document.pdf` | WebView → Go | `GET` → PDF del **objetivo de compilación** (el del archivo principal si existe; si no, el del activo) (`Cache-Control: no-store`, admite Range; 404 si todavía no existe). La UI agrega `?t=<timestamp>` para forzar la recarga | 4 |

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
- **Debounce:** cada cambio solo reinicia un `setTimeout` de 1000 ms; al vencer, el documento se convierte en texto **una sola vez** y se envía con `SaveContent`. Nunca hay más de un guardado en curso: los que llegan mientras tanto se juntan en un único guardado posterior con el contenido más reciente. Cmd/Ctrl+S guarda de inmediato.
- **Seguridad de datos:** si falla `LoadContent`, el editor **no se monta**, para que el autoguardado no sobrescriba el archivo con un documento vacío.
- **Pendiente:** el guardado al cerrar la ventana (`OnBeforeClose`). La ruta del documento ya no depende del directorio de trabajo (ver deuda técnica #4).

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
  - `loadPdf(url)` hace `fetch(url, {cache:'no-store'})` → `getDocument({ data })`. No se usa `getDocument(url)` porque la capa de red de PDF.js (Range/streaming) no es fiable sobre el esquema `wails://`.
  - **Recarga:** `pdfTimestamp` cambia → `$effect` → `reload()`. Cada carga recibe un número de secuencia y las viejas se descartan. La página se dibuja en un canvas fuera de pantalla y se copia al visible de una vez (sin parpadeo). Si algo falla, el canvas conserva el último PDF válido.
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
2. **Se compila con cada pausa de 1 s.** Convendría un debounce propio para la compilación (por ejemplo 2–3 s), además de Cmd+S para compilar al instante.
3. **No se guarda al cerrar la ventana:** falta `OnBeforeClose` para hacer un flush en Go.
4. ~~**La carpeta *inicial* depende del directorio de trabajo** (`./workspace`).~~ **Resuelto el 2026-11-07:** la app abre un repositorio central en `~/Documents/My Creative Ink Repository`, una ruta **absoluta** construida con `os.UserHomeDir()` y `filepath.Join` (válida en macOS y Linux). Se crea con `MkdirAll(0755)` si no existe y lleva un `main.tex` inicial si no hay ningún `.tex`, sin sobrescribir nunca. Funciona igual desde `wails dev` y desde el Finder. **Ampliado el mismo día:** la ruta depende del SO y de la elección del primer arranque ("Solo para mí" o "Para todos los usuarios"); en Linux se respeta XDG (`~/Documentos`) y en Windows, Known Folders (ver §1). *Pendiente menor:* recordar la última carpeta abierta y permitir cambiar el repositorio desde la UI (hoy hay que borrar `settings.json`).
5. **El PDF se carga entero en memoria (`fetch` → `data`).** Basta para documentos normales. Para PDFs de cientos de MB habría que usar `PDFDataRangeTransport`.
6. ~~**Proyectos de varios archivos:** se compila el `.tex` **activo**.~~ **Resuelto el 2026-11-07:** cada carpeta puede tener un **archivo principal** (`SetMainFile`, guardado en `.creative-ink-meta`). Se compila y se muestra ese archivo mientras se edita cualquier capítulo (ver §4.3.2). Texto original: Si editas un capítulo que se incluye con `\input`, se compila el capítulo y no el documento principal. Falta definir un "archivo principal" por carpeta.
7. ~~**El árbol no se actualiza solo** (no hay `fsnotify`).~~ **Resuelto el 2026-11-07:** `core.FolderWatcher` (fsnotify) avisa con `workspace:changed`, con debounce y solo si el árbol visible cambió, y la UI refresca el explorador sola (ver §4.3.3). El botón Actualizar se mantiene como respaldo.

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
