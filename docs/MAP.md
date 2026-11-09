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
    ├── menu.go                   ← menú nativo (solo macOS): Archivo, Edición (portapapeles), Buscar, Ver, Ventana → evento menu:command
    ├── preview.go                ← middleware del AssetServer: sirve el PDF en /__preview/document.pdf
    ├── wails.json                ← configuración del proyecto Wails
    ├── go.mod
    ├── internal/                 ← lógica Go no importable desde fuera del módulo
    │   ├── ci/
    │   │   └── ci.go             ← comandos internos `ci` de la terminal híbrida (traducción del Makefile, sin shell ni make)
    │   ├── compiler/
    │   │   ├── orchestrator.go   ← CompileLaTeXContext: latexmk -xelatex -synctex=1 con timeout/cancelación, PATH de TeX, errores del .log
    │   │   ├── synctex.go        ← SyncTeXForward/Backward: CLI synctex view/edit y parseo de su salida
    │   │   ├── manual.go         ← RunPasses: secuencias de herramientas (xelatex, bibtex…) con el candado de compilación
    │   │   └── scheduler.go      ← Scheduler: compilación en segundo plano, una a la vez, juntando ráfagas
    │   ├── core/                 ← dominio: proyectos, archivos, (futuro) plugins
    │   │   ├── file_manager.go   ← FileManager: guardado atómico (tmp + rename) y carga de documentos
    │   │   ├── workspace.go      ← ScanTree, ResolveInFolder, DefaultTeXFile, PrepareRepository
    │   │   ├── settings.go       ← <UserConfigDir>/creative-ink/settings.json: repositorio, autoCompile y recientes (UpdateSettings no pisa otros campos)
    │   │   ├── repository.go     ← estructura del repositorio, Slugify, CreateProject, CreateQuickDoc, StaleQuickDocs
    │   │   ├── hide_{windows,other}.go ← ocultar .creative-ink en Windows (atributo hidden)
    │   │   ├── project_meta.go   ← <carpeta>/.creative-ink-meta: archivo principal del proyecto (ruta relativa)
    │   │   ├── watcher.go        ← FolderWatcher (fsnotify): cambios externos con debounce; TreeSignature
    │   │   ├── pdf_snapshots.go  ← PDFSnapshots: copias inmutables y versionadas del PDF para la vista previa
    │   │   └── terminal*.go      ← ExecuteCommand(Context): sh -c / cmd /c, 30 s, sin stdin, grupo de procesos, salida limitada
    │   └── system/               ← entorno: health check de LaTeX y detección/instalación de motores
    │       ├── health.go         ← HealthStatus: herramientas de LaTeX, hardware y repositorio
    │       ├── platform.go       ← RepositoryOptions por SO, parser de XDG user-dirs; RepositoryName = My-Creative-Ink-Repository
    │       ├── reveal.go         ← RevealInFileManager (open / xdg-open / explorer, sin shell)
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
        │   │   ├── BottomPanel.svelte ← panel inferior con pestañas "Errores LaTeX" | "Terminal" (siempre montado)
        │   │   ├── CompileLog.svelte  ← contenido de la pestaña de errores de LaTeX
        │   │   ├── TerminalPanel.svelte ← terminal integrada: historial, entrada, ↑/↓, Ctrl+C
        │   │   ├── HealthPanel.svelte ← modal "Estado del sistema" (incluye "Sistema y repositorio")
        │   │   ├── RepositorySetup.svelte ← primer arranque: "Solo para mí" / "Para todos los usuarios"
        │   │   ├── StartScreen.svelte ← cada sesión: Nuevo proyecto, Documento rápido, proyectos, recientes y aviso de documentos rápidos antiguos
        │   │   └── Preview.svelte     ← PDF.js en <canvas>, doble buffer, conserva la página al recargar
        │   ├── lib/
        │   │   ├── ui.svelte.js          ← estado visual ($state): sidebarOpen, activeTool, healthOpen. Nunca va a Go
        │   │   ├── compilation.svelte.js ← reflejo de los eventos compile:* (status, logs, pdfTimestamp)
        │   │   ├── workspace.svelte.js   ← reflejo de Go: currentFolderPath, currentFilePath, fileTree
        │   │   ├── actions.js            ← acciones globales y runCommand (atajos y menú nativo); registro de los dos paneles del editor
        │   │   ├── languages.js          ← resaltado por tipo de archivo (stex, BibTeX propio, lua, python, shell, yaml)
        │   │   ├── terminal.svelte.js    ← entradas y historial de la terminal (sobreviven al cambio de pestaña)
        │   │   ├── synctex.svelte.js     ← SyncTeX: highlight (Editor → PDF) y reveal (PDF → Editor)
        │   │   └── pdf.js        ← configuración de PDF.js (worker local) y loadPdf(url)
        │   └── style.css         ← estilos globales mínimos
        └── wailsjs/              ← GENERADO por Wails: no editar a mano
            ├── go/main/App.js    ← bindings de los métodos de App
            ├── go/models.ts      ← tipos TS de los structs de Go (p. ej. system.HealthStatus)
            └── runtime/          ← EventsOn, EventsEmit, etc.
```

**Fuera del repo, en el sistema operativo del usuario (Sección 18):**

```
~/Documents/My-Creative-Ink-Repository/   ← repositorio central = LANZADOR (nunca se abre como proyecto)
├── .creative-ink/                        ← oculta (en Windows con el atributo hidden): datos del editor
│   ├── config.json                       ← valores por defecto del repositorio (servicio de configuración, paso 2)
│   ├── plugins/                          ← plugins del usuario (solo en repositorios personales)
│   ├── templates/                        ← plantillas de "Nuevo proyecto" (paso 2+)
│   └── guide/                            ← guía de `ci` autogenerada y compilada (paso 4)
├── tmp-Workspaces/                       ← documentos rápidos de un solo archivo
│   ├── unnamed-tmp1/main.tex             ← uno por subcarpeta: no mezclan auxiliares
│   └── unnamed-tmp2/main.tex
└── Tesis-de-Maestria/                    ← un proyecto = una carpeta (nombre normalizado con "-")
    ├── .creative-ink-meta                ← archivo principal (main.tex)
    ├── main.tex · capitulos/ · figuras/ · referencias.bib
