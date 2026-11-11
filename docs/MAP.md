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
├── docs/WORKFLOW_BLUEPRINT.md    ← la metodología de trabajo (Tríada, contrato IPC, deuda técnica), generalizada para otros proyectos
└── creative-ink/                 ← app Wails (generada por `wails init`)
    ├── main.go                   ← arranque de Wails: ventana, assets, Bind(&App{})
    ├── app.go                    ← struct App: ÚNICA fachada IPC expuesta al frontend
    ├── window_theme.go           ← color nativo de la ventana = --surface-base del tema del sistema (sin destello al arrancar)
    ├── menu.go                   ← menú nativo (solo macOS): Archivo, Edición (portapapeles), Buscar, Ver, Ventana → evento menu:command
    ├── registry_app.go           ← ListTerminalCommands, guía en la vista previa (preview:show), CreateProjectFromTemplate
    ├── trust_app.go              ← confianza en el .latexmkrc: TrustProject/UntrustProject, -norc, evento trust:changed
    ├── config_app.go             ← GetConfig/SetConfig, cerrojo de compile.auto, vista previa del PDF existente (sin compilar)
    ├── health_app.go             ← detector de proyecto grande: análisis con debounce → evento project:health (solo avisa)
    ├── preview.go                ← middleware del AssetServer: sirve el PDF en /__preview/document.pdf
    ├── wails.json                ← configuración del proyecto Wails
    ├── go.mod
    ├── internal/                 ← lógica Go no importable desde fuera del módulo
    │   ├── ci/
    │   │   ├── registry.go       ← registro de comandos (CommandDef): Run, ci help, Info() para el autocompletado, RequiresTrust
    │   │   ├── guide.go          ← ci guide: guía .tex generada del registro y compilada → vista previa
    │   │   ├── status.go         ← ci status: ubicación, git, sincronización, objetivo, confianza y avisos
    │   │   └── ci.go             ← comandos internos `ci` de la terminal híbrida (traducción del Makefile, sin shell ni make)
    │   ├── config/
    │   │   └── config.go         ← servicio de configuración: registro de claves tipadas y capas (por defecto < usuario < proyecto)
    │   ├── compiler/
    │   │   ├── orchestrator.go   ← CompileLaTeXContext: latexmk -xelatex -synctex=1 con timeout/cancelación, PATH de TeX, errores del .log
    │   │   ├── synctex.go        ← SyncTeXForward/Backward: CLI synctex view/edit y parseo de su salida
    │   │   ├── manual.go         ← RunPasses: secuencias de herramientas (xelatex, bibtex…) con el candado de compilación
    │   │   └── scheduler.go      ← Scheduler: compilación en segundo plano, una a la vez, juntando ráfagas
    │   ├── core/                 ← dominio: proyectos, archivos, (futuro) plugins
    │   │   ├── file_manager.go   ← FileManager: guardado atómico (tmp + rename) y carga de documentos
    │   │   ├── workspace.go      ← ScanTree, ResolveInFolder, DefaultTeXFile, IsEditableFile
    │   │   ├── settings.go       ← <UserConfigDir>/creative-ink/settings.json: repositorio, autoCompile y recientes (UpdateSettings no pisa otros campos)
    │   │   ├── repository.go     ← estructura del repositorio, Slugify, CreateProject, CreateQuickDoc, StaleQuickDocs
    │   │   ├── hide_{windows,other}.go ← ocultar .creative-ink en Windows (atributo hidden)
    │   │   ├── templates.go      ← ListTemplates/CreateProjectFromTemplate: plantillas del repositorio y del usuario
    │   │   ├── trust.go          ← GetTrustStatus/TrustProject/UntrustProject: hash SHA-256 del .latexmkrc en settings.json
    │   │   ├── project_meta.go   ← <carpeta>/.creative-ink-meta: archivo principal del proyecto (ruta relativa)
    │   │   ├── watcher.go        ← FolderWatcher (fsnotify): cambios externos con debounce; TreeSignature
    │   │   ├── pdf_snapshots.go  ← PDFSnapshots: copias inmutables y versionadas del PDF para la vista previa
    │   │   ├── terminal*.go      ← ExecuteCommand(Context|Stream): sh -c / cmd /c, 30 s, sin stdin, grupo de procesos, salida limitada (+ copia al spool)
    │   │   ├── output_spool.go   ← OutputSpool: salida completa de cada comando de la terminal en disco (temporal, 8 ejecuciones), lectura por trozos UTF-8
    │   │   ├── project_health.go ← AnalyzeProject: demasiados archivos, archivos muy llenos, demasiadas imágenes
    │   │   ├── location.go       ← ClassifyLocation (repositorio / documento rápido / externa), FindGitRoot, SyncService, CheckProjectFolder (carpetas demasiado amplias)
    │   │   └── location_messages.go ← set de mensajes por defecto sobre la ubicación (externa, externa con git, sincronizada, copia de git)
    │   └── system/               ← entorno: health check de LaTeX y detección/instalación de motores
    │       ├── health.go         ← HealthStatus: herramientas de LaTeX, hardware y repositorio
    │       ├── platform.go       ← RepositoryOptions por SO, parser de XDG user-dirs; RepositoryName = My-Creative-Ink-Repository
    │       ├── appearance_<os>.go ← SystemPrefersDark: apariencia del sistema al arrancar (defaults, registro, gsettings)
    │       ├── proctree_{unix,windows}.go ← KillTreeOnCancel: cancelar mata el árbol entero (latexmk → xelatex/biber)
    │       ├── reveal.go         ← RevealInFileManager (open / xdg-open / explorer, sin shell)
│       ├── open_terminal*.go ← OpenTerminal: terminal nativa en una carpeta (Terminal, emulador del escritorio Linux, Windows Terminal / cmd), sin shell
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
        │   │   ├── FileTreeItem.svelte ← nodo recursivo del árbol; abre archivos de texto y se pueden arrastrar al editor (panel secundario)
        │   │   ├── Workspace.svelte   ← Editor | Preview (+ CompileLog bajo el editor); solo layout
        │   │   ├── Editor.svelte      ← CodeMirror 6 (tema de tokens, stex) + autoguardado con debounce
        │   │   ├── BottomPanel.svelte ← panel inferior con pestañas "Errores LaTeX" | "Terminal" (siempre montado)
        │   │   ├── CompileLog.svelte  ← contenido de la pestaña de errores de LaTeX
        │   │   ├── TrustBanner.svelte ← aviso "modo restringido" con un .latexmkrc sin confianza: Confiar y recompilar / Mantener restringido
        │   │   ├── LocationBanner.svelte ← franja de ubicación sobre la entrada de la Terminal CI y la Shell (ligera sin git, fuerte con git)
        │   │   ├── TerminalPanel.svelte ← terminal integrada: historial, entrada, ↑/↓, Ctrl+C
        │   │   ├── HealthPanel.svelte ← modal "Estado del sistema" (incluye "Sistema y repositorio")
        │   │   ├── RepositorySetup.svelte ← primer arranque: "Solo para mí" / "Para todos los usuarios"
        │   │   ├── StartScreen.svelte ← cada sesión: Nuevo proyecto, Documento rápido, proyectos, recientes y aviso de documentos rápidos antiguos
        │   │   └── Preview.svelte     ← PDF.js en <canvas>, doble buffer, conserva la página al recargar
        │   ├── lib/
        │   │   ├── ui.svelte.js          ← estado visual ($state): sidebarOpen, activeTool, healthOpen, splitEditor, focusedPane, startOpen. Nunca va a Go
        │   │   ├── compilation.svelte.js ← reflejo de los eventos compile:* (status, logs, pdfVersion, cached, disparador adaptativo)
        │   │   ├── config.svelte.js      ← reflejo del servicio de configuración (autoCompile) y del evento config:changed
        │   │   ├── workspace.svelte.js   ← reflejo de Go: carpeta, archivo de cada panel, último .tex, archivo principal, árbol, avisos de proyecto grande
        │   │   ├── actions.js            ← acciones globales y runCommand (atajos y menú nativo); registro de los dos paneles del editor
        │   │   ├── languages.js          ← resaltado por tipo de archivo (stex, BibTeX propio, lua, python, shell, yaml)
        │   │   ├── terminal.svelte.js    ← entradas y historial de la terminal (sobreviven al cambio de pestaña)
        │   │   ├── synctex.svelte.js     ← SyncTeX: highlight (Editor → PDF) y reveal (PDF → Editor)
        │   │   ├── theme.svelte.js       ← tema claro/oscuro: preferencia system|light|dark en localStorage → <html data-theme>
        │   │   ├── editor-theme.js       ← tema de CodeMirror hecho solo con tokens (EditorView.theme + HighlightStyle)
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
│   └── templates/<Nombre>/               ← plantillas de "Nuevo proyecto" compartidas con el repositorio
├── tmp-Workspaces/                       ← documentos rápidos de un solo archivo
│   ├── unnamed-tmp1/main.tex             ← uno por subcarpeta: no mezclan auxiliares
│   └── unnamed-tmp2/main.tex
└── Tesis-de-Maestria/                    ← un proyecto = una carpeta (nombre normalizado con "-")
    ├── .creative-ink-meta                ← archivo principal (main.tex)
    ├── main.tex · capitulos/ · figuras/ · referencias.bib
