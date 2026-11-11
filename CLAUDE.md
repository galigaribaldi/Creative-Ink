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
6. **Ramas (acordado el 2026-11-08), tres niveles:**
   - **`main`**: versión estable. Solo recibe `DEV` cuando una etapa está probada.
   - **`DEV`**: integración. Recibe las ramas de trabajo terminadas.
   - **`DEV-feat-<Tema>`**: ramas de trabajo, que **nacen siempre de `DEV`**. Ahí se escriben los cambios y se proponen los commits.
   - Integración siempre con *fast-forward* (`git merge --ff-only`), para que la historia siga lineal. Al integrar, la rama de trabajo se borra.
   - **Rama de trabajo actual: `DEV-feat-Rendimiento-latex`**: pruebas de rendimiento con documentos grandes (la tesis) antes de empezar otra cosa.
   - Antes de proponer commits, confirma la rama con `git branch --show-current`. Si es `main` o `DEV`, propón primero crear o cambiar a la rama `DEV-feat-…` correspondiente.

## Gotchas técnicos
- **Svelte 5 (runes):** en `<!-- svelte-ignore a, b -->` los códigos se separan con **coma**. Después de un espacio, el compilador trata el resto como comentario.
- **Svelte 5:** no llames `state` (ni `props`, `derived`, `effect`) a una prop o variable si usas la runa correspondiente: `$state` se interpretaría como un *store*.
- **pdfjs-dist 6.5, renderer worker:** se pinta en un Web Worker propio (`GlobalWorkerOptions.rendererSrc`), uno por documento, que termina `loadingTask.destroy()`. No vuelvas a poner `disableWorkerRendering: true`: el pintado volvería al hilo de CodeMirror.
- **Compilación automática apagada por defecto** (`compile.auto`, `internal/config`). Lee y escribe la configuración solo a través del servicio (`GetConfig`/`SetConfig`, `ci config`), nunca de `settings.json` o `.creative-ink-meta` a mano. Las peticiones del temporizador van con `RequestCompile('auto')`; las del usuario, con `'manual'`.
- **Confianza en el `.latexmkrc` (Sección 21):** nunca lances latexmk sin pasar por la confianza. El Scheduler y `ci` ya lo hacen: `compileOptionsFor(dir)` pone `-norc` con `compiler.WithCompileOptions`. Una carpeta `untrusted` jamás se compila automáticamente. Las decisiones solo cambian con `TrustProject`/`UntrustProject` o `ci trust`.
- **Comandos `ci`:** se declaran en `internal/ci/registry.go` y solo ahí. `ci help`, la guía (`ci guide`) y el Tab de la Terminal CI salen de ese registro: un comando fuera de él no existe para el usuario.
- **Procesos externos que se puedan cancelar** (latexmk, herramientas de `ci`): usa `system.KillTreeOnCancel(cmd)`; si no, cancelar deja huérfanos a xelatex/biber.
- **pdfjs-dist 6:** `PDFDocumentProxy.destroy()` **no existe**. Se libera con `doc.loadingTask.destroy()` (vía `releasePdf()` en `src/lib/pdf.js`). Usa siempre el `PDFWorker` compartido: sin él, cada documento crea un Web Worker nuevo.
- **fsnotify en macOS (kqueue):** abre un descriptor por carpeta **y por archivo** vigilado, y no es recursivo. Respeta `maxWatchedDirs` y los filtros de `isIgnoredEntry`; nunca vigiles carpetas de build ni `node_modules`.
- **pdfjs-dist 6 por rangos:** `disableAutoFetch: true` **exige** `disableStream: true`. Sirve siempre instantáneas inmutables (`?v=`), nunca el `<doc>.pdf` que latexmk reescribe. En Linux sin `-tags webkit2_41`, Wails no pasa cabeceras: no hay rangos (se descarga entero).
- **Terminal integrada (`RunTerminalCommand`):** es la **única** excepción a "nunca `sh -c`" y es ejecución arbitraria por diseño. En Windows la línea es `cmd.exe /d /s /c "<comando>"` (`core.cmdLine`): sin `/s`, `cmd` rompe los comandos que empiezan con comillas. La terminal nativa (`OpenSystemTerminal`, `ci terminal`) nunca recibe texto de la UI: solo la carpeta abierta, que elige Go. Pásale solo lo que el usuario escribió en la terminal; nunca construyas comandos a partir de documentos, logs o nombres de archivo, y nunca uses `{@html}` con contenido externo.
- **macOS necesita el menú nativo (`menu.go`):** sin el rol Edición, ⌘C/⌘X/⌘V no funcionan en WKWebView. Los atajos de la app viven en `App.svelte` (`runCommand`) y hacen `preventDefault`, así el menú no los repite. En Linux y Windows no se crea menú.
- **Archivos de texto:** Go solo abre UTF-8 sin bytes nulos y de hasta 8 MB (`FileManager.LoadText`). Nunca abras un archivo que el puente pueda alterar: el autoguardado lo reescribiría.
- **Editor dividido:** un mismo archivo nunca puede estar en los dos paneles (los autoguardados se pisarían). Usa siempre `openFile()` de `actions.js`; no asignes `workspace.currentFilePath` directamente desde otros módulos.
- **Convención de nombres:** todo lo que crea el editor usa `-` en lugar de espacios y sin acentos (`core.Slugify`). Nunca crees carpetas o archivos con espacios.
- **Dependencias del frontend:** `npm install` se ejecuta **siempre en `creative-ink/frontend/`**. En `creative-ink/` crea un `package.json` y un `node_modules` sueltos (ya pasó una vez).