```
- **Convención de nombres:** todo lo que crea el editor usa `-` en lugar de espacios (`My-Creative-Ink-Repository`, `Tesis-de-Maestria`) y sin acentos en las carpetas (`core.Slugify`), para evitar problemas con herramientas que pasan por la shell (minted, externalización de TikZ) y con actualizaciones. El título del documento conserva el nombre original (escapado con `core.EscapeTeX`).
- **Migración:** si `settings.json` apunta a `My Creative Ink Repository`, al arrancar se renombra a `My-Creative-Ink-Repository` (y se actualizan `repositoryPath` y los recientes). Si ya existen las dos carpetas, no se mueve nada y la pantalla de inicio lo avisa.
- **Plugins por usuario:** en un repositorio personal, `<repo>/.creative-ink/plugins`; en uno compartido, `<UserConfigDir>/creative-ink/plugins` (nunca donde otras cuentas pueden escribir).
- **Documentos rápidos antiguos:** al empezar cada sesión, la pantalla de inicio lista los que llevan **más de 8 h** sin modificarse (`core.StaleQuickDocs`) y ofrece abrir `tmp-Workspaces` en el Finder. **El editor nunca los borra** (por ahora no hay opción automática).
- **En `settings.json` (por usuario, fuera del repositorio):** la ruta del repositorio, su ámbito, `autoCompile` y los **recientes** (`recentFolders`, 10 como máximo).

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
- El frontend **nunca** toca el disco ni lanza procesos. Todo pasa por Go. La única puerta a comandos arbitrarios es `RunTerminalCommand` (terminal integrada), y solo con lo que el usuario escribe en ella (ver §4.3.4).
- `frontend/wailsjs/` se regenera con `wails dev` o `wails build` (el programador los ejecuta).

## 3. Contrato IPC (vivo: actualizar con cada método o evento nuevo)

| Tipo    | Nombre                | Dirección  | Firma / payload | Sección |
|---------|-----------------------|------------|-----------------|---------|
| Binding | `SystemHealthCheck`   | UI → Go    | `() → system.HealthStatus` = `{ hasXeLaTeX, hasLatexmk, hasBiber: boolean; totalRAMGB, freeDiskSpaceGB: number; cpuCores: number; hardwareTier: string; hardwareTierId: 'basic'\|'advanced'\|'unknown'; repository: system.RepositoryOptions; warnings: string[] }`. Incluye el SO y el repositorio (actual y rutas propuestas) | 2, 9 |
| Binding | `GetWorkspace`        | UI → Go    | `() → main.WorkspaceState` = `{ needsSetup, needsStart, repositoryPath, canceled, folderPath, filePath, texFilePath, mainFilePath, autoCompile, tree: core.FileNode, truncated }`. `filePath` es el documento activo (cualquier archivo de texto); **`texFilePath`** es el último `.tex` activo: el objetivo de compilación si no hay archivo principal (Sección 19). **`needsStart: true`** con el repositorio listo y sin proyecto abierto: la UI muestra la pantalla de inicio. `autoCompile` es el ajuste de `ci config autocompile` (por defecto `true`). `mainFilePath` es el archivo principal del proyecto (`""` si no hay, o si se borró). Carpeta abierta, documento activo y árbol. **`needsSetup: true`** en el primer arranque, sin crear nada en disco. Desde la Sección 18 **no abre la raíz del repositorio**: sin proyecto, `needsStart: true` | 9, 18 |
| Binding | `GetRepositoryOptions` | UI → Go   | `() → system.RepositoryOptions` = `{ os, osName, user, shared: RepositoryOption; warnings: string[]; currentScope, currentPath }`, con `RepositoryOption` = `{ scope, path, available, reason }`. Rutas propuestas para este SO | 9 |
| Binding | `SetupRepository`     | UI → Go    | `(scope: 'user'\|'shared') → main.WorkspaceState`. Crea el repositorio y su estructura (`.creative-ink/`, `tmp-Workspaces/`) en la ruta que calcula Go y guarda la elección. Devuelve `needsStart` (no abre ninguna carpeta). Rechaza ámbitos desconocidos o no disponibles | 9 |
| Binding | `GetStartScreen`      | UI → Go    | `() → main.StartScreen` = `{ repositoryPath, projects, quickDocs, staleQuickDocs: core.ProjectInfo[]; staleAfterHours; recent: string[]; pluginsPath; warnings }`, con `ProjectInfo` = `{ name, path, mainFile, modified }` | 18 |
| Binding | `CreateProject`       | UI → Go    | `(name) → main.WorkspaceState`. Crea `<repo>/<Slugify(name)>/` desde la plantilla con `main.tex` como principal, lo abre y lo compila. Error si ya existe o el nombre está reservado | 18 |
| Binding | `CreateQuickDocument` | UI → Go    | `() → main.WorkspaceState`. Crea `tmp-Workspaces/unnamed-tmp<N>/main.tex` (N siguiente libre), lo abre y lo compila | 18 |
| Binding | `OpenProject`         | UI → Go    | `(path) → main.WorkspaceState`. Solo carpetas **dentro del repositorio** (no la raíz, `.creative-ink` ni `tmp-Workspaces`) o **recientes** | 18 |
| Binding | `RevealQuickDocuments`| UI → Go    | `() → void`: abre `tmp-Workspaces` en el gestor de archivos (`open` / `xdg-open` / `explorer`, sin shell) | 18 |
| Binding | `NewFileDialog`       | UI → Go    | `() → main.WorkspaceState`. **⌘N / Ctrl+N "Nuevo archivo…"**: diálogo nativo en la carpeta abierta; crea un archivo **vacío** de texto (`.tex` si no lleva extensión) con el nombre normalizado (`core.Slugify`, `Capítulo Dos` → `Capitulo-Dos.tex`) y lo hace documento activo, sin compilar. Debe quedar dentro de la carpeta abierta y **nunca reemplaza** un archivo (`core.ErrFileExists`). `canceled: true` si se cierra. Sin carpeta → `ErrNoFolder` (la UI crea entonces un documento rápido) | 19 |
| Binding | `LoadFile`            | UI → Go    | `(path) → main.OpenedFile`. Lee un archivo de texto de la carpeta para el **segundo panel** del editor; a diferencia de `OpenFile`, **no** cambia el documento activo ni compila | 19 |
| Binding | `RefreshWorkspace`    | UI → Go    | `() → main.WorkspaceState`: vuelve a escanear la carpeta abierta. La UI solo aplica el árbol y el archivo principal (`applyTree`), **nunca** `filePath` | 8, 11 |
| Binding | `OpenFolderDialog`    | UI → Go    | `() → main.WorkspaceState`. Diálogo nativo; `canceled: true` si se cierra. Abre la carpeta y elige `main.tex` o el primer `.tex` | 8 |
| Binding | `OpenFile`            | UI → Go    | `(path) → main.OpenedFile` = `{ path, content }`. Cualquier **archivo de texto** dentro de la carpeta abierta (`core.IsEditableFile`: `.tex`, `.bib`, `.sty`, `.cls`, `.txt`, `.md`…). Rechaza los de más de 8 MB y los binarios o no UTF-8 (`core.ErrNotUTF8`: guardarlos los corrompería). Lo hace documento activo y encola la compilación del **objetivo** (un `.bib` conserva el último `.tex`) | 8, 19 |
| Binding | `SaveContent`         | UI → Go    | `(path, content) → main.SaveResult` = `{ saved }`. Responde de inmediato y **no compila**: la UI pide `RequestCompile` tras 3 s sin escribir o con Cmd/Ctrl+S. `path` (absoluto, archivo de texto, dentro de la carpeta) viaja con cada guardado. Rechaza la promesa si la ruta no es válida o falla la escritura | 8, 12 |
| Binding | `SaveAsDialog`        | UI → Go    | `(content) → main.WorkspaceState`. Diálogo nativo; si falta la extensión, usa la del documento activo. Solo archivos de texto (`core.ErrNotEditable`). Guarda el **panel principal**. Si el destino está fuera de la carpeta, su directorio pasa a ser la carpeta abierta. Lo hace documento activo y compila | 8 |
| Binding | `SetMainFile`         | UI → Go    | `(path) → main.WorkspaceState`. `.tex` existente dentro de la carpeta, o `""` para quitarlo. Se guarda (ruta relativa) en `<carpeta>/.creative-ink-meta` y se recompila el nuevo objetivo | 10 |
| Binding | `RequestCompile`      | UI → Go    | `() → boolean`: encola una compilación del **objetivo** (principal o activo) sin guardar. Es **el único disparador al editar**: inactividad de 3 s, Cmd/Ctrl+S, botones Guardar y Compilar. `false` solo durante el cierre | 7, 12 |
| Binding | `SyncTexForward`      | UI → Go    | `(line, file) → main.SyncTexForwardResult` = `{ found, page, x, y, width, height, reason }`. Dónde aparece la línea de `file` (`.tex` del proyecto) en el PDF del objetivo de compilación, en puntos PDF desde la esquina superior izquierda. Sin datos → `found: false` y `reason`, nunca error | 16 |
| Binding | `SyncTexBackward`     | UI → Go    | `(page, x, y) → main.SyncTexBackwardResult` = `{ found, file, line, reason }`. Qué `.tex` del proyecto y qué línea (desde 1) generaron ese punto. Los archivos fuera del proyecto (p. ej. `article.cls`) dan `found: false` | 16 |
| Binding | `RunTerminalCommand`  | UI → Go    | `(cmd) → main.TerminalResult` = `{ command, folder, output, exitCode, timedOut, canceled, durationMs }`. **Terminal híbrida:** si la primera palabra es `ci`, la resuelve Go por dentro (§4.3.6, sin shell, sin carpeta obligatoria); si no, ejecuta `cmd` con `sh -c` (macOS/Linux) o `cmd /c` (Windows) en la carpeta abierta, sin stdin, con límite de 30 s y PATH ampliado con TeX y Homebrew. Un código distinto de 0, el timeout o la cancelación son resultados normales; rechaza si no hay carpeta, si el comando está vacío o si ya hay otro en curso. **Ejecución arbitraria por diseño:** solo con texto que escribió el usuario | 15 |
| Binding | `CancelTerminalCommand` | UI → Go  | `() → void`: detiene el comando en curso (mata su grupo de procesos) | 15 |
| Binding | `ConfirmClose`        | UI → Go    | `(saved: boolean) → void`: respuesta a `app:before-close` cuando la UI ya volcó CodeMirror a disco. Si `saved` es `false`, Go pregunta con un diálogo nativo antes de cerrar | 12 |
| Evento  | `compile:started`     | Go → UI    | `main.CompileStartedEvent` = `{ file }`: el **objetivo de compilación** (principal o activo). La UI ignora los eventos de otro objetivo | 8, 10 |
| Evento  | `compile:finished`    | Go → UI    | `main.CompileFinishedEvent` = `{ file, success, logs, durationMs, pdfVersion, pdfModTime }`. `pdfVersion` es la instantánea publicada (0 si falló la compilación o la copia); **si el PDF no cambió** (mismo `ModTime` y tamaño), Go **reutiliza la versión anterior** sin copiarla. `pdfModTime` es el `ModTime` del PDF en ms Unix. No se emite después del cierre | 8, 13, 14 |
| Evento  | `app:before-close`    | Go → UI    | Sin payload. Lo emite `OnBeforeClose` en el primer intento de cerrar la ventana; la UI hace `waitIdle()` y responde con `ConfirmClose`. Go cierra igual a los 2 s si no hay respuesta | 12 |
| Evento  | `menu:command`        | Go → UI    | `main.MenuCommandEvent` = `{ id }`. Clic en el menú nativo (solo macOS): `new-file`, `new-quick-doc`, `open-folder`, `start`, `save`, `save-as`, `find`, `find-next`, `find-previous`, `select-next`, `goto-line`, `toggle-comment`, `toggle-sidebar`, `toggle-terminal`, `split-editor`. La UI lo ejecuta con `runCommand`, igual que los atajos | 19 |
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
- **CodeMirror 6** con un setup mínimo hecho a mano (sin `basicSetup`): números de línea, historial, `defaultKeymap`, `indentWithTab`, `bracketMatching`, resaltado según el archivo (`lib/languages.js`) y tema `one-dark`.
- **Archivos de texto (Sección 19):** se abren y editan `.tex`, `.bib`, `.sty`, `.cls`, `.bst`, `.txt`, `.md`, `.csv`, `.json`, `.yaml`, `.lua`, `.py`, `.sh`, `Makefile`… (`core.IsEditableFile`); el resto sale atenuado en el explorador. Los auxiliares (`.aux`, `.log`…) siguen ocultos. Go rechaza archivos de más de 8 MB, binarios o **no UTF-8** (un `.bib` antiguo en Latin-1 se corrompería al autoguardar: hay que convertirlo antes). Resaltado: `stex` para la familia TeX, un modo **BibTeX** propio, y `lua`/`python`/`shell`/`yaml` de `legacy-modes`; el resto, texto plano.
  - **Objetivo de compilación:** el archivo principal (★) o, si no hay, el **último `.tex`** abierto en el panel principal (`App.texDoc` / `WorkspaceState.texFilePath`). Abrir `referencias.bib` no cambia la vista previa, y guardarlo recompila el documento. Un `.bib` nunca es principal ni usa SyncTeX.
- **Comandos de edición (todos de CodeMirror, `@codemirror/search` + `@codemirror/commands`):** deshacer/rehacer (⌘Z, ⇧⌘Z), **buscar y reemplazar** (⌘F; panel arriba, en español), siguiente/anterior (⌘G, ⇧⌘G), **ir a la línea** (⌥⌘G), siguiente coincidencia (⌘D), resaltar las demás apariciones de la selección, **varios cursores** (⌥-clic) y selección en columna (⇧⌥-arrastrar), comentar/descomentar con `%` (⌘/), mover y copiar líneas (⌥↑/↓, ⇧⌥↑/↓), borrar línea (⇧⌘K), sangrar (⌘] / ⌘[ y Tab). ⌘-clic sigue siendo SyncTeX en los `.tex`. En Linux y Windows, ⌘ = Ctrl y ⌥ = Alt.
- **Atajos de la app** (`App.svelte` → `runCommand` en `actions.js`): ⌘N nuevo archivo en el proyecto (sin proyecto, documento rápido), ⇧⌘N documento rápido, ⌘O abrir carpeta, ⌘S guardar y compilar, ⇧⌘S guardar como, ⌘B explorador, ⌘J / Ctrl+` terminal, ⌘\ dividir el editor.
- **Menú nativo (`menu.go`, solo macOS):** Archivo, Edición (rol nativo de Wails: deshacer, cortar, **copiar, pegar**, seleccionar todo), Buscar, Ver y Ventana. **Sin él, ⌘C/⌘X/⌘V no funcionan en WKWebView** (Wails no crea ningún menú si `options.App.Menu` es nil). Los clics emiten `menu:command` y la UI ejecuta el mismo `runCommand` que los atajos. Los aceleradores del menú no duplican los atajos: WKWebView entrega cada tecla primero a la página y solo pasa al menú las que nadie atendió (`preventDefault`). En Linux y Windows no hay menú (sería una barra dentro de la ventana) y los WebViews ya gestionan el portapapeles. Las etiquetas del menú Edición las pone Wails (no se pueden traducir).
- **Editor dividido (⌘\, botón en la ActionBar, menú Ver):** un segundo panel junto al principal para ver dos archivos lado a lado (por ejemplo, un capítulo y `referencias.bib`); el editor ocupa 2/3 del ancho y la vista previa 1/3. El panel principal es el documento activo de Go (`OpenFile`: decide qué se compila; Guardar como actúa sobre él); el segundo solo lee y guarda en su sitio (`LoadFile` + `SaveContent`). El explorador abre en el panel con el foco (marcado con una línea de acento). **Un mismo archivo nunca está en los dos paneles** (sus autoguardados se pisarían): elegirlo enfoca el panel que ya lo tiene. Guardar con ⌘S, cambiar de carpeta y cerrar la app vuelcan los dos paneles; cerrar la división guarda antes el segundo. *Pendiente:* divisor arrastrable y recordar la división entre sesiones.
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
- `internal/compiler.CompileLaTeXContext(ctx, workdir, filename)` ejecuta `latexmk -xelatex -interaction=nonstopmode -halt-on-error -synctex=1 <filename>` (sin shell). `-synctex=1` (desde la Sección 16) escribe `<doc>.synctex.gz` para SyncTeX. `CompileLaTeX` es el mismo sin cancelación externa.
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