```
- **Convención de nombres:** todo lo que crea el editor usa `-` en lugar de espacios (`My-Creative-Ink-Repository`, `Tesis-de-Maestria`) y sin acentos en las carpetas (`core.Slugify`), para evitar problemas con herramientas que pasan por la shell (minted, externalización de TikZ) y con actualizaciones. El título del documento conserva el nombre original (escapado con `core.EscapeTeX`).
- **Migración:** si `settings.json` apunta a `My Creative Ink Repository`, al arrancar se renombra a `My-Creative-Ink-Repository` (y se actualizan `repositoryPath` y los recientes). Si ya existen las dos carpetas, no se mueve nada y la pantalla de inicio lo avisa.
- **Del usuario, en `<UserConfigDir>/creative-ink/`:** `templates/<Nombre>/` (sus plantillas de proyecto; ante un nombre repetido gana la del repositorio) y `guide/ci-guia.tex` → `.pdf` (la guía que genera `ci guide`).
- **Plantillas de proyecto (§4.6 paso 4):** cada subcarpeta es una plantilla y se copia entera al crear el proyecto (`core.CreateProjectFromTemplate`). No se copian enlaces simbólicos, ocultos (salvo `.creative-ink-meta` y `.latexmkrc`) ni auxiliares, y hay un límite de 2000 archivos y 200 MB. En los archivos de texto, `{{TITULO}}`/`{{TITLE}}` se sustituyen por el nombre del proyecto escapado para LaTeX. El archivo principal sale del `.creative-ink-meta` de la plantilla o, si no hay, de `main.tex` o el primer `.tex`. Un `.latexmkrc` de una plantilla propia queda **de confianza** (decisión 7).
- **Plugins por usuario:** en un repositorio personal, `<repo>/.creative-ink/plugins`; en uno compartido, `<UserConfigDir>/creative-ink/plugins` (nunca donde otras cuentas pueden escribir).
- **Documentos rápidos antiguos:** al empezar cada sesión, la pantalla de inicio lista los que llevan **más de 8 h** sin modificarse (`core.StaleQuickDocs`) y ofrece abrir `tmp-Workspaces` en el Finder. **El editor nunca los borra** (por ahora no hay opción automática).
- **En `settings.json` (por usuario, fuera del repositorio):** la ruta del repositorio, su ámbito, `autoCompile` y los **recientes** (`recentFolders`, 10 como máximo).

| SO | "Solo para mí" (`user`) | "Para todos los usuarios" (`shared`) | Cómo se calcula |
|----|-------------------------|--------------------------------------|-----------------|
| macOS | `~/Documents/My-Creative-Ink-Repository` | `/Users/Shared/My-Creative-Ink-Repository` | Rutas fijas del sistema |
| Linux | `<XDG_DOCUMENTS_DIR>/My-Creative-Ink-Repository` (p. ej. `~/Documentos/…`) | **No disponible** (no hay una carpeta compartida estándar sin `sudo`) | `$XDG_CONFIG_HOME/user-dirs.dirs`; si falta, `~/Documents` |
| Windows | `<FOLDERID_Documents>\My-Creative-Ink-Repository` (puede estar en OneDrive) | `<FOLDERID_PublicDocuments>\My-Creative-Ink-Repository` (normalmente `C:\Users\Public\Documents`) | Known Folders (`golang.org/x/sys/windows`), nunca `%USERPROFILE%` a mano |

- **Avisos del diagnóstico:** en macOS, si Documentos se sincroniza con iCloud; en Windows, si Documentos está en OneDrive (cada compilación subiría `.aux`/`.pdf`). En Linux, cuando la carpeta XDG no es `~/Documents`.
- **Por qué en el primer arranque y no en el instalador:** Wails v2 solo genera instalador en Windows (NSIS); en macOS es un `.app` y en Linux un binario. El primer arranque funciona igual en los tres sistemas.
- **La ruta la decide Go:** el frontend solo envía `"user"` o `"shared"` a `SetupRepository`; los ámbitos desconocidos o no disponibles se rechazan.
- **Creación:** `core.EnsureRepositoryLayout` con `MkdirAll(…, 0755)`: crea `.creative-ink/` y `tmp-Workspaces/`, y **ningún documento en la raíz** (es el lanzador). En modo compartido, la raíz y `tmp-Workspaces` pasan a `0777` + sticky (como `/Users/Shared`: todos pueden crear documentos y solo su dueño puede borrarlos); en Windows mandan las ACL de Documentos públicos. Los archivos nuevos se crean con `O_EXCL`: **nunca sobrescribe**.
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
| Binding | `GetWorkspace`        | UI → Go    | `() → main.WorkspaceState` = `{ location, needsSetup, needsStart, repositoryPath, canceled, folderPath, filePath, texFilePath, mainFilePath, autoCompile, tree: core.FileNode, truncated, trustStatus, trustFiles }`. **`location`** (Sección 23, §4.7) = `core.ProjectLocation` `{ kind: 'repository'\|'quickdoc'\|'external', folder, gitRoot, syncService, notices: { id, level: 'info'\|'warning', title, text, steps }[] }`: dónde vive la carpeta abierta, calculado por Go al cambiar de carpeta y **antes de abrirla**. **`trustStatus`** (`''`\|`trusted`\|`untrusted`\|`restricted`) y **`trustFiles`** describen el `.latexmkrc` de la carpeta donde corre latexmk (Sección 21). `filePath` es el documento activo (cualquier archivo de texto); **`texFilePath`** es el último `.tex` activo: el objetivo de compilación si no hay archivo principal (Sección 19). **`needsStart: true`** con el repositorio listo y sin proyecto abierto: la UI muestra la pantalla de inicio. `autoCompile` es el ajuste de `ci config autocompile` (por defecto `true`). `mainFilePath` es el archivo principal del proyecto (`""` si no hay, o si se borró). Carpeta abierta, documento activo y árbol. **`needsSetup: true`** en el primer arranque, sin crear nada en disco. Desde la Sección 18 **no abre la raíz del repositorio**: sin proyecto, `needsStart: true` | 9, 18 |
| Binding | `GetRepositoryOptions` | UI → Go   | `() → system.RepositoryOptions` = `{ os, osName, user, shared: RepositoryOption; warnings: string[]; currentScope, currentPath }`, con `RepositoryOption` = `{ scope, path, available, reason }`. Rutas propuestas para este SO | 9 |
| Binding | `SetupRepository`     | UI → Go    | `(scope: 'user'\|'shared') → main.WorkspaceState`. Crea el repositorio y su estructura (`.creative-ink/`, `tmp-Workspaces/`) en la ruta que calcula Go y guarda la elección. Devuelve `needsStart` (no abre ninguna carpeta). Rechaza ámbitos desconocidos o no disponibles | 9 |
| Binding | `GetStartScreen`      | UI → Go    | `() → main.StartScreen` = `{ repositoryPath, projects, quickDocs, staleQuickDocs: core.ProjectInfo[]; staleAfterHours; recent: main.RecentFolder[] (= { path, location, gitRoot }, sin las carpetas demasiado amplias, Sección 23); pluginsPath; warnings; templates: core.TemplateInfo[] }` (`TemplateInfo` = `{ name, path, source: 'repository'\|'user' }`), con `ProjectInfo` = `{ name, path, mainFile, modified }` | 18 |
| Binding | `CreateProject`       | UI → Go    | `(name) → main.WorkspaceState`. Crea `<repo>/<Slugify(name)>/` desde la plantilla con `main.tex` como principal, lo abre y lo compila. Error si ya existe o el nombre está reservado | 18 |
| Binding | `CreateQuickDocument` | UI → Go    | `() → main.WorkspaceState`. Crea `tmp-Workspaces/unnamed-tmp<N>/main.tex` (N siguiente libre), lo abre y lo compila | 18 |
| Binding | `OpenProject`         | UI → Go    | `(path) → main.WorkspaceState`. Solo carpetas **dentro del repositorio** (no la raíz, `.creative-ink` ni `tmp-Workspaces`) o **recientes** | 18 |
| Binding | `CreateProjectFromTemplate` | UI → Go | `(name, template) → main.WorkspaceState`. Como `CreateProject`, pero copia la plantilla `template` (su `path`; **solo** las que lista `GetStartScreen`, nunca una carpeta arbitraria). Confía en su `.latexmkrc` si lo trae | 22 |
| Binding | `ListTerminalCommands` | UI → Go   | `() → ci.CommandInfo[]` = `{ name, usage, description, requiresTrust, subcommands: { syntax, description, completion }[] }[]`: el **registro de comandos `ci`** sin manejadores. `completion` es la forma hasta su primer argumento (`compile solo `). La Terminal CI lo usa para Tab; `ci help` y `ci guide` salen del mismo registro | 22 |
| Binding | `OpenSystemTerminal`  | UI → Go    | `() → string`: abre la **terminal del sistema** en la carpeta abierta y devuelve su nombre (`Terminal`, `Konsole`, `Windows Terminal`…). La UI no envía nada: Go elige la carpeta y el emulador, y no ejecuta ningún comando. macOS: `open -a Terminal`; Linux: `$TERMINAL`, el emulador del escritorio (`XDG_CURRENT_DESKTOP`), `x-terminal-emulator` u otro conocido; Windows: `wt.exe -d .` o `cmd.exe` en una consola nueva. Error sin carpeta (`ErrNoFolder`) o sin emulador (`ErrNoTerminal`). Botón "Terminal del sistema" de la pestaña Shell y `ci terminal` | 23 |
| Binding | `RevealQuickDocuments`| UI → Go    | `() → void`: abre `tmp-Workspaces` en el gestor de archivos (`open` / `xdg-open` / `explorer`, sin shell) | 18 |
| Binding | `NewFileDialog`       | UI → Go    | `() → main.WorkspaceState`. **⌘N / Ctrl+N "Nuevo archivo…"**: diálogo nativo en la carpeta abierta; crea un archivo **vacío** de texto (`.tex` si no lleva extensión) con el nombre normalizado (`core.Slugify`, `Capítulo Dos` → `Capitulo-Dos.tex`) y lo hace documento activo, sin compilar. Debe quedar dentro de la carpeta abierta y **nunca reemplaza** un archivo (`core.ErrFileExists`). `canceled: true` si se cierra. Sin carpeta → `ErrNoFolder` (la UI crea entonces un documento rápido) | 19 |
| Binding | `LoadFile`            | UI → Go    | `(path) → main.OpenedFile`. Lee un archivo de texto de la carpeta para el **segundo panel** del editor; a diferencia de `OpenFile`, **no** cambia el documento activo ni compila | 19 |
| Binding | `RefreshWorkspace`    | UI → Go    | `() → main.WorkspaceState`: vuelve a escanear la carpeta abierta. La UI solo aplica el árbol y el archivo principal (`applyTree`), **nunca** `filePath` | 8, 11 |
| Binding | `OpenFolderDialog`    | UI → Go    | `() → main.WorkspaceState`. Diálogo nativo; `canceled: true` si se cierra. Abre la carpeta y elige `main.tex` o el primer `.tex`. **Rechaza (`core.ErrBroadFolder`) las carpetas que no son un proyecto** (raíz del repositorio, `.creative-ink`, `tmp-Workspaces`, la carpeta personal, Escritorio, Descargas, Documentos, la raíz de un disco) antes de abrir nada; `location` llega ya calculada | 8, 23 |
| Binding | `OpenFile`            | UI → Go    | `(path) → main.OpenedFile` = `{ path, content }`. Cualquier **archivo de texto** dentro de la carpeta abierta (`core.IsEditableFile`: `.tex`, `.bib`, `.sty`, `.cls`, `.txt`, `.md`…). Rechaza los de más de 8 MB y los binarios o no UTF-8 (`core.ErrNotUTF8`: guardarlos los corrompería). Lo hace documento activo y encola la compilación del **objetivo** (un `.bib` conserva el último `.tex`) | 8, 19 |
| Binding | `SaveContent`         | UI → Go    | `(path, content) → main.SaveResult` = `{ saved }`. Responde de inmediato y **no compila**: la UI pide `RequestCompile` tras 3 s sin escribir o con Cmd/Ctrl+S. `path` (absoluto, archivo de texto, dentro de la carpeta) viaja con cada guardado. Rechaza la promesa si la ruta no es válida o falla la escritura | 8, 12 |
| Binding | `SaveAsDialog`        | UI → Go    | `(content) → main.WorkspaceState`. Diálogo nativo; si falta la extensión, usa la del documento activo. Solo archivos de texto (`core.ErrNotEditable`). Guarda el **panel principal**. Si el destino está fuera de la carpeta, su directorio pasa a ser la carpeta abierta, **salvo que sea demasiado amplio** (`core.ErrBroadFolder`, comprobado **antes de escribir**). Lo hace documento activo y compila | 8, 23 |
| Binding | `SetMainFile`         | UI → Go    | `(path) → main.WorkspaceState`. `.tex` existente dentro de la carpeta, o `""` para quitarlo. Se guarda (ruta relativa) en `<carpeta>/.creative-ink-meta` y se recompila el nuevo objetivo | 10 |
| Binding | `RequestCompile`      | UI → Go    | `(trigger: 'auto'\|'manual') → boolean`: encola una compilación del **objetivo** (principal o último `.tex`) sin guardar. **`'auto'`** = temporizador de inactividad del editor: **Go la rechaza si `compile.auto` está apagado** (segundo cerrojo) **o si la carpeta tiene un `.latexmkrc` sin confianza** (Sección 21) y el Scheduler la puede **cancelar** si llega otra más nueva (§4.3). **`'manual'`** = ⌘S y los botones Guardar y Compilar: siempre se encola y nunca se cancela. `false` si se rechazó, el trigger es desconocido o la app se está cerrando | 7, 12, 20 |
| Binding | `GetConfig`           | UI → Go    | `() → config.Entry[]` = `{ id, type, default, scopes, description, value, scope }[]`: valor efectivo de cada clave y la capa de la que sale (`default` < `user` < `project`) | 20 |
| Binding | `SetConfig`           | UI → Go    | `(key, value, scope: 'user'\|'project'\|'') → config.Entry`. Valida clave (id o alias, p. ej. `autocompile`), ámbito y valor; `value: ""` quita el valor de esa capa. Guarda en `settings.json` (usuario) o en `.creative-ink-meta` (proyecto) y emite `config:changed`. Devuelve la entrada efectiva (puede venir de otra capa) | 20 |
| Binding | `ReadTerminalOutput`  | UI → Go    | `(runId, offset) → core.OutputChunk` = `{ runId, data, offset, next, size, done, dropped }`. Hasta 64 KB de la **salida completa** de un comando (en disco), cortados en un límite UTF-8. La UI solo lo llama con "Salida completa" abierta (cada 500 ms mientras el comando sigue). Error si la ejecución ya rotó (se guardan 8) | 20 |
| Binding | `GetProjectHealth`    | UI → Go    | `() → core.ProjectHealth` = `{ folder, warnings: { kind: 'files'\|'size'\|'images', message }[], files, texFiles, largestFile, largestBytes, largestLines, imageCalls, imageBytes, truncated }`: último análisis del detector de proyecto grande (solo informa) | 20 |
| Binding | `SyncTexForward`      | UI → Go    | `(line, file) → main.SyncTexForwardResult` = `{ found, page, x, y, width, height, reason }`. Dónde aparece la línea de `file` (`.tex` del proyecto) en el PDF del objetivo de compilación, en puntos PDF desde la esquina superior izquierda. Sin datos → `found: false` y `reason`, nunca error | 16 |
| Binding | `SyncTexBackward`     | UI → Go    | `(page, x, y) → main.SyncTexBackwardResult` = `{ found, file, line, reason }`. Qué `.tex` del proyecto y qué línea (desde 1) generaron ese punto. Los archivos fuera del proyecto (p. ej. `article.cls`) dan `found: false` | 16 |
| Binding | `RunTerminalCommand`  | UI → Go    | `(cmd) → main.TerminalResult` = `{ command, folder, output, exitCode, timedOut, canceled, durationMs, runId }`. Desde la Sección 20, la **salida completa** se escribe en disco mientras corre (`runId`, `ReadTerminalOutput`) y `output` es solo el resumen (shell: 256 KB como mucho; `ci`: una línea por pasada). **Terminal híbrida:** si la primera palabra es `ci`, la resuelve Go por dentro (§4.3.6, sin shell, sin carpeta obligatoria); si no, ejecuta `cmd` con `sh -c` (macOS/Linux) o `cmd /c` (Windows) en la carpeta abierta, sin stdin, con límite de 30 s y PATH ampliado con TeX y Homebrew. Un código distinto de 0, el timeout o la cancelación son resultados normales; rechaza si no hay carpeta, si el comando está vacío o si ya hay otro en curso. **Ejecución arbitraria por diseño:** solo con texto que escribió el usuario | 15 |
| Binding | `CancelTerminalCommand` | UI → Go  | `() → void`: detiene el comando en curso (mata su grupo de procesos) | 15 |
| Binding | `ConfirmClose`        | UI → Go    | `(saved: boolean) → void`: respuesta a `app:before-close` cuando la UI ya volcó CodeMirror a disco. Si `saved` es `false`, Go pregunta con un diálogo nativo antes de cerrar | 12 |
| Evento  | `compile:started`     | Go → UI    | `main.CompileStartedEvent` = `{ file }`: el **objetivo de compilación** (principal o activo). La UI ignora los eventos de otro objetivo | 8, 10 |
| Evento  | `compile:finished`    | Go → UI    | `main.CompileFinishedEvent` = `{ file, success, logs, durationMs, pdfVersion, pdfModTime, canceled, cached }`. **`canceled`**: una compilación automática obsoleta se canceló y la siguiente ya empezó (la UI sigue en "Compilando…"). **`cached`**: no se compiló nada; es el PDF que ya había en disco, publicado al abrir con `compile.auto` apagado (no cuenta para el disparador adaptativo). `pdfVersion` es la instantánea publicada (0 si falló la compilación o la copia); **si el PDF no cambió** (mismo `ModTime` y tamaño), Go **reutiliza la versión anterior** sin copiarla. `pdfModTime` es el `ModTime` del PDF en ms Unix. No se emite después del cierre | 8, 13, 14 |
| Evento  | `app:before-close`    | Go → UI    | Sin payload. Lo emite `OnBeforeClose` en el primer intento de cerrar la ventana; la UI hace `waitIdle()` y responde con `ConfirmClose`. Go cierra igual a los 2 s si no hay respuesta | 12 |
| Binding | `TrustProject`        | UI → Go    | `() → main.WorkspaceState`. Confía en el `.latexmkrc`/`latexmkrc` de la carpeta donde corre latexmk (la del objetivo de compilación): guarda su SHA-256 en `settings.json`, quita una restricción previa, emite `trust:changed` y **recompila** (manual) con su configuración. Lo usa el botón "Confiar y recompilar"; `ci trust add` hace lo mismo sin recompilar | 21 |
| Binding | `UntrustProject`      | UI → Go    | `() → main.WorkspaceState`. Restringe la carpeta: se olvida el hash y se compila **siempre con `-norc`**, sin volver a preguntar (`restricted`). Emite `trust:changed`. Botón "Mantener restringido" y `ci trust remove` | 21 |
| Evento  | `preview:show`        | Go → UI    | `main.PreviewShowEvent` = `{ file, title, pdfVersion, pdfModTime }`: muestra en la vista previa un PDF que **no** es el objetivo de compilación (la guía de `ci guide`) hasta la siguiente compilación. La UI ofrece "Volver al documento" y desactiva SyncTeX mientras tanto | 22 |
| Evento  | `trust:changed`       | Go → UI    | `main.TrustChangedEvent` = `core.TrustInfo` (`{ folder, status: 'trusted'\|'untrusted'\|'restricted', rcFiles, currentHash, trustedHash }`) + `{ project }`. Al decidir (banner o `ci trust`) o cuando una compilación ve que el `.latexmkrc` cambió (p. ej. tras un `git pull`). La UI lo aplica si `project` es la carpeta abierta | 21 |
| Evento  | `config:changed`      | Go → UI    | `main.ConfigChangedEvent` = `config.Entry` + `{ written, folder }`: una clave cambió (p. ej. `ci config autocompile on`). `value`/`scope` son los efectivos para `folder`; la UI los aplica si es la carpeta abierta | 20 |
| Evento  | `project:health`      | Go → UI    | `core.ProjectHealth` (ver `GetProjectHealth`). Se recalcula con un debounce de 2 s al abrir una carpeta, tras cambios externos y tras guardar; solo se emite si cambió. **Solo avisa:** la UI muestra "⚠ Proyecto grande" con un tooltip y no cambia nada más | 20 |
| Evento  | `terminal:started`    | Go → UI    | `main.TerminalStartedEvent` = `{ runId, command }`: empezó un comando y su salida completa se está guardando en disco | 20 |
| Evento  | `terminal:progress`   | Go → UI    | `main.TerminalProgressEvent` = `{ runId, pass, passes, tool, page, pages, elapsedMs, done, failed }`: progreso de `ci compile`/`ci clean` (pasada, herramienta, última página `[n]` que TeX escribió y total esperado de la compilación anterior), como mucho ~4 por segundo y en cada cambio de pasada | 20 |
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
- **CodeMirror 6** con un setup mínimo hecho a mano (sin `basicSetup`): números de línea, historial, `defaultKeymap`, `indentWithTab`, `bracketMatching`, resaltado según el archivo (`lib/languages.js`) y el tema de tokens de `lib/editor-theme.js` (claro y oscuro con las mismas variables CSS; ver la skill `ui-svelte-architect`, A2–A3).
- **Archivos de texto (Sección 19):** se abren y editan `.tex`, `.bib`, `.sty`, `.cls`, `.bst`, `.txt`, `.md`, `.csv`, `.json`, `.yaml`, `.lua`, `.py`, `.sh`, `Makefile`… (`core.IsEditableFile`); el resto sale atenuado en el explorador. Los auxiliares (`.aux`, `.log`…) siguen ocultos. Go rechaza archivos de más de 8 MB, binarios o **no UTF-8** (un `.bib` antiguo en Latin-1 se corrompería al autoguardar: hay que convertirlo antes). Resaltado: `stex` para la familia TeX, un modo **BibTeX** propio, y `lua`/`python`/`shell`/`yaml` de `legacy-modes`; el resto, texto plano.
  - **Objetivo de compilación:** el archivo principal (★) o, si no hay, el **último `.tex`** abierto en el panel principal (`App.texDoc` / `WorkspaceState.texFilePath`). Abrir `referencias.bib` no cambia la vista previa, y guardarlo recompila el documento. Un `.bib` nunca es principal ni usa SyncTeX.
- **Comandos de edición (todos de CodeMirror, `@codemirror/search` + `@codemirror/commands`):** deshacer/rehacer (⌘Z, ⇧⌘Z), **buscar y reemplazar** (⌘F; panel arriba, en español), siguiente/anterior (⌘G, ⇧⌘G), **ir a la línea** (⌥⌘G), siguiente coincidencia (⌘D), resaltar las demás apariciones de la selección, **varios cursores** (⌥-clic) y selección en columna (⇧⌥-arrastrar), comentar/descomentar con `%` (⌘/), mover y copiar líneas (⌥↑/↓, ⇧⌥↑/↓), borrar línea (⇧⌘K), sangrar (⌘] / ⌘[ y Tab). ⌘-clic sigue siendo SyncTeX en los `.tex`. En Linux y Windows, ⌘ = Ctrl y ⌥ = Alt.
- **Atajos de la app** (`App.svelte` → `runCommand` en `actions.js`): ⌘N nuevo archivo en el proyecto (sin proyecto, documento rápido), ⇧⌘N documento rápido, ⌘O abrir carpeta, ⌘S guardar y compilar, ⇧⌘S guardar como, ⌘B explorador, ⌘J / Ctrl+` terminal, ⌘\ dividir el editor.
- **Menú nativo (`menu.go`, solo macOS):** Archivo, Edición (rol nativo de Wails: deshacer, cortar, **copiar, pegar**, seleccionar todo), Buscar, Ver y Ventana. **Sin él, ⌘C/⌘X/⌘V no funcionan en WKWebView** (Wails no crea ningún menú si `options.App.Menu` es nil). Los clics emiten `menu:command` y la UI ejecuta el mismo `runCommand` que los atajos. Los aceleradores del menú no duplican los atajos: WKWebView entrega cada tecla primero a la página y solo pasa al menú las que nadie atendió (`preventDefault`). En Linux y Windows no hay menú (sería una barra dentro de la ventana) y los WebViews ya gestionan el portapapeles. Las etiquetas del menú Edición las pone Wails (no se pueden traducir).
- **Editor dividido (⌘\, botón en la ActionBar, menú Ver):** un segundo panel junto al principal para ver dos archivos lado a lado (por ejemplo, un capítulo y `referencias.bib`); el editor ocupa 2/3 del ancho y la vista previa 1/3. El panel principal es el documento activo de Go (`OpenFile`: decide qué se compila; Guardar como actúa sobre él); el segundo solo lee y guarda en su sitio (`LoadFile` + `SaveContent`). El explorador abre en el panel con el foco (marcado con una línea de acento). **Un mismo archivo nunca está en los dos paneles** (sus autoguardados se pisarían): elegirlo enfoca el panel que ya lo tiene. Guardar con ⌘S, cambiar de carpeta y cerrar la app vuelcan los dos paneles; cerrar la división guarda antes el segundo. **Arrastrar y soltar (2026-11-09):** un archivo editable arrastrado desde el explorador y soltado sobre el área del editor se abre en el **panel secundario** (activa la división si hacía falta; `openInSecondaryPane` en `actions.js`, que solo acepta archivos editables del árbol). Si ya está en un panel, se enfoca allí. Los manejadores van en fase de captura (`Workspace.svelte`) para que CodeMirror no inserte la ruta como texto; un tipo MIME propio (`application/x-creative-ink-path`) distingue estos arrastres de los de texto. **Divisor arrastrable (2026-11-09):** una barra de 4 px entre los dos paneles (`cursor: col-resize`) ajusta `ui.splitRatio` (20–80 %) al arrastrar. También responde a ←/→ con el foco y a doble clic, que centra la división. La proporción se recuerda entre sesiones en `localStorage`, porque es puramente visual.
- **Dos debounces** que cada tecla reinicia (Sección 12):
  - **1 s → guardar** (`SAVE_DELAY_MS`): el documento se convierte en texto **una sola vez** y se envía con `SaveContent`. Nunca hay más de un guardado en curso; los que llegan mientras tanto se juntan en uno con el contenido más reciente.
  - **Compilar tras la inactividad** (adaptativo, 3 s como mínimo, §4.5): **solo si `compile.auto` está activado** (por defecto no, Sección 20). `saveAndCompile()` espera a que el guardado llegue a disco y llama a `RequestCompile('auto')`. Mientras se escribe de corrido, latexmk no se ejecuta.
