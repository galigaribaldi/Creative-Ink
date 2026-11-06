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
- Decisiones **pendientes** (no las asumas, espera instrucciones): componente del editor, visor de PDF, modelo de plugins, migración a TypeScript, licencia y patrón final de IPC.

## Reglas de trabajo (OBLIGATORIAS)
Las hace cumplir `.claude/settings.json`. Aun así, respétalas aunque un permiso falte:
1. **Git:** NO ejecutar `git add`, `git rm`, `git commit`, `git push` ni nada que publique ramas o reescriba historia. Solo se permiten `git checkout`/`switch`/`stash`/`branch`, y siempre **con autorización previa**. Lectura (`status`, `diff`, `log`) libre.
2. **Instalación, instanciación y arranque:** NO ejecutar `wails init/dev/build`, `npm install/run`, `go get/install/mod tidy`, `brew`, `tlmgr`, `sudo`, ni compiladores LaTeX. **El programador ejecuta esos comandos.** Claude escribe el código y le dice exactamente qué comando correr.
3. Sí se permite verificar sin efectos externos: `go build`, `go vet`, `gofmt`, `wails doctor`.
4. Idioma de comunicación: **español**. Código e identificadores: inglés.
5. **Mensajes de commit:** NO agregar la línea `Co-Authored-By` ni ninguna otra atribución a Claude en los mensajes de commit que se propongan. Formato: prefijo convencional (`feat`, `fix`, `build`, `chore`…) y mensaje en español.

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
- [ ] **Siguiente sección** — pendiente de instrucciones