## 4.3.4 Terminal integrada (Sección 15)
- **Backend:** `core.ExecuteCommand(cmd, workdir)` / `ExecuteCommandContext(ctx, …, extraEnv)`:
  - **`sh -c` en macOS/Linux y `cmd /c` en Windows.** Es la **única excepción** a la regla de "nunca usar un shell" de la skill `latex-toolchain-orchestrator`: una terminal existe precisamente para ejecutar lo que escribe el usuario. latexmk y cualquier otro proceso interno siguen sin shell.
  - Límite de **30 s**; un timeout o una cancelación matan el **grupo de procesos** entero (`Setpgid` + `kill(-pid)`), no solo el `sh`. En Windows, `cmd.exe` se lanza sin ventana (`CREATE_NO_WINDOW`) y con la línea de órdenes sin reescapar (`CmdLine`); *límite conocido:* allí los nietos pueden sobrevivir (haría falta un Job Object).
  - **No interactivo:** sin stdin (EOF inmediato), `TERM=dumb`, `NO_COLOR`, `PAGER`/`GIT_PAGER=cat`, `GIT_TERMINAL_PROMPT=0` y `GIT_EDITOR=true` (un `git commit` sin `-m` se aborta en lugar de quedarse esperando). `sudo` y los editores no funcionan.
  - **PATH:** `system.ToolPATH()` añade al final las rutas de TeX y Homebrew que faltan (una app abierta desde el Finder no las tiene), así `tlmgr` y `git` de Homebrew se encuentran.
  - Salida combinada (stdout+stderr) limitada a **256 KB**, con un aviso de truncado.