- **Cmd+S (macOS) / Ctrl+S (Linux y Windows) ya está soportado:** listener global en `App.svelte` con `preventDefault()`. Guarda al instante, sin esperar los debounces, y luego compila. También funciona dentro de CodeMirror (su `keydown` llega a `window`; se quitó el `Mod-s` del keymap del editor para que no haya dos manejadores). Los botones Guardar y Compilar hacen lo mismo. Si el guardado falla, no se compila.
- **Seguridad de datos:** si falla `OpenFile` (o `LoadFile` en el segundo panel), el editor no cambia de archivo y queda de solo lectura hasta abrir otro, así el autoguardado nunca sobrescribe un archivo con un documento vacío.
- **Cierre seguro (`OnBeforeClose`, Sección 12):** protocolo en dos fases sin bloquear (en Windows el hook corre en el hilo de la UI, y `runtime.Quit` vuelve a llamarlo en los tres sistemas):
  1. Primer cierre → `App.beforeClose` lo **previene**, emite `app:before-close` y arma un temporizador de 2 s (`closeFlushTimeout`).
  2. La UI hace `waitIdle()` (vuelca CodeMirror a disco) → `ConfirmClose(saved)` → `closeAllowed` → `runtime.Quit` → `beforeClose` ahora permite cerrar.
  3. Sin respuesta en 2 s, Go cierra igual. Con `saved = false`, un diálogo nativo pregunta "Cerrar sin guardar" / "Cancelar".
  - Un segundo clic en ✕ mientras se vuelca se ignora. `quit` y `confirmQuit` se inyectan para los tests.