## Estado actual (actualizar al cerrar cada sección)
Todo lo de abajo está **integrado en `main` y `DEV` (commit `32ad4e7`, 2026-11-08)**. El detalle de cada sección está en `docs/MAP.md` y en el historial de git (`git log --oneline`).

### Secciones integradas
| # | Sección | Dónde está documentada |
|---|---------|------------------------|
| 1 | Infraestructura y puente IPC (validada con `wails dev` el 2026-11-04) | MAP §2, §3 |
| 2 | Health check de LaTeX y perfil de hardware (validada el 2026-11-05) | MAP §4, §4.1 |
| 3 | Editor CodeMirror 6 con autoguardado (validada el 2026-11-05) | MAP §4.2 |
| 4 | Pipeline de compilación con latexmk | MAP §4.3 |
| 5 | Visor PDF.js en `<canvas>` | MAP §4.3 |
| 6 | Guardado y compilación desacoplados (Scheduler y eventos) | MAP §4.3 |
| 7 | Layout ActionBar \| Sidebar \| Workspace | MAP §1 |
| 8 | Carpetas, archivos y explorador | MAP §4.3.1 |
| 9 | Repositorio central multiplataforma y primer arranque | MAP §1 |
| 10 | Archivo principal (`.creative-ink-meta`) | MAP §4.3.2 |
| 11 | Explorador reactivo (fsnotify) | MAP §4.3.3 |
| 12 | Cmd/Ctrl+S, debounces y cierre seguro | MAP §4.2 |
| 13 | Visor PDF por rangos HTTP e instantáneas inmutables | MAP §4.3 |
| 14 | Disparador adaptativo y sin recarga si el PDF no cambió | MAP §4.5 |
| 15 | Terminal integrada | MAP §4.3.4 |
| 16 | SyncTeX bidireccional | MAP §4.3.5 |
| 17 | Terminal híbrida con comandos `ci` | MAP §4.3.6 |
| 18 | Estructura del repositorio (`.creative-ink/`, `tmp-Workspaces/`) y pantalla de inicio | MAP §1, §4.6 (paso 1) |
| 19 | Comandos básicos de editor, archivos de texto, ⌘N, menú de macOS y editor dividido | MAP §4.2 |
| 20 | Escalabilidad: configuración por capas (`compile.auto` apagado), PDF en un worker, detector de proyecto grande, cancelación acotada, terminal en disco | MAP §4.3, §4.3.6, §4.5, §4.6 (paso 2) |

### En curso: `DEV-feat-Rendimiento-latex`
Pruebas de rendimiento con la tesis (unas 200 páginas, imágenes pesadas, TikZ, bibliografía) antes de pasar a otra cosa. El plan de medición está en `docs/MAP.md` §4.5 ("Cómo medir antes de decidir").
- [ ] Programador: reiniciar `wails dev` y probar la Sección 20: abrir sin compilar (PDF `cached`), `ci config autocompile on|off`, ⌘S, `ci compile completo` con progreso y "Salida completa", el aviso "⚠ Proyecto grande" y escribir con la vista previa recargándose
- [ ] Medir: `durationMs` (primera e incrementales), CPU mientras se escribe, tiempo de carga y de dibujo del PDF, y memoria del WebView tras 20 compilaciones
- [ ] Ajustar con esos datos los umbrales del detector (`core/project_health.go`) y de la cancelación (`compiler/scheduler.go`)

### Pendientes (siguientes secciones; no empezar sin confirmación)
- **§4.6 paso 5 — Host de plugins** (JSON-RPC por stdio; el modelo de plugins sigue sin decidir).
- **Configuración:** capa del repositorio (`.creative-ink/config.json`) y claves nuevas: motor seleccionable (`pdflatex`/`xelatex`/`lualatex`, requisito del MVP), límite de tiempo configurable (hoy 60 s fijos), modo borrador para las compilaciones automáticas.
- **Decisión abierta:** ofrecer o no `--shell-escape` (externalización de TikZ).
- **Escalabilidad (§4.5):** prioridad baja para latexmk (`nice`), valores por defecto según el hardware, progreso de latexmk en la compilación automática.
- **UI:** cambiar el repositorio desde la UI (hoy hay que borrar `settings.json`); un selector de tema claro/oscuro.
- **Windows:** Job Objects para que la shell de la terminal mate a todos los nietos (latexmk y `ci` ya usan `taskkill /T`); compilar en Linux con `-tags webkit2_41` para tener rangos HTTP.