- **Fachada:** `RunTerminalCommand` ejecuta en la carpeta abierta, **un comando a la vez** (`termMu`), y devuelve `TerminalResult`. `CancelTerminalCommand` lo detiene; `OnShutdown` también. El binding puede tardar hasta 30 s: no bloquea otros bindings (Wails atiende cada llamada en su propia goroutine), y la UI muestra "Ejecutando…".
- **Frontend:** `BottomPanel.svelte` bajo el editor, con pestañas **"Errores LaTeX" | "Terminal"**, siempre montado. Un error de compilación lo abre en su pestaña solo si estaba cerrado (si no, un punto rojo en la pestaña), y lo cierra al compilar bien si lo había abierto él. `TerminalPanel.svelte`: Enter ejecuta, ↑/↓ recorre el historial, Ctrl+C o "Detener" cancela, `clear`/`cls` limpia la vista. Atajos: **Ctrl+`** o **⌘J / Ctrl+J**, y botón en la ActionBar.
- **Seguridad:** `RunTerminalCommand` es **ejecución arbitraria de comandos por diseño**. Si alguna vez se ejecutara JavaScript ajeno en el WebView (por ejemplo, por una vulnerabilidad de PDF.js con un PDF malicioso), podría usarla. Reglas: la UI **solo** le pasa lo que el usuario escribió en la terminal, **nunca** texto derivado de documentos, logs o nombres de archivo; mantener `pdfjs-dist` actualizado; y no renderizar HTML que no sea nuestro (`{@html}` está prohibido con contenido externo).
- **Pendiente:** salida en streaming (eventos) en lugar de esperar al final, PTY real y `xterm.js` (requiere instalar dependencias), y Job Objects en Windows.

## 4.3.5 SyncTeX: navegación bidireccional (Sección 16)
- **Compilación:** latexmk lleva ahora `-synctex=1` (escribe `<doc>.synctex.gz` junto al PDF). `<doc>.synctex(busy)`, que existe solo mientras TeX escribe, se filtra del explorador y del watcher.
- **Backend (`internal/compiler/synctex.go`):** `system.FindBinary("synctex")`, sin shell, con límite de 5 s y en la carpeta del PDF.
  - Hacia delante: `synctex view -i <línea>:0:<archivo> -o <pdf>`. Toma el **primer** registro; `h`/`v` son el borde izquierdo y la línea base, `W`/`H` el ancho y el alto, así que la caja empieza en `v − H`.
  - Hacia atrás: `synctex edit -o <página>:<x>:<y>:<pdf>`. `Input` puede venir relativo a la carpeta del PDF: se convierte en absoluto, se limpia y se valida dentro del proyecto.
  - **Errores silenciosos:** sin `.synctex.gz`, sin binario, con la salida vacía o con un código distinto de 0 → `found: false` (se registra en el log), nunca un error de JavaScript. La UI muestra un aviso discreto en la barra de la vista previa ("compila con ⌘S…").
  - Se usa el PDF del **objetivo de compilación**: con archivo principal, hacia delante desde un capítulo salta al lugar correcto del documento completo.
- **Hacia delante (código → PDF):** ⌘-clic (Ctrl-clic en Linux y Windows) en una línea del editor, o **⌘⌥J / Ctrl+Alt+J** desde la línea del cursor → `sync.highlight` → `Preview` cambia a esa página si hace falta, dibuja un recuadro ámbar que se desvanece (1,8 s) y lo centra con un scroll.
- **Hacia atrás (PDF → código):** doble clic en el `<canvas>`. La posición en CSS se divide por `rendered.cssScale` (píxeles CSS por punto PDF, guardado en cada render) → `SyncTexBackward` → `sync.reveal` + `workspace.currentFilePath`. Si es otro archivo, el `Editor` lo carga (guardando antes el actual) y luego selecciona la línea y la centra.
- **Limitaciones:** las coordenadas suponen un `MediaBox` con origen 0,0 (lo normal en LaTeX); usa los datos de la **última** compilación, así que tras editar puede desviarse unas líneas hasta compilar de nuevo. Solo se muestra una página: la vista previa salta de página, no hace scroll continuo.

## 4.3.6 Terminal híbrida: comandos internos `ci` (Sección 17)
La terminal integrada es **híbrida**. `RunTerminalCommand` mira la primera palabra de la línea (sin contar los espacios iniciales):
- **`ci …` (palabra reservada, *Creative Ink*):** la resuelve `internal/ci` en Go, **sin shell del sistema ni `make`**. Las herramientas (`xelatex`, `bibtex`, `latexmk`) se ejecutan directamente con `os/exec`, resueltas con `system.FindBinary` y con `system.ToolPATH()`; los archivos se borran con `os.Remove` y `filepath.Glob`. Funciona aunque no haya una carpeta abierta (por ejemplo, `ci help` o `ci config`).
- **Cualquier otra línea:** se ejecuta en la shell del sistema (`sh -c` / `cmd /c`), como en §4.3.4.

Los comandos se aplican al **archivo principal (★)** o, si no hay, al `.tex` abierto, y se ejecutan en la carpeta de ese archivo. Son la traducción del Makefile de la tesis:

| Comando | Equivale a (Makefile) | Qué hace |
|---------|-----------------------|----------|
| `ci help` | `make ayuda` | Lista los comandos |
| `ci config autocompile [on\|off]` | — | Muestra o guarda `autoCompile` en `settings.json` (`core.UpdateSettings`) |
| `ci compile rapido` | `make rapido` | `xelatex -interaction=nonstopmode -synctex=1 <doc>.tex` (una pasada) |
| `ci compile borrador` | `make borrador` | Dos pasadas de `xelatex … "\def\borrador{}\input{<doc>}"` (la primera puede fallar). Requiere que el preámbulo use `\borrador` |
| `ci compile solo <capítulo>` | `make solo CAP=…` | Dos pasadas de `xelatex -jobname=<doc>_<carpeta> "\def\SoloCapitulo{<capítulo>}\input{<doc>}"` → `<doc>_<carpeta>.pdf` |
| `ci compile completo` | `make pdf` | XeLaTeX → BibTeX → XeLaTeX → XeLaTeX (las XeLaTeX intermedias pueden fallar, como `\|\| true`) |
| `ci clean tikz` | `make tikz-clean` | Borra `*.pdf`, `*.md5`, `*.log` y `*.dpth` de `Figures/TiKz_Libraries/externalized/` |
| `ci clean all` | `make limpiar-todo` | `latexmk -C <doc>.tex` y los restos (`.brf`, `.run.xml`, `.idx`, `.ilg`, `.ind`, `.blg`, `.bbl`, `.xdv`). Borra también el PDF |

**Diferencias deliberadas con el Makefile:**
- **`ci compile solo` usa `-jobname`.** El Makefile compila sobre `tesis.pdf` y luego lo renombra, así que **borra el PDF completo**. Con `-jobname`, el capítulo sale en un PDF aparte, con sus propios `.aux`, y el documento completo no se toca.
- **Sin `--shell-escape`**, que el Makefile usa para externalizar TikZ: permitiría al documento ejecutar comandos del sistema. Si la caché de TikZ ya existe funciona igual; si no, haría falta una opción explícita (pendiente de decisión del usuario).
- **Se añade `-synctex=1`** a cada XeLaTeX para que SyncTeX siga funcionando tras una compilación manual.
- **`<capítulo>` se valida** (`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`, sin `..`) porque se inserta en código TeX y en `-jobname`.
- **Sin `ci check` ni `ci formatear`:** dependen de scripts propios del proyecto (`tools/*.py`). El núcleo solo incluye comandos nativos de LaTeX, del sistema o de Go; esas funciones llegarán como **plugins** (§4.6).

**Integración con el resto de la app:**
- **Mismo candado que latexmk:** `ci compile` y `ci clean` corren con `compiler.RunPasses` / `WithCompileLock`, así que nunca se solapan con la compilación automática (que esperará). La copia de la instantánea del PDF también se hace con el candado tomado, tanto desde el worker como desde `ci compile`.
- **Límites:** `ci compile` y `ci clean` tienen 10 min (`ManualBuildTimeout`, no los 30 s de la shell). "Detener" o Ctrl+C los cancelan.
- **Vista previa:** `rapido`, `borrador` y `completo` emiten `compile:started` / `compile:finished` (con `pdfVersion` y la duración real), así que la vista previa se actualiza sin cambios en Svelte y el disparador adaptativo aprende el coste real. Un fallo también emite `compile:finished` (`success: false`). `solo` no toca la vista previa (genera otro PDF).
- **Salida compacta:** una línea por pasada (`▶ …`, `✓ 3.2 s` / `✗ falló`), con los errores de TeX o las últimas 15 líneas si algo falla.

**Pendiente:** la UI todavía **no respeta** `autoCompile` (el temporizador vive en `Editor.svelte`; se expone en `WorkspaceState.autoCompile` para conectarlo). Tampoco hay autocompletado de `ci` en el panel.

**Seguridad (riesgo previo, no introducido por `ci`):** latexmk lee el **`.latexmkrc` del proyecto**, que es código Perl. Abrir una carpeta ajena con la compilación automática activa equivale a ejecutar ese código. No se desactiva (`-norc`) porque hay proyectos, como la tesis, que dependen de él. Opción futura: confirmar antes de compilar una carpeta nueva que tenga `.latexmkrc`.

## 4.6 Configuración, confianza y extensibilidad (diseño acordado el 2026-11-08)

### Decisiones del usuario (2026-11-08)
1. **Carpeta oculta del editor:** `<repo>/.creative-ink/` (config, plugins, plantillas, guía).
2. **Abrir un proyecto = abrir su carpeta.** La raíz del repositorio es solo el lanzador (pantalla de inicio).
3. **`tmp-Workspaces`:** nunca se borra nada automáticamente. Al empezar cada sesión se avisa de los documentos de más de 8 h; el usuario decide. Cada documento rápido va en su subcarpeta `unnamed-tmp1`, `unnamed-tmp2`…
4. **Nombres normalizados con `-`:** `My-Creative-Ink-Repository` y todas las carpetas que crea el editor.
5. **Plugins solo por usuario**, nunca en repositorios compartidos.
6. **La guía de `ci`** se genera y se compila automáticamente.
7. **Confianza:** se define en la instalación y se gestiona desde la terminal (`ci trust …`). Los proyectos creados por el editor son de confianza.

### Plan y estado
- [x] **Paso 1 — Estructura (Sección 18):** `.creative-ink/`, `tmp-Workspaces/`, pantalla de inicio (Nuevo proyecto, Documento rápido, proyectos, recientes, Abrir carpeta), plantilla de proyecto, migración del nombre y aviso de documentos rápidos antiguos.
- [ ] **Paso 2 — Servicio de configuración por capas** (por defecto < repositorio `.creative-ink/config.json` < proyecto `.creative-ink-meta`; lo sensible solo desde el `settings.json` del usuario), `config:changed`, `ci config list|get|set` y `autoCompile` respetado por la UI y por Go (`RequestCompile(trigger)`).
- [ ] **Paso 3 — Confianza:** política elegida en la instalación, `ci trust status|add|remove`, modo restringido (`latexmk -norc`) para carpetas ajenas con `.latexmkrc`, hash del archivo.
- [ ] **Paso 4 — Registro de comandos `ci`** + guía autogenerada (`.creative-ink/guide/ci-guia.tex` → PDF) + `ci guide` + salida en streaming.
- [ ] **Paso 5 — Host de plugins** (JSON-RPC por stdio, manifiesto con contribuciones y capacidades).

### Análisis original (propuesta del 2026-11-08)

> **Solo análisis, nada implementado.** Resuelve tres problemas abiertos con una sola base: (1) `ci config autocompile` se guarda pero la UI no lo respeta; (2) el `.latexmkrc` de un proyecto ajeno es código Perl que latexmk ejecuta; (3) los plugins futuros necesitarán aportar comandos y ajustes.

### Diagnóstico
- **No hay una fuente única de configuración.** Hoy hay ajustes en tres sitios sin un contrato común: `settings.json` (usuario: repositorio, `autoCompile`), `.creative-ink-meta` (proyecto: archivo principal) y constantes del código (motor `-xelatex`, límites de tiempo, debounces). La terminal escribe en un archivo que la UI no escucha.
- **Go no puede hacer cumplir `autoCompile`.** `RequestCompile()` no distingue un disparo automático de ⌘S.
- **No hay noción de confianza.** Cualquier carpeta abierta se compila de inmediato con su `.latexmkrc`; lo mismo pasará con los plugins y con un futuro `shell-escape`.
- **`ci` tiene sus comandos fijos en un `switch`.** Un plugin no podría añadir `ci check bib` sin tocar el núcleo, y la ayuda no se puede generar.
- **El requisito del MVP "motor seleccionable" (pdflatex/xelatex/lualatex) sigue sin implementar** y necesita esta misma base.

### Propuesta en 4 fases
**Fase 1 — Servicio de configuración (base de todo lo demás).**
- `internal/config`, sin Wails: un **registro de claves tipadas** declaradas en código (`id`, tipo, valor por defecto, ámbitos permitidos, descripción, si requiere confianza). Claves iniciales: `compile.auto` (bool), `compile.engine` (`xelatex`\|`pdflatex`\|`lualatex`), `compile.autoDelayMs`, `compile.shellEscape` (requiere confianza).
- **Capas con precedencia:** valores por defecto < **usuario** (`settings.json`) < **proyecto** (`<carpeta>/.creative-ink/settings.json`) < sesión (memoria). Es el modelo de VS Code (*User* / *Workspace*).
- **Una sola API para todos:** bindings `GetConfig()` / `SetConfig(key, value, scope)` y el evento `config:changed {key, value, scope}`. La terminal (`ci config list | get <k> | set <k> <v> [--user|--project]`), la UI y los plugins pasan por el mismo camino; nadie lee `settings.json` directamente.
- **`autoCompile` con dos cerrojos:** la UI refleja `config:changed` en un store y no programa el temporizador; además Go rechaza las peticiones automáticas: `RequestCompile(trigger)` con `trigger: "auto" | "manual"` (cambio de contrato). Así un plugin o un frontend con fallos no puede saltárselo.
- **Recomendación:** `compile.auto` por **proyecto** (la tesis pesada sí, un documento corto no), con el valor del usuario como respaldo.

**Fase 2 — Confianza de carpetas (*workspace trust*).**
- **Almacén de confianza** del usuario (en `settings.json`): ruta real de la carpeta + **hash del `.latexmkrc`**. Si alguien modifica el archivo (por ejemplo, tras un `git pull`), se vuelve a preguntar.
- **Carpeta sin confianza y con `latexmkrc`/`.latexmkrc`:** se compila en **modo restringido** con `latexmk -norc` (más los rc del sistema y del usuario de forma explícita, con `-r`), sin compilación automática hasta decidir, y `compile.shellEscape` bloqueado. Una franja discreta en la UI ofrece "Confiar en esta carpeta"; en la terminal, `ci trust` / `ci trust status` / `ci untrust`.
- **Sin `.latexmkrc`:** no se pregunta nada (no hay código del proyecto que ejecutar).
- La misma confianza se aplicará a los plugins que pertenezcan al proyecto.

**Fase 3 — Registro de comandos `ci`.**
- Cada comando se registra con su nombre, uso, descripción, argumentos (una lista cerrada o un validador), si es largo y si **requiere confianza**. `ci help` se genera del registro, y un binding `ListTerminalCommands()` permitirá autocompletar en el panel.
- **Salida en streaming:** evento `terminal:output {runId, chunk}` en lugar de devolver todo al final. Hace falta para builds largos y para plugins.

**Fase 4 — Host de plugins (requiere la decisión pendiente del modelo de plugins).**
- **Recomendación inicial (se mantiene):** procesos externos que hablan JSON-RPC por stdin/stdout, al estilo LSP. Son aislados, pueden escribirse en cualquier lenguaje y se pueden matar con timeout.
- Un **manifiesto** declara sus **contribuciones** (comandos `ci <plugin> …` o subcomandos como `ci check bib`, claves de configuración `plugin.<id>.*`) y sus **capacidades** (leer o escribir dentro del proyecto, ejecutar herramientas, red). El host las hace cumplir y pide confianza.
- `ci check` y `ci formatear` (los scripts `tools/*.py` de la tesis) serían el primer plugin de ejemplo.

### Decisiones que necesita el usuario
1. **Ámbito de `compile.auto`:** por proyecto con respaldo del usuario (recomendado) o solo por usuario.
2. **Ubicación de la configuración del proyecto:** carpeta `<proyecto>/.creative-ink/` (recomendado; deja espacio para plugins y absorbería `.creative-ink-meta`) o ampliar `.creative-ink-meta`.
3. **Carpeta sin confianza con `.latexmkrc`:** modo restringido con `-norc` y sin compilación automática hasta confiar (recomendado), o solo avisar.
4. **Modelo de plugins:** confirmar procesos externos con JSON-RPC por stdio.
5. **Orden de implementación:** Fase 1 → 2 → 3 → 4 (recomendado: la 1 desbloquea `autoCompile` y el motor seleccionable).

## 4.4 Deuda técnica conocida (análisis del 2026-11-05)
1. ~~**Guardar y compilar están acoplados en `SaveContent`.**~~ **Resuelto el 2026-11-06:** `SaveContent` responde de inmediato y la compilación va por el `Scheduler` con eventos (ver §4.3).
2. ~~**Se compila con cada pausa de 1 s.**~~ **Resuelto el 2026-11-07:** guardar (1 s) y compilar (3 s) tienen debounces separados; `SaveContent` ya no compila. **Cmd+S / Ctrl+S ya está soportado** en la UI: guarda al instante y compila (ver §4.2).
3. ~~**No se guarda al cerrar la ventana.**~~ **Resuelto el 2026-11-07:** `OnBeforeClose` con protocolo en dos fases (`app:before-close` → `waitIdle()` → `ConfirmClose`), un límite de 2 s y un diálogo nativo si el guardado falló (ver §4.2).
4. ~~**La carpeta *inicial* depende del directorio de trabajo** (`./workspace`).~~ **Resuelto el 2026-11-07:** la app abre un repositorio central en `~/Documents/My Creative Ink Repository`, una ruta **absoluta** construida con `os.UserHomeDir()` y `filepath.Join` (válida en macOS y Linux). Se crea con `MkdirAll(0755)` si no existe y lleva un `main.tex` inicial si no hay ningún `.tex`, sin sobrescribir nunca. Funciona igual desde `wails dev` y desde el Finder. **Ampliado el mismo día:** la ruta depende del SO y de la elección del primer arranque ("Solo para mí" o "Para todos los usuarios"); en Linux se respeta XDG (`~/Documentos`) y en Windows, Known Folders (ver §1). *Pendiente menor:* recordar la última carpeta abierta y permitir cambiar el repositorio desde la UI (hoy hay que borrar `settings.json`).
5. ~~**El PDF se carga entero en memoria (`fetch` → `data`).**~~ **Resuelto el 2026-11-07:** PDF.js carga por **rangos HTTP** (`getDocument({ url, rangeChunkSize, disableAutoFetch, disableStream: true })`) desde instantáneas inmutables (`?v=<pdfVersion>`) que sirve `preview.go` con `206 Partial Content`. *Pendiente:* medirlo con un PDF grande en la app (ver §4.5) y, en Linux, compilar con `-tags webkit2_41` para tener rangos.
6. ~~**Proyectos de varios archivos:** se compila el `.tex` **activo**.~~ **Resuelto el 2026-11-07:** cada carpeta puede tener un **archivo principal** (`SetMainFile`, guardado en `.creative-ink-meta`). Se compila y se muestra ese archivo mientras se edita cualquier capítulo (ver §4.3.2). Texto original: Si editas un capítulo que se incluye con `\input`, se compila el capítulo y no el documento principal. Falta definir un "archivo principal" por carpeta.
7. ~~**El árbol no se actualiza solo** (no hay `fsnotify`).~~ **Resuelto el 2026-11-07:** `core.FolderWatcher` (fsnotify) avisa con `workspace:changed`, con debounce y solo si el árbol visible cambió, y la UI refresca el explorador sola (ver §4.3.3). El botón Actualizar se mantiene como respaldo.
8. **Escalabilidad de la compilación automática y la vista previa con documentos grandes** (tesis, libros, muchas imágenes o TikZ). Analizado el 2026-11-07. **Parcialmente resuelto**: disparador adaptativo y prevención de recarga implementados; el resto sigue pendiente (ver §4.5).

## 4.5 Escalabilidad con documentos grandes (análisis del 2026-11-07; implementación parcial)

> **Implementado (Sección 14):** el **disparador adaptativo** de la compilación (opción 1) y la **prevención de recarga visual** cuando el PDF no cambió (primer punto de la opción 7). El resto sigue siendo análisis. Antes de decidir lo demás, hay que **medir** con un documento de prueba representativo (ver al final).

### Implementado
- **Disparador adaptativo** (`lib/compilation.svelte.js` + `Editor.svelte`):
  - tras cada `compile:finished` del objetivo, `autoCompileDelayMs = max(3000, durationMs × 2)` (`adaptiveDelay`). El editor lo lee cada vez que programa una compilación automática;
  - si `durationMs > 10000` (`HEAVY_COMPILE_MS`), `autoCompilePaused = true`: no hay compilación automática y la barra de estado del editor muestra *"Auto-compilación pausada (Documento pesado). Usa ⌘S para compilar"* (Ctrl+S en Linux y Windows). Al pausarse se cancela el temporizador que ya estuviera programado;
  - es **temporal y por proyecto**: se reanuda en cuanto una compilación (por ejemplo, con ⌘S) termina en 10 s o menos, y se reinicia al cambiar el objetivo de compilación (`resetCompilation`). No se guarda en disco.
- **Sin recarga si el PDF no cambió:**
  - `App.publishPreview` corre en la goroutine del worker justo después de `CompileLaTeXContext`. Lee el `ModTime` del PDF con `os.Stat` y, si coinciden ruta, `ModTime` y tamaño con la última publicación (latexmk: *"All targets are up-to-date"*), **reutiliza `pdfVersion`**: no copia la instantánea y no cambia la URL;
  - `compile:finished` incluye `pdfModTime`. La UI solo actualiza `pdfVersion` si el `ModTime` cambió (segunda barrera), así que `Preview.svelte` no vuelve a cargar ni a dibujar el `<canvas>`.
  - *Nota:* `CompileLaTeXContext` mantiene su firma `(string, error)`; el `ModTime` se lee en el hook `OnFinished`, en la misma goroutine y sin que latexmk pueda estar escribiendo.

### Problema
La compilación automática (3 s sin escribir) funciona bien con documentos pequeños. Con documentos grandes (cientos de páginas, decenas de imágenes, figuras TikZ, bibliografía con biber) puede convertirse en un **cuello de botella** de CPU, disco y memoria, y también en la vista previa (decodificar y dibujar el PDF).

### Cómo se comporta hoy (verificado en el código)
| Pieza | Comportamiento actual | Por qué escala mal |
|-------|-----------------------|--------------------|
| Disparador (`Editor.svelte`) | ~~3 s fijos~~ → **adaptativo** desde la Sección 14: `max(3 s, 2 × duración)`, y pausa por encima de 10 s | Resuelto para el caso típico. Pendiente: un documento justo por debajo de 10 s sigue compilando en cada pausa de unos 20 s |
| Scheduler (`scheduler.go`) | Un solo worker; una petición nueva **no cancela** la compilación en curso, solo deja una pendiente (`pending = true`) | Encadena compilaciones casi sin pausa: la CPU queda ocupada de forma continua (ventiladores, batería) y el resultado ya está viejo cuando llega |
| latexmk (`orchestrator.go`) | Compilación completa con `-xelatex`; latexmk repite pasadas para referencias, índices y biber | xelatex es de un solo hilo y vuelve a procesar todas las imágenes y las figuras TikZ en cada pasada |
| Límite de tiempo | `compileTimeout = 60 s`, fijo | Un documento grande puede superarlo **siempre**: la compilación se corta y el usuario solo ve "superó el límite" |
| Prioridad del proceso | latexmk corre con prioridad normal | Compite con la UI (WebView, CodeMirror) por la CPU |
| Vista previa (`pdf.js`, `Preview.svelte`) | ~~`fetch` del PDF completo~~ → desde la Sección 13, **carga por rangos** de una instantánea | Ya no se copia el PDF entero en RAM (deuda #5 resuelta). Siguen pendientes: copiar la instantánea en disco en cada compilación (coste de E/S proporcional al tamaño) y decodificar las imágenes de la página visible en el hilo principal (`disableWorkerRendering: true`) |
| Recarga sin cambios | ~~se recargaba en cada éxito~~ → desde la Sección 14 se reutiliza `pdfVersion` si el `ModTime` no cambió | Resuelto: ni copia en disco ni recarga del canvas |

Ya existe un dato útil: `compile:finished` incluye `durationMs`, todavía sin usar.

### Opciones para más adelante (no decididas)
1. ~~**Disparador adaptativo.**~~ **Implementado** (umbral de 10 s, en memoria; ver "Implementado"). *Posible mejora:* recordar la pausa por proyecto en `.creative-ink-meta`.
2. **Cancelar compilaciones viejas.** Si llega una petición y la compilación en curso supera cierto tiempo, cancelarla y empezar de nuevo. *Riesgo:* un latexmk cortado a mitad puede dejar `.aux` inconsistentes; latexmk suele recuperarse en la pasada siguiente, pero hay que probarlo.
3. **Límite de tiempo configurable** por proyecto en lugar de 60 s fijos, o un límite más amplio cuando la compilación la pide el usuario (Cmd+S) frente a la automática.
4. **Modo borrador para la vista previa automática** (requiere decisión del usuario, porque cambia los flags de latexmk; ver la skill `latex-toolchain-orchestrator`):
   - `graphicx` con la opción `draft` en las compilaciones automáticas (marcos en lugar de imágenes), y compilación completa con Cmd+S. Ejemplo: `latexmk -usepretex='\PassOptionsToPackage{draft}{graphicx}'`.
   - Con archivo principal, `\includeonly{<capítulo activo>}` para compilar solo el capítulo que se edita.
   - Externalizar TikZ (`\usetikzlibrary{external}`) para que las figuras se compilen una vez y se reutilicen.
5. **Prioridad baja para latexmk:** `nice` en macOS/Linux y `BELOW_NORMAL_PRIORITY_CLASS` en Windows, para que la UI siga fluida mientras compila.
6. **Valores por defecto según el hardware:** usar el tier de `SystemHealthCheck` (`basic` / `advanced`) y `cpuCores` para elegir debounce y modo (borrador por defecto en equipos `basic`).
7. **Vista previa más barata:**
   - ~~No recargar si el PDF no cambió.~~ **Implementado** con `pdfModTime` y la reutilización de `pdfVersion`.
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