## 4.3 Pipeline de compilación (Módulo 3)
- `internal/compiler.CompileLaTeXContext(ctx, workdir, filename)` ejecuta `latexmk -xelatex -interaction=nonstopmode -halt-on-error -synctex=1 <filename>` (sin shell). `-synctex=1` (desde la Sección 16) escribe `<doc>.synctex.gz` para SyncTeX. `CompileLaTeX` es el mismo sin cancelación externa. **Con un `.latexmkrc` sin confianza (Sección 21)** añade delante `-norc` y `-r <rc del usuario>` (`~/.latexmkrc`, `$XDG_CONFIG_HOME/latexmk/latexmkrc`): la opción llega en el `context` (`compiler.WithCompileOptions`), así que ni el Scheduler ni `CompileFunc` cambian de firma.
  - **Resolución de binarios:** `system.FindBinary` encuentra latexmk; su directorio se antepone al `PATH` del subproceso para que latexmk encuentre xelatex y biber aunque la app se abra desde el Finder.
  - **Seguridad:** el nombre de archivo no puede tener directorios ni empezar con `-`. Las ejecuciones van en serie (mutex) y tienen un límite de 60 s.
  - **Logs:** stdout y stderr van al mismo buffer. Si falla, los bloques `! ...` hasta `l.<n>` (más su línea de continuación) salen primero de `<doc>.log` y, si el `.log` no tiene errores, de stdout. Después va la cola de la salida (máximo 32 KB). Un bloque sin `l.<n>` se corta a las 6 líneas o en el siguiente `! `.
