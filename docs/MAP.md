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
    ├── workspace/main.tex        ← carpeta abierta por defecto al iniciar (ignorada por git); se crea un documento inicial si falta
    ├── go.mod
    ├── internal/                 ← lógica Go no importable desde fuera del módulo
    │   ├── compiler/
    │   │   ├── orchestrator.go   ← CompileLaTeXContext: latexmk -xelatex con timeout/cancelación, PATH de TeX, errores del .log
    │   │   └── scheduler.go      ← Scheduler: compilación en segundo plano, una a la vez, juntando ráfagas
    │   ├── core/                 ← dominio: proyectos, archivos, (futuro) plugins
    │   │   ├── file_manager.go   ← FileManager: guardado atómico (tmp + rename) y carga de documentos
    │   │   └── workspace.go      ← ScanTree (árbol con límites), ResolveInFolder (rutas dentro de la carpeta), DefaultTeXFile
    │   └── system/               ← entorno: health check de LaTeX y detección/instalación de motores
    │       └── health.go         ← HealthStatus: herramientas de LaTeX y perfil de hardware
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
        │   │   ├── HealthPanel.svelte ← modal "Estado del sistema"
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
| Binding | `SystemHealthCheck`   | UI → Go    | `() → system.HealthStatus` = `{ hasXeLaTeX, hasLatexmk, hasBiber: boolean; totalRAMGB, freeDiskSpaceGB: number; cpuCores: number; hardwareTier: string; hardwareTierId: 'basic'\|'advanced'\|'unknown'; warnings: string[] }` | 2 |
| Binding | `GetWorkspace`        | UI → Go    | `() → main.WorkspaceState` = `{ canceled, folderPath, filePath, tree: core.FileNode, truncated }`. Carpeta abierta (al inicio, `./workspace`), documento activo y árbol | 8 |
| Binding | `RefreshWorkspace`    | UI → Go    | `() → main.WorkspaceState`: vuelve a escanear la carpeta abierta | 8 |
| Binding | `OpenFolderDialog`    | UI → Go    | `() → main.WorkspaceState`. Diálogo nativo; `canceled: true` si se cierra. Abre la carpeta y elige `main.tex` o el primer `.tex` | 8 |
| Binding | `OpenFile`            | UI → Go    | `(path) → main.OpenedFile` = `{ path, content }`. Solo `.tex` dentro de la carpeta abierta. Lo hace documento activo y encola su compilación | 8 |
| Binding | `SaveContent`         | UI → Go    | `(path, content) → main.SaveResult` = `{ saved }`. Responde de inmediato. `path` (absoluto, `.tex`, dentro de la carpeta) viaja con cada guardado; **solo compila si `path` es el documento activo**. Rechaza la promesa si la ruta no es válida o falla la escritura | 8 |
| Binding | `SaveAsDialog`        | UI → Go    | `(content) → main.WorkspaceState`. Diálogo nativo (filtro `*.tex`; agrega `.tex` si falta). Si el destino está fuera de la carpeta, su directorio pasa a ser la carpeta abierta. Lo hace documento activo y compila | 8 |
| Binding | `RequestCompile`      | UI → Go    | `() → boolean`: encola una compilación del documento activo **sin guardar**; `false` solo durante el cierre | 7 |
| Evento  | `compile:started`     | Go → UI    | `main.CompileStartedEvent` = `{ file }`. La UI ignora los eventos de un archivo que ya no está activo | 8 |
| Evento  | `compile:finished`    | Go → UI    | `main.CompileFinishedEvent` = `{ file, success, logs, durationMs }`. No se emite después del cierre | 8 |
| HTTP    | `/__preview/document.pdf` | WebView → Go | `GET` → PDF del **documento activo** (`Cache-Control: no-store`, admite Range; 404 si todavía no existe). La UI agrega `?t=<timestamp>` para forzar la recarga | 4 |

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
- **Pendiente:** el guardado al cerrar la ventana (`OnBeforeClose`) y la ruta del documento cuando la app se abre desde el Finder (el directorio de trabajo es `/`).

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

## 4.4 Deuda técnica conocida (análisis del 2026-11-05)
1. ~~**Guardar y compilar están acoplados en `SaveContent`.**~~ **Resuelto el 2026-11-06:** `SaveContent` responde de inmediato y la compilación va por el `Scheduler` con eventos (ver §4.3).
2. **Se compila con cada pausa de 1 s.** Convendría un debounce propio para la compilación (por ejemplo 2–3 s), además de Cmd+S para compilar al instante.
3. **No se guarda al cerrar la ventana:** falta `OnBeforeClose` para hacer un flush en Go.
4. **La carpeta *inicial* depende del directorio de trabajo** (`./workspace`). Desde el Finder ese directorio es `/`. Ya se puede abrir cualquier carpeta; falta recordar la última abierta en lugar de usar `./workspace`.
5. **El PDF se carga entero en memoria (`fetch` → `data`).** Basta para documentos normales. Para PDFs de cientos de MB habría que usar `PDFDataRangeTransport`.
6. **Proyectos de varios archivos:** se compila el `.tex` **activo**. Si editas un capítulo que se incluye con `\input`, se compila el capítulo y no el documento principal. Falta definir un "archivo principal" por carpeta.
7. **El árbol no se actualiza solo** (no hay `fsnotify`): se refresca al abrir carpeta, al usar Guardar como o con el botón Actualizar.

## 5. Skills / conocimiento requerido

| Área | Qué hay que dominar |
|------|---------------------|
| Wails v2 | `wails.Run`, `options.App`, `Bind`, ciclo de vida (`startup(ctx)`, `shutdown`), `runtime.EventsEmit/On`, generación de `wailsjs/`, `wails doctor` |
| Go | `context`, `os/exec` con cancelación, streaming de stdout/stderr, `fsnotify` (futuro), errores envueltos, `internal/` |
| Svelte 5 + Vite | Runes (`$state`, `$derived`, `$effect`), eventos `onclick`, `mount()`, import de bindings desde `../wailsjs/go/main/App.js` |
| LaTeX toolchain | `latexmk` (`-pdf/-xelatex/-lualatex`, `-interaction=nonstopmode`, `-synctex=1`, `-outdir`), parseo del `.log`, `kpsewhich`, `tlmgr` |
| Multiplataforma | macOS: WKWebView y Xcode CLT. Linux: WebKitGTK (`libwebkit2gtk-4.0` o `4.1` con el tag `webkit2_41`) |

## 6. Comandos que ejecuta el PROGRAMADOR (nunca Claude)

| Acción | Comando (desde `creative-ink/`) |
|--------|---------------------------------|
| Desarrollo con hot reload | `wails dev` |
| Build de producción | `wails build` |
| Dependencias del frontend | `cd frontend && npm install` |
| Dependencias de Go | `go mod tidy` |