- **Flujo asíncrono:** autoguardado → `SaveContent` → responde `{saved}` (no compila). Compilar es otra petición: `RequestCompile('auto'|'manual')` → `Scheduler.Request(trigger)` → goroutine: `compile:started` → `CompileLaTeXContext` → `compile:finished` → `compilation.svelte.js` actualiza `pdfVersion` → `Preview` recarga el canvas.
  - **Scheduler:** un solo worker. Si llegan peticiones mientras compila, se juntan en **una** compilación más, que lee el archivo más reciente del disco: tres guardados rápidos producen como mucho dos compilaciones. Un `panic` se recupera y se informa como fallo. Al cerrar la app, `OnShutdown` → `Close(3s)` cancela latexmk y descarta lo pendiente.
  - **Cancelación acotada (Sección 20):** cada compilación tiene su propio contexto y su *trigger* (`auto`/`manual`). Una petición nueva **cancela la compilación automática en curso** si todavía es joven (menos de la mitad de lo que tardó la última, o 5 s antes de la primera) y la siguiente empieza en cuanto latexmk muere. Límites contra la inanición: **nunca** se cancela una manual (⌘S), y la que sigue a una cancelada **no se puede cancelar** (nunca dos veces seguidas). La cancelada emite `compile:finished` con `canceled: true` y no publica PDF. *Riesgo conocido:* un `.aux` a medias puede costar una pasada extra; latexmk suele recuperarse solo.
  - **Árbol de procesos:** latexmk y las herramientas de `ci` se lanzan con `system.KillTreeOnCancel`. En macOS/Linux van en su propio grupo de procesos (`Setpgid`) y cancelar hace `kill(-pid)`; en Windows, `taskkill /T /F` (y sin ventana de consola). Antes, cancelar mataba latexmk pero dejaba **xelatex/biber huérfanos** escribiendo el mismo `.aux`.
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
    - **Renderer worker (Sección 20):** `GlobalWorkerOptions.rendererSrc = pdf.renderer.min.mjs?url`. PDF.js 6.5 pinta la página en un `OffscreenCanvas` dentro de su propio Web Worker y el hilo principal solo copia el `ImageBitmap` terminado (`drawImage`), así que las imágenes pesadas y las figuras TikZ **ya no bloquean CodeMirror**. Es **un worker por documento** y lo termina `loadingTask.destroy()` (`releasePdf`). El doble buffer de `Preview.svelte` no cambia. Si falta `OffscreenCanvas` 2D (WebKitGTK antiguo), PDF.js vuelve solo al hilo principal, sin errores.
    - **Propiedad:** solo existe un documento vivo (`pdfDoc`). Uno nuevo se promueve (y se libera el anterior) o se libera en el `finally`, sin excepción.
    - Antes de liberar un documento, se cancela su render (`renderTask.cancel()`), como exige PDF.js.
  - **Retención de página:** `currentPageNumber` (por defecto 1) se conserva entre compilaciones y se ajusta a `numPages` si el PDF nuevo tiene menos páginas.
  - **Navegación (2026-11-09):** el encabezado tiene "‹ Página X de Y ›". Los botones cambian `currentPageNumber` y vuelven a dibujar con PDF.js; si hay una recarga en curso, solo mueven la página y es la recarga la que la dibuja.
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
  - Límite de **30 s**; un timeout o una cancelación matan el **grupo de procesos** entero (`Setpgid` + `kill(-pid)`), no solo el `sh`. En Windows, `cmd.exe` se lanza sin ventana (`CREATE_NO_WINDOW`) y con la línea de órdenes sin reescapar (`CmdLine`): **`cmd.exe /d /s /c "<comando>"`** (2026-11-10). Con `/s`, `cmd` solo quita el par de comillas exterior, así que un comando que empieza con comillas y tiene más de dos (`"C:\Program Files\…\git.exe" log "--format=%h %s"`) llega intacto; `/d` ignora los AutoRun del registro. *Límites conocidos en Windows:* los nietos pueden sobrevivir a la cancelación (haría falta un Job Object), y la salida de `cmd` llega en la página de códigos de la consola, no en UTF-8 (acentos sin verificar).
  - **Intérprete:** `/bin/sh` (en Debian/Ubuntu es `dash`) y `cmd.exe`, no la shell del usuario: no hay alias ni PATH de `.zshrc`/`.bashrc`, ni PowerShell.
  - **No interactivo:** sin stdin (EOF inmediato), `TERM=dumb`, `NO_COLOR`, `PAGER`/`GIT_PAGER=cat`, `GIT_TERMINAL_PROMPT=0` y `GIT_EDITOR=true` (un `git commit` sin `-m` se aborta en lugar de quedarse esperando). `sudo` y los editores no funcionan.
  - **PATH:** `system.ToolPATH()` añade al final las rutas de TeX y Homebrew que faltan (una app abierta desde el Finder no las tiene), así `tlmgr` y `git` de Homebrew se encuentran.
  - Salida combinada (stdout+stderr) limitada a **256 KB**, con un aviso de truncado.
- **Fachada:** `RunTerminalCommand` ejecuta en la carpeta abierta, **un comando a la vez** (`termMu`), y devuelve `TerminalResult`. `CancelTerminalCommand` lo detiene; `OnShutdown` también. El binding puede tardar hasta 30 s: no bloquea otros bindings (Wails atiende cada llamada en su propia goroutine), y la UI muestra "Ejecutando…".
- **Frontend:** `BottomPanel.svelte` bajo el editor, con pestañas **"Errores LaTeX" | "Terminal CI" | "Shell"**, siempre montado. Son **dos contextos separados**, cada uno con su panel, su historial (↑/↓) y sus ejecuciones; Go sigue ejecutando un solo comando a la vez:
  - **Terminal CI:** prompt `CI ❯` en el color de acento. Se escribe **sin `ci`**: `compile completo` se envía como `ci compile completo` (`toCiCommand`; si el usuario escribe `ci`, no se duplica). La sugerencia en gris sale de `CI_COMMANDS` y se acepta con Tab o →. Funciona sin carpeta abierta.
  - **Shell:** prompt `$` neutro tras la carpeta. La línea se envía **tal cual** a `RunTerminalCommand` y necesita una carpeta. Un error de compilación lo abre en su pestaña solo si estaba cerrado (si no, un punto rojo en la pestaña), y lo cierra al compilar bien si lo había abierto él. `TerminalPanel.svelte`: Enter ejecuta, ↑/↓ recorre el historial, Ctrl+C o "Detener" cancela, `clear`/`cls` limpia la vista. Atajos: **Ctrl+`** o **⌘J / Ctrl+J**, y botón en la ActionBar.
- **Seguridad:** `RunTerminalCommand` es **ejecución arbitraria de comandos por diseño**. Si alguna vez se ejecutara JavaScript ajeno en el WebView (por ejemplo, por una vulnerabilidad de PDF.js con un PDF malicioso), podría usarla. Reglas: la UI **solo** le pasa lo que el usuario escribió en la terminal, **nunca** texto derivado de documentos, logs o nombres de archivo; mantener `pdfjs-dist` actualizado; y no renderizar HTML que no sea nuestro (`{@html}` está prohibido con contenido externo).
- **Salida completa en disco (Sección 20):** cada comando se guarda mientras corre (`core.OutputSpool`, `ExecuteCommandStream`) y "Salida completa" la lee con `ReadTerminalOutput` (ver §4.3.6).
- **Terminal del sistema (2026-11-10):** para lo que esta terminal no puede hacer (`sudo`, comandos interactivos, la shell del usuario con sus alias), el botón **"Terminal del sistema"** de la pestaña Shell y `ci terminal` abren la terminal nativa en la carpeta del proyecto (`OpenSystemTerminal` → `system.OpenTerminal`). Go elige la carpeta y el emulador; la UI no envía ningún texto, así que no amplía la superficie de `RunTerminalCommand`. En Linux, el emulador se elige con `$TERMINAL` → el del escritorio → `x-terminal-emulator` → el primero conocido instalado, y se separa de la sesión de la app (`Setsid`). En Windows se pasa `-d .` a `wt.exe` con la carpeta como directorio de trabajo, porque `wt` trata `;` como separador de comandos.
- **Pendiente:** PTY real y `xterm.js` (requiere instalar dependencias), Job Objects en Windows para la shell, decodificar la página de códigos de `cmd.exe` y elegir la terminal externa preferida (iTerm, WezTerm…) con una clave `terminal.external`.

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
| `ci config [list]` | — | Cada clave con su valor efectivo y de qué capa sale (por defecto, usuario, proyecto) |
| `ci config autocompile [on\|off] [--project]` | — | Ver o cambiar `compile.auto` (**apagada por defecto**). Con `--project` solo para el proyecto abierto (gana sobre el valor del usuario) |
| `ci config get\|set\|unset <clave> [valor] [--project]` | — | Forma general; `unset` quita el valor de esa capa. Avisa si el proyecto oculta tu valor de usuario |
| `ci compile rapido` | `make rapido` | `xelatex -interaction=nonstopmode -synctex=1 <doc>.tex` (una pasada) |
| `ci compile borrador` | `make borrador` | Dos pasadas de `xelatex … "\def\borrador{}\input{<doc>}"` (la primera puede fallar). Requiere que el preámbulo use `\borrador` |
| `ci compile solo <capítulo>` | `make solo CAP=…` | Dos pasadas de `xelatex -jobname=<doc>_<carpeta> "\def\SoloCapitulo{<capítulo>}\input{<doc>}"` → `<doc>_<carpeta>.pdf` |
| `ci compile completo` | `make pdf` | XeLaTeX → BibTeX → XeLaTeX → XeLaTeX (las XeLaTeX intermedias pueden fallar, como `\|\| true`) |
| `ci guide` | — | Genera `<UserConfigDir>/creative-ink/guide/ci-guia.tex` **recorriendo el registro** (cada comando, su uso, su descripción y sus formas), lo compila con el orquestador y lo muestra en la vista previa (`preview:show`). Solo usa el núcleo de LaTeX (funciona con BasicTeX) |
| `ci status` | — | Carpeta, ubicación (repositorio, documento rápido o ⚠ fuera), git, sincronización, objetivo de compilación, confianza, `compile.auto` y los avisos de ubicación (los mismos mensajes que la franja de la terminal) |
| `ci terminal` | — | Abre la terminal del sistema en la carpeta del proyecto (`OpenSystemTerminal`) |
| `ci clean tikz` | `make tikz-clean` | Borra `*.pdf`, `*.md5`, `*.log` y `*.dpth` de `Figures/TiKz_Libraries/externalized/` |
| `ci clean all` | `make limpiar-todo` | `latexmk -C <doc>.tex` (con `-norc` si el `.latexmkrc` no es de confianza) y los restos (`.brf`, `.run.xml`, `.idx`, `.ilg`, `.ind`, `.blg`, `.bbl`, `.xdv`). Borra también el PDF |

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
- **Progreso y salida completa (Sección 20, opciones B y C):** la salida de cada pasada **no se guarda en memoria**. Va en streaming a un archivo en disco (`core.OutputSpool`, carpeta temporal privada, 8 ejecuciones, 64 MB como mucho por ejecución), y Go solo conserva una cola de 256 KB para el resumen de errores. Del mismo flujo se extrae el progreso, que llega como `terminal:progress` (pasada, herramienta, página `[n]` y total esperado leído del `.log` anterior: "Pasada 2/4 · xelatex · página 143 de 212 · 34 s", con una barra). El botón **"Salida completa"** de cada comando lee del disco con `ReadTerminalOutput` (trozos de 64 KB; cada 500 ms mientras sigue corriendo; en la vista, los últimos 256 KB). Los comandos de la shell también se guardan en disco (`ExecuteCommandStream`).

**Registro de comandos (Sección 22, §4.6 paso 4):** `internal/ci/registry.go` declara cada comando (`CommandDef`: `Name`, `Usage`, `Description`, `Subcommands`, `RequiresTrust`, `Run`). `ci.Run` despacha por el registro y rechaza un comando `RequiresTrust` si la carpeta no es de confianza (`ErrNeedsTrust`). De él salen `ci help`, la guía de `ci guide` y `ListTerminalCommands`. En la Terminal CI, Tab completa: con una coincidencia la escribe (y se ve en gris antes); con varias, completa el prefijo común y abre un menú que Tab recorre.

**Seguridad: confianza en el `.latexmkrc` (Sección 21, §4.6 paso 3).** latexmk lee el `.latexmkrc`/`latexmkrc` de la carpeta en la que corre, y ese archivo es **código Perl**. Una carpeta con uno que el usuario no ha aceptado (`untrusted`) se compila siempre con `latexmk -norc` (también `ci clean all`) y **nunca automáticamente**. La UI muestra un banner con "Confiar y recompilar" / "Mantener restringido".

| Comando | Qué hace |
|---------|----------|
| `ci trust [status]` | Carpeta, estado (de confianza, sin confianza o restringida), archivos rc y el hash **actual** frente al **confiado** (avisa si el archivo cambió desde que confiaste) |
| `ci trust add` | Confía en el contenido actual: guarda su SHA-256 y quita una restricción. Si el archivo cambia, vuelve a estar sin confianza |
| `ci trust remove` | Restringe la carpeta: siempre `-norc` y sin volver a preguntar |

`ci compile …` ejecuta xelatex y bibtex directamente, así que nunca lee el `.latexmkrc`.

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
- [x] **Paso 2 — Servicio de configuración por capas (Sección 20):** `internal/config` (claves tipadas) con capas por defecto < usuario (`settings.json`, `config`) < proyecto (`.creative-ink-meta`, `config`); `GetConfig`/`SetConfig`, `config:changed`, `ci config list|get|set|unset [--project]`. Primera clave: `compile.auto`, **apagada por defecto** y respetada por la UI (no programa el temporizador) y por Go (`RequestCompile('auto')` se rechaza). El `autoCompile` antiguo de `settings.json` se lee como valor del usuario y desaparece al escribir. *Pendiente:* la capa del repositorio (`.creative-ink/config.json`) y más claves (motor, límites de tiempo).
- [x] **Paso 3 — Confianza (Sección 21):**
  - `core.GetTrustStatus`/`TrustProject`/`UntrustProject`, con `TrustedProjects` (ruta real → SHA-256 de los rc) y `RestrictedProjects` en `settings.json`;
  - tres estados: `trusted` (sin rc, o el hash coincide), `untrusted` (rc nuevo o cambiado) y `restricted` (el usuario dijo que no);
  - `latexmk -norc` mientras no sea `trusted`; `compile.auto` rechazado si `untrusted`;
  - `ci trust status|add|remove`, banner en la UI y `WorkspaceState.trustStatus/trustFiles`.
  - *Pendiente:* elegir la política en la instalación (hoy, por defecto, se pregunta por cada carpeta con rc), y que los proyectos creados por el editor nazcan confiados (hoy no traen `.latexmkrc`, así que ya lo son).
- [x] **Paso 4 — Registro de comandos `ci` (Sección 22):** `internal/ci/registry.go` y `ci.Info()`/`ListTerminalCommands`; `ci help` generado; `ci guide` (guía `.tex` del registro, compilada y mostrada con `preview:show`, en `<UserConfigDir>/creative-ink/guide/`); autocompletado con Tab y menú en la Terminal CI; plantillas de proyecto del repositorio y del usuario (`CreateProjectFromTemplate`).
- [ ] **Paso 5 — Host de plugins** (JSON-RPC por stdio, manifiesto con contribuciones y capacidades).

### Diseño pendiente (pasos 3 a 5)
El análisis original del 2026-11-08 quedó resuelto en los pasos 1 y 2: `compile.auto` por proyecto con respaldo del usuario, configuración del proyecto en `.creative-ink-meta` y orden 1 → 2 → 3 → 4. Queda como referencia para lo que falta:
- **Confianza (paso 3):** almacén del usuario con la ruta real de la carpeta y el **hash del `.latexmkrc`** (si cambia, por ejemplo tras un `git pull`, se vuelve a preguntar). Una carpeta sin confianza y con `latexmkrc`/`.latexmkrc` se compila en **modo restringido** (`latexmk -norc`, con los rc del sistema y del usuario explícitos con `-r`), sin compilación automática y sin `shell-escape`. Una franja discreta en la UI ofrece "Confiar en esta carpeta"; en la terminal, `ci trust` / `ci trust status` / `ci untrust`. Sin `.latexmkrc` no se pregunta nada.
- **Registro de comandos `ci` (paso 4):** hecho en la Sección 22. Los plugins añadirán sus comandos al mismo registro.
- **Host de plugins (paso 5; el modelo sigue sin decidir):** recomendación, procesos externos con JSON-RPC por stdin/stdout, al estilo LSP. Un manifiesto declara contribuciones (comandos `ci <plugin> …`, claves `plugin.<id>.*`) y capacidades (leer o escribir en el proyecto, herramientas, red). `ci check` y `ci formatear` (los `tools/*.py` de la tesis) serían el primer plugin.
- **Claves de configuración previstas:** `compile.engine` (`xelatex`\|`pdflatex`\|`lualatex`), `compile.timeoutMs`, `compile.shellEscape` (requerirá confianza).

## 4.7 Proyectos fuera del repositorio: ubicación, git, importar y guardar en otra ruta (Sección 23, 2026-11-10)

### Reglas acordadas con el usuario (2026-11-10)
- **Antes de abrir** una carpeta que no es del repositorio (⌘O, un reciente, Guardar como en otra ruta), Go revisa si está **bajo git** (`.git` en ella o en una carpeta padre; nunca cuentan la carpeta personal ni la raíz del disco, para que un repositorio de *dotfiles* en `~` no marque todo).
  - **Sin git → aviso ligero** (`info`): no afecta a la integridad; **sugiere** copiarlo al repositorio, solo como sugerencia.
  - **Con git → aviso fuerte** (`warning`, color de aviso): **no se recomienda copiar** (la copia y el original divergirían) y explica cómo guardar en git (`git status` → `git add -A` → `git commit -m` → `git push`).
  - **Si aun así se copia** un proyecto con git (fase 2), la copia lleva un **archivo de comentario** (`COPIA-DE-GIT.md`, `core.GitCopyNoteFile`) y la terminal muestra el aviso `git-copy`: se trabaja con una copia y hay que guardar en git para llevar los cambios al original.
  - Los textos son un **set de mensajes por defecto** en Go (`core/location_messages.go`: `external`, `external-git`, `synced`, `git-copy`); la UI solo los muestra.
- **Fuera del repositorio todo funciona igual**: compilación, Terminal CI, Shell, SyncTeX y confianza.
- **Carpetas sincronizadas** (iCloud Drive, Dropbox, OneDrive, Google Drive, `~/Library/CloudStorage/…`): **solo se avisa** (`synced`), nunca se bloquea.
- **Importar:** se ofrecen **las dos formas**, "Copiar a Creative-ink" y "Abrir en su ubicación". `.git` **sí se copia**. Antes de copiar **siempre** se informa del tamaño (sean KB o GB); por encima de **2 GB**, un aviso grande de que puede tardar.
- **Aviso en la terminal: las dos cosas**, una franja fija sobre la entrada y una línea en el historial.
- **`.latexmkrc` importado:** se copia, pero **sin confianza** (banner "Confiar y recompilar").
- **Guardar como fuera de la carpeta, con varios archivos:** **las dos cosas**, preguntar en el momento ("Solo este archivo" / "El proyecto entero") y un comando aparte "Guardar proyecto como…".

### Fase 1 — Ubicación y avisos (hecha, rama `DEV-feat-Ubicacion-proyectos`)
- `core.ClassifyLocation(repo, carpeta)` → `ProjectLocation` (`repository`, `quickdoc` o `external`, más `gitRoot`, `syncService` y `notices`). `App.setWorkspace` la calcula **una vez por cambio de carpeta** y la guarda con el resto del estado (`a.location`); viaja en `WorkspaceState.location`.
- **Sincronización** (`core.SyncService`, solo mira rutas): `Mobile Documents` (iCloud), `Library/CloudStorage/<Servicio>-…`, el "Escritorio y Documentos" de iCloud y nombres de carpeta conocidos en cualquier SO (`Dropbox`, `OneDrive`, `OneDrive - Empresa`, `Google Drive`…).
- **Carpetas demasiado amplias** (`core.CheckProjectFolder` → `ErrBroadFolder`): raíz del repositorio, `.creative-ink`, `tmp-Workspaces`, la carpeta personal, Escritorio/Descargas/Documentos (nombres en inglés y en español), la raíz de un disco y `/Volumes/<disco>`, `/mnt/<disco>`, `/media/<usuario>/<disco>`. Se rechazan en ⌘O, en `OpenProject` (un reciente antiguo) y en Guardar como **antes de escribir**; la pantalla de inicio deja de listarlas en recientes.
- **Terminal:** `LocationBanner.svelte` sobre la entrada de las dos pestañas (ligera: superficie de cromo y texto apagado; fuerte: borde y fondo `--warning-soft`), plegada a una línea por aviso con "Detalles" (pasos de git) y "×" (oculta hasta abrir otra carpeta). Al abrir una carpeta con avisos, `noteLocation()` deja una línea en el historial de cada contexto. `ci status` resume la ubicación y repite los avisos.
- **Pantalla de inicio:** los recientes externos llevan la etiqueta "fuera del repositorio" (ligera) o "git · fuera del repositorio" (fuerte).

### Fase 2 — Importar proyecto (pendiente, rama `DEV-feat-Importar-proyecto`)
- `InspectImport()`: Go abre el diálogo y **se queda la ruta** (la UI recibe un `importId`). Análisis en segundo plano: nombre propuesto, archivos, **tamaño total** y espacio libre, candidatos a archivo principal, referencias que salen de la carpeta (`../`, rutas absolutas), `.latexmkrc`, `.git` (tamaño), enlaces simbólicos y textos no UTF-8.
- Resumen en la UI con el tamaño **siempre** visible y el aviso grande por encima de 2 GB; con git, el aviso fuerte y "Abrir en su ubicación" como opción destacada.
- `ImportProject(importId, opciones)`: asíncrono (`import:progress`, `import:finished`, cancelable), copia a `<repo>/.creative-ink/importing/<id>/` y `os.Rename` al final; el original nunca se toca. `core.CopyTree` común con las plantillas, **sin sustituir marcadores** en la importación. Copia `.git`, `.latexmkrc` (sin confianza), `.gitignore` y el PDF del principal; no copia auxiliares ni `.synctex.gz`. Con git, escribe `COPIA-DE-GIT.md` y la terminal muestra `git-copy` (se guardará el origen en `.creative-ink-meta`).

### Fase 3 — Guardar en otra ruta (pendiente, rama `DEV-feat-Guardar-proyecto`)
- Guardar como fuera de la carpeta en un proyecto de varios archivos: pregunta "Solo este archivo" / "El proyecto entero". Comando "Guardar proyecto como…" (copia la carpeta con `CopyTree`) y "Mover al repositorio" para un proyecto externo abierto (= importar la carpeta actual).

## 4.4 Deuda técnica conocida (análisis del 2026-11-05)
1. ~~**Guardar y compilar están acoplados en `SaveContent`.**~~ **Resuelto el 2026-11-06:** `SaveContent` responde de inmediato y la compilación va por el `Scheduler` con eventos (ver §4.3).
2. ~~**Se compila con cada pausa de 1 s.**~~ **Resuelto el 2026-11-07:** guardar (1 s) y compilar (3 s) tienen debounces separados; `SaveContent` ya no compila. **Cmd+S / Ctrl+S ya está soportado** en la UI: guarda al instante y compila (ver §4.2).
3. ~~**No se guarda al cerrar la ventana.**~~ **Resuelto el 2026-11-07:** `OnBeforeClose` con protocolo en dos fases (`app:before-close` → `waitIdle()` → `ConfirmClose`), un límite de 2 s y un diálogo nativo si el guardado falló (ver §4.2).
4. ~~**La carpeta *inicial* depende del directorio de trabajo** (`./workspace`).~~ **Resuelto el 2026-11-07:** la app abre un repositorio central en `~/Documents/My Creative Ink Repository`, una ruta **absoluta** construida con `os.UserHomeDir()` y `filepath.Join` (válida en macOS y Linux). Se crea con `MkdirAll(0755)` si no existe y lleva un `main.tex` inicial si no hay ningún `.tex`, sin sobrescribir nunca. Funciona igual desde `wails dev` y desde el Finder. **Ampliado el mismo día:** la ruta depende del SO y de la elección del primer arranque ("Solo para mí" o "Para todos los usuarios"); en Linux se respeta XDG (`~/Documentos`) y en Windows, Known Folders (ver §1). **Desde la Sección 18** se llama `My-Creative-Ink-Repository`, la raíz es un lanzador (sin `main.tex`) y los recientes se recuerdan en `settings.json`. *Pendiente menor:* cambiar el repositorio desde la UI (hoy hay que borrar `settings.json`).
5. ~~**El PDF se carga entero en memoria (`fetch` → `data`).**~~ **Resuelto el 2026-11-07:** PDF.js carga por **rangos HTTP** (`getDocument({ url, rangeChunkSize, disableAutoFetch, disableStream: true })`) desde instantáneas inmutables (`?v=<pdfVersion>`) que sirve `preview.go` con `206 Partial Content`. *Pendiente:* medirlo con un PDF grande en la app (ver §4.5) y, en Linux, compilar con `-tags webkit2_41` para tener rangos.
6. ~~**Proyectos de varios archivos:** se compila el `.tex` **activo**.~~ **Resuelto el 2026-11-07:** cada carpeta puede tener un **archivo principal** (`SetMainFile`, guardado en `.creative-ink-meta`). Se compila y se muestra ese archivo mientras se edita cualquier capítulo (ver §4.3.2). Texto original: Si editas un capítulo que se incluye con `\input`, se compila el capítulo y no el documento principal. Falta definir un "archivo principal" por carpeta.
7. ~~**El árbol no se actualiza solo** (no hay `fsnotify`).~~ **Resuelto el 2026-11-07:** `core.FolderWatcher` (fsnotify) avisa con `workspace:changed`, con debounce y solo si el árbol visible cambió, y la UI refresca el explorador sola (ver §4.3.3). El botón Actualizar se mantiene como respaldo.
8. **Escalabilidad de la compilación automática y la vista previa con documentos grandes** (tesis, libros, muchas imágenes o TikZ). Analizado el 2026-11-07. **En gran parte resuelto:** disparador adaptativo y sin recarga (Sección 14); `compile.auto` apagado por defecto, PDF dibujado en un worker, cancelación acotada, terminal en disco con progreso y detector de proyecto grande (Sección 20). Lo pendiente y las mediciones, en §4.5 (rama `DEV-feat-Rendimiento-latex`).

## 4.5 Escalabilidad con documentos grandes (análisis del 2026-11-07; Secciones 14 y 20)

> **Implementado:** disparador adaptativo y sin recarga si el PDF no cambió (Sección 14); las cuatro decisiones del 2026-11-08 y el detector de proyecto grande (Sección 20). **Siguiente paso: medir** con la tesis en la rama `DEV-feat-Rendimiento-latex` (ver "Cómo medir" al final) antes de decidir lo que queda.

### Implementado
- **Sección 20 (decisiones del 2026-11-08 sobre los 4 cuellos de botella):**
  1. **`autoCompile` conectado (opción C):** servicio de configuración por capas, **apagado por defecto** y cambiable desde la terminal (`ci config autocompile on [--project]`). Doble cerrojo: la UI no programa el temporizador y Go rechaza `RequestCompile('auto')`. Con la compilación automática apagada, abrir un proyecto **no ejecuta latexmk**: Go publica el PDF que ya hay en disco (`cached`).
  2. **Pintado del PDF fuera del hilo principal (opción A):** renderer worker de PDF.js 6.5 (ver §4.3). **Barrera extra: detector de proyecto grande** (`core.AnalyzeProject`), con tres esquemas: **demasiados archivos** (más de 1500 archivos visibles o 100 `.tex`), **archivos muy llenos** (un `.tex`/`.bib`/`.sty`/`.cls` de más de 300 KB o 4000 líneas) y **demasiadas imágenes** (más de 60 `\includegraphics`/`\includepdf`/`\includesvg` fuera de comentarios; el tooltip suma además los MB de imágenes de la carpeta). Corre en segundo plano (debounce de 2 s; como mucho 20 000 entradas y 3000 lecturas) y **solo avisa**: "⚠ Proyecto grande" en la barra de estado del editor con un tooltip. No cambia la compilación ni la vista previa.
  3. **Cancelación acotada del Scheduler (opción A)** y árbol de procesos (ver §4.3).
  4. **Terminal: progreso estructurado (B) y salida en disco bajo demanda (C)** (ver §4.3.6).
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
| Scheduler (`scheduler.go`) | ~~no cancelaba~~ → desde la Sección 20, **cancelación acotada** de las automáticas jóvenes y árbol de procesos con `KillTreeOnCancel`; `compile.auto` apagado por defecto | Resuelto en lo principal. Pendiente: medir cuántas cancelaciones hay al escribir con pausas y si algún `.aux` queda inconsistente |
| latexmk (`orchestrator.go`) | Compilación completa con `-xelatex`; latexmk repite pasadas para referencias, índices y biber | xelatex es de un solo hilo y vuelve a procesar todas las imágenes y las figuras TikZ en cada pasada |
| Límite de tiempo | `compileTimeout = 60 s`, fijo (las de `ci` tienen 10 min) | Un documento grande puede superarlo **siempre** en la compilación con ⌘S: se corta y el usuario solo ve "superó el límite". Alternativa hoy: `ci compile completo` |
| Prioridad del proceso | latexmk corre con prioridad normal | Compite con la UI (WebView, CodeMirror) por la CPU |
| Vista previa (`pdf.js`, `Preview.svelte`) | ~~`fetch` del PDF completo~~ → desde la Sección 13, **carga por rangos** de una instantánea | Ya no se copia el PDF entero en RAM (deuda #5) y, desde la Sección 20, se dibuja en el renderer worker (fuera del hilo de CodeMirror). Sigue pendiente: copiar la instantánea en disco en cada compilación (coste de E/S proporcional al tamaño) |
| Recarga sin cambios | ~~se recargaba en cada éxito~~ → desde la Sección 14 se reutiliza `pdfVersion` si el `ModTime` no cambió | Resuelto: ni copia en disco ni recarga del canvas |

`compile:finished` incluye `durationMs`, que ya usa el disparador adaptativo y que es el primer dato que hay que medir.

### Opciones para más adelante (no decididas)
1. ~~**Disparador adaptativo.**~~ **Implementado** (umbral de 10 s, en memoria; ver "Implementado"). *Posible mejora:* recordar la pausa por proyecto en `.creative-ink-meta`.
2. ~~**Cancelar compilaciones viejas.**~~ **Implementado** en la Sección 20 (cancelación acotada, §4.3). Falta comprobar con la tesis que latexmk se recupera bien de un `.aux` a medias.
3. **Límite de tiempo configurable** por proyecto en lugar de 60 s fijos, o un límite más amplio cuando la compilación la pide el usuario (Cmd+S) frente a la automática.
4. **Modo borrador para la vista previa automática** (requiere decisión del usuario, porque cambia los flags de latexmk; ver la skill `latex-toolchain-orchestrator`):
   - `graphicx` con la opción `draft` en las compilaciones automáticas (marcos en lugar de imágenes), y compilación completa con Cmd+S. Ejemplo: `latexmk -usepretex='\PassOptionsToPackage{draft}{graphicx}'`.
   - Con archivo principal, `\includeonly{<capítulo activo>}` para compilar solo el capítulo que se edita.
   - Externalizar TikZ (`\usetikzlibrary{external}`) para que las figuras se compilen una vez y se reutilicen.
5. **Prioridad baja para latexmk:** `nice` en macOS/Linux y `BELOW_NORMAL_PRIORITY_CLASS` en Windows, para que la UI siga fluida mientras compila.
6. **Valores por defecto según el hardware:** usar el tier de `SystemHealthCheck` (`basic` / `advanced`) y `cpuCores` para elegir debounce y modo (borrador por defecto en equipos `basic`).
7. **Vista previa más barata:**
   - ~~No recargar si el PDF no cambió.~~ **Implementado** con `pdfModTime` y la reutilización de `pdfVersion`.
   - ~~Cargar por rangos~~ **Implementado** en la Sección 13.
   - ~~Dibujar en el worker~~ **Implementado** en la Sección 20 (renderer worker).
8. **Visibilidad:** ~~progreso como eventos~~ **implementado para `ci compile`** en la Sección 20 (`terminal:progress`). Falta para la compilación automática (latexmk).

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
