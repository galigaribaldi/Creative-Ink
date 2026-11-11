# Workflow Blueprint: desarrollo de software con un asistente de IA

> Manual de ingeniería para equipos que construyen software junto a un asistente de IA. Recoge el método con el
> que se construyó Creative-ink (Go + Wails v2 + Svelte 5), generalizado para replicarlo en otros proyectos.
> Los ejemplos vienen de este repositorio; las reglas sirven para cualquier pila con una frontera clara entre
> interfaz y núcleo.

---

## 0. La idea en una página

Un asistente de IA rinde como un ingeniero senior solo si, en cada sesión, tiene **el mismo contexto que tendría
un miembro veterano del equipo**: las reglas, la arquitectura, el estado y los detalles finos de cada
tecnología. Ese contexto no puede vivir en la memoria de la conversación, porque se pierde y se resume. Tiene que
vivir **en el repositorio**, versionado junto al código.

El método se sostiene en cinco piezas:

1. **La Tríada de contexto:** `CLAUDE.md` (reglas), `docs/MAP.md` (arquitectura y estado) y `.claude/skills/`
   (conocimiento especializado). Se cargan o se consultan en cada sesión.
2. **Un contrato vivo en la frontera:** una tabla con cada binding, evento y ruta entre la interfaz y el núcleo.
3. **Un núcleo que decide y una interfaz que refleja:** cero lógica de negocio en el frontend.
4. **La deuda técnica como un artefacto:** cada cuello de botella se nombra, se analiza con opciones medibles, se
   decide, se implementa por fases y se tacha con fecha.
5. **Una verificación que no depende de la confianza:** comandos objetivos (formato, análisis estático, tests con
   detector de carreras, verificación por sistema operativo, chequeo de la UI) antes de dar algo por terminado.

Y una regla de gobierno que lo envuelve todo: **la IA escribe y verifica; el humano decide, instala, ejecuta la
app y publica.**

---

## 1. El Sistema de la Tríada

Cada archivo responde a una pregunta distinta. Mezclarlas es la forma más rápida de que la documentación se
pudra.

| Pieza | Pregunta que responde | Cuándo se lee | Cuándo cambia | Tamaño deseable |
|-------|-----------------------|---------------|---------------|-----------------|
| `CLAUDE.md` | ¿Qué está permitido, qué es obligatorio y dónde estamos? | **Siempre**: se carga en cada sesión | Al cambiar una regla, al descubrir un *gotcha*, al cerrar una sección | Corto (~100 líneas) |
| `docs/MAP.md` | ¿Cómo está construido, qué cruza la frontera y por qué se decidió así? | Al planificar o tocar una zona | Con **cada** cambio de estructura, de contrato o de decisión | Lo que haga falta, por secciones |
| `.claude/skills/<dominio>/` | ¿Cómo se hace bien *esto* concreto, y cómo compruebo que está bien? | Al trabajar en ese dominio | Al aprender algo verificable del dominio | Enfocado: un dominio por skill |

### 1.1 `CLAUDE.md`: la constitución
Contiene solo lo que tiene que estar en la cabeza en **todo** momento:
- **Stack y prohibiciones** (p. ej. "Prohibido: Electron").
- **Visión del producto** en viñetas: las decisiones tomadas y las **pendientes**, marcadas como "no las asumas,
  espera instrucciones".
- **Reglas de trabajo obligatorias**, numeradas: qué comandos puede ejecutar la IA, idioma, formato de commits,
  modelo de ramas.
- **Gotchas técnicos:** cada uno, una trampa real que ya costó tiempo. Ejemplos de este proyecto:
  - `pdfjs-dist 6` no tiene `PDFDocumentProxy.destroy()`;
  - en macOS, sin menú nativo, ⌘C/⌘V no funcionan en el WebView;
  - `npm install` fuera de `frontend/` crea un `package.json` suelto.
- **Estado actual compacto:** una tabla de secciones integradas que apunta a MAP.md, más lo que está en curso y la
  lista de pendientes. El detalle va en MAP.md y en el historial de git, no aquí.

> **Regla:** si algo de `CLAUDE.md` puede vivir en MAP.md o en una skill sin riesgo, muévelo. Cada línea de
> `CLAUDE.md` se paga en todas las sesiones.

**Las reglas se hacen cumplir, no solo se escriben.** `.claude/settings.json` traduce las reglas a permisos:
- `allow` para lo que no tiene efectos externos: `git status/diff/log`, `go vet`, `go build`, `gofmt`;
- `ask` para lo que necesita autorización previa: `git switch`, `git branch`, `git stash`;
- `deny` para lo que es del humano: `git add/commit/push`, `wails dev/build`, `npm install`, `go get`, `go mod tidy`,
  `sudo` y los compiladores de LaTeX.

`CLAUDE.md` explica el porqué; `settings.json` lo garantiza aunque alguien lo olvide.

### 1.2 `docs/MAP.md`: el plano y la bitácora técnica
Es la **fuente única de verdad de la arquitectura**. Secciones que demostraron su valor:

1. **Mapa de directorios** con una línea por archivo ("← qué hace"). Se actualiza en el mismo cambio que crea o
   borra el archivo.
2. **Reglas de capas** con un diagrama (UI → fachada → núcleo → sistema) y sus prohibiciones (p. ej.
   "`internal/*` no importa Wails").
3. **Contrato IPC** (ver §3).
4. **Una sección por subsistema** (editor, pipeline de compilación, explorador, terminal, SyncTeX,
   configuración…): qué hace, por qué así, límites conocidos y qué queda *pendiente*.
5. **Diseños acordados**, con las **decisiones del usuario** numeradas y fechadas, y un **plan por pasos** con casillas.
6. **Deuda técnica** numerada y **análisis de escalabilidad** (ver §5).
7. **Comandos que ejecuta el humano**, para que la IA nunca tenga que adivinarlos.

> **Regla:** MAP.md describe el código **que existe**. Cuando algo cambia, el párrafo viejo se reescribe o se
> tacha con la fecha de resolución (`~~…~~ **Resuelto el 2026-11-07:** …`), nunca se deja contradiciendo al código.

### 1.3 Skills: el conocimiento del especialista
Una skill es una carpeta con un `SKILL.md`, opcionalmente con scripts. La IA la activa por su descripción cuando
la tarea entra en ese dominio. En este proyecto hay tres:

| Skill | Dominio | Qué aporta |
|-------|---------|------------|
| `ui-svelte-architect` | Interfaz Svelte 5 | La **biblia de diseño**: tokens, temas claro y oscuro, reglas estrictas del color de acento, tipografía, iconos, movimiento, un checklist de revisión y un registro de la deuda visual. Más la arquitectura de componentes y estado, y el script `check-svelte.mjs` |
| `go-wails-ipc-bridge` | Backend y fachada IPC | Una sola fachada, ejecución que no bloquea, resultados frente a errores, carreras de datos, eventos `dominio:acción` |
| `latex-toolchain-orchestrator` | Procesos externos (latexmk) | Resolución de binarios sin el PATH de la shell, ejecución sin shell, timeouts, matar el árbol de procesos, extraer errores del `.log` |

Anatomía de una buena skill:
- **Frontmatter con una descripción "cuándo usarla"** precisa: es lo que decide si se activa.
- **"Lee primero"**, que remite a `CLAUDE.md` y MAP.md, y una declaración de **precedencia**: si discrepan, ganan
  ellos.
- **Contexto del proyecto "verifícalo antes de fiarte"**: rutas y módulos reales, que pueden cambiar.
- **Principios** (pocos y accionables) y **pasos** para la tarea típica.
- **Gotchas verificados en el código fuente**, no recordados de memoria.
- **Una forma objetiva de comprobar el trabajo**: un script de solo lectura que no instala nada. En este proyecto,
  `check-svelte.mjs` compila cada componente con el propio compilador del proyecto.

### 1.4 Cómo se mantiene sincronizada la Tríada
- **Precedencia:** `CLAUDE.md` > `MAP.md` > skill. Una skill nunca contradice el contrato.
- **Cada cambio de código arrastra su documentación en el mismo commit:** un binding nuevo actualiza la tabla IPC;
  un archivo nuevo, el mapa de directorios; una trampa descubierta, los *gotchas*.
- **Barridos de obsolescencia periódicos.** Busca en los tres archivos referencias a funciones, flags o archivos que
  ya no existen. En este proyecto aparecieron, entre otros:
  - `PrepareRepository` y `pdfTimestamp`, que ya no existían;
  - un "solo se abren `.tex`" de cuando ya se abría cualquier archivo de texto;
  - un "pendiente: streaming" que ya estaba hecho.

  Cada resto obsoleto es una instrucción falsa para la próxima sesión.

**Anti-patrones:**
- duplicar el mismo detalle en `CLAUDE.md` y en MAP.md;
- skills genéricas ("buenas prácticas de Go") sin nada propio del proyecto;
- un estado de secciones con veinte "falta validar" que nadie cierra: compáctalo en una tabla en cuanto se integra.

---

## 2. El ciclo de trabajo por sección

El trabajo avanza en **secciones numeradas** (aquí, de la 1 a la 22). Cada una sigue el mismo ciclo:

```
 Petición ─► (Análisis sin código, si se pide) ─► Decisiones del humano
     ─► Implementación ─► Verificación objetiva ─► Documentación (Tríada)
     ─► Commits propuestos ─► Validación humana en la app ─► Integración
```

1. **Petición con instrucciones estrictas.** El humano fija el alcance y lo que *no* se toca ("No toques
   `Editor.svelte`").
2. **Análisis primero cuando hay incertidumbre.** Para problemas abiertos, la IA entrega opciones evaluadas y una
   recomendación, sin escribir código (ver §5).
3. **Decisiones explícitas.** Las ambigüedades que cambian el diseño se preguntan, con opciones concretas y una
   recomendada. Las que tienen una respuesta convencional se deciden y se anuncian.
4. **Implementación** siguiendo las skills del dominio.
5. **Verificación** con comandos (§6). Si algo falla, se corrige antes de informar. Si algo no se pudo verificar
   (p. ej. la app real), se dice explícitamente.
6. **Documentación:** MAP.md (contrato, mapa, sección), `CLAUDE.md` (estado, *gotchas*) y la skill si se aprendió
   algo del dominio.
7. **Commits propuestos** en el formato acordado: prefijo convencional, mensaje en español y sin atribución a la IA.
   **Cada commit debe compilar por sí solo.** Si un cambio de firma cruza paquetes, va en el mismo commit.
8. **Validación humana** en la app real: lo que la IA no puede ejecutar (aquí, `wails dev` y los compiladores).
9. **Integración** según el modelo de ramas.

### 2.1 Reparto de responsabilidades
| La IA hace | El humano hace |
|------------|----------------|
| Escribe código, tests y documentación | Decide alcance, prioridades y disyuntivas de diseño |
| Ejecuta verificaciones sin efectos externos (formato, análisis estático, tests, build) | Instala dependencias (`npm install`, `go get`, `go mod tidy`) |
| Propone commits y comandos exactos | Ejecuta la app, hace commits, integra y publica |
| Lee el código fuente de las dependencias para verificar sus APIs | Ejecuta herramientas con efectos externos (compiladores, gestores de paquetes) |

> **Por qué funciona:** el humano conserva el control de todo lo irreversible o lo que sale del repositorio, y la
> IA nunca queda bloqueada, porque siempre entrega "el comando exacto que tienes que ejecutar".

### 2.2 Modelo de ramas
- `main`: estable. Solo recibe `DEV` cuando una etapa está probada.
- `DEV`: integración.
- `DEV-feat-<Tema>`: ramas de trabajo que **nacen de `DEV`**.
- **Integración solo con `git merge --ff-only`**: la historia queda lineal y nada se reescribe. Antes de abrir una
  rama nueva se integra la anterior, para que dos ramas hermanas nunca diverjan.
- La IA comprueba la rama (`git branch --show-current`) antes de proponer commits y, si está en `main` o `DEV`,
  propone primero cambiar.

---

## 3. El Contrato IPC: una tabla viva en la frontera

En una app de escritorio con interfaz web y núcleo nativo, **la frontera IPC es la superficie de ataque, la
fuente de los bugs de integración y el lugar donde dos lenguajes se ponen de acuerdo.** Por eso se documenta en una
única tabla, y se actualiza en el mismo cambio que la modifica.

### 3.1 Formato
| Tipo | Nombre | Dirección | Firma / payload | Sección |
|------|--------|-----------|-----------------|---------|
| Binding | `RequestCompile` | UI → Go | `(trigger: 'auto'\|'manual') → boolean`. `'auto'` se rechaza si `compile.auto` está apagado o la carpeta no es de confianza; `'manual'` siempre se encola y nunca se cancela | 7, 12, 20, 21 |
| Evento | `compile:finished` | Go → UI | `{ file, success, logs, durationMs, pdfVersion, pdfModTime, canceled, cached }` | 8, 13, 14 |
| HTTP | `/__preview/document.pdf?v=<n>` | WebView → Go | Instantánea inmutable; admite `Range` → `206` | 4, 13 |

Cada fila responde a cinco preguntas: qué es, quién inicia, qué viaja exactamente (con los nombres JSON reales),
qué semántica tiene (casos límite, valores especiales como `0` o `""`) y en qué sección se introdujo o cambió.

### 3.2 Por qué es vital
1. **Es la fuente de verdad para los dos lados.** La IA que escribe el Svelte y la que escribe el Go (a menudo la
   misma, en sesiones distintas) leen la misma fila. Los nombres camelCase de los tags JSON se escriben aquí antes de
   usarse.
2. **Hace visibles los cambios de contrato.** Pasar de `RequestCompile()` a `RequestCompile(trigger)` es una
   decisión que se anota, se comunica y se aplica a la vez en los dos lados. Sin tabla, solo se descubre al
   fallar en ejecución.
3. **Es el inventario de la superficie de seguridad.** Cada binding que recibe una ruta dice cómo se valida
   ("solo `.tex` dentro de la carpeta abierta", "solo plantillas listadas"). El único binding que ejecuta comandos
   arbitrarios está señalado como tal.
4. **Documenta el modelo asíncrono.** Qué responde de inmediato (`SaveContent → { saved }`) y qué llega después
   por eventos (`compile:started`/`compile:finished`). Así nadie diseña un binding que bloquee la UI 60 segundos.
5. **Distingue resultados de errores.** Un fallo de compilación es un *resultado* (`success: false`); un archivo
   que no se pudo escribir es un *error* (la promesa se rechaza). La tabla lo dice en cada fila.

### 3.3 Reglas del contrato
- **Una sola fachada** (`App`). Solo sus métodos exportados son bindings. Los auxiliares no se exportan.
- **Eventos con el nombre `dominio:acción`** (`trust:changed`, `project:health`) y payloads como structs con tags JSON.
- **Rutas absolutas validadas en el núcleo** contra una raíz permitida, siguiendo los symlinks.
- **El código generado** (`wailsjs/`) no se edita a mano: se regenera, y se hace commit de él después.
- **Los tipos de metadatos para la UI van sin funciones:** `ci.CommandInfo` frente a `ci.CommandDef`, para no
  depender de cómo serializa el generador.
- **Cada fila nueva se añade en el mismo cambio** que el binding o el evento.

---

## 4. Cero lógica de negocio en el frontend

> **La interfaz no decide; refleja.** El núcleo (Go) es el dueño del disco, de los procesos, de la configuración, de
> la seguridad y de las reglas del dominio. El frontend muestra el estado que el núcleo le dicta y le pide acciones.

### 4.1 Qué vive en cada lado
| Vive en el núcleo (Go) | Vive en la interfaz (Svelte) |
|------------------------|------------------------------|
| Archivos, rutas y su validación | Qué cajón está abierto, qué pestaña se ve |
| Qué se compila (objetivo, archivo principal) | La página del PDF que se está viendo |
| Configuración por capas (`compile.auto`) y su persistencia | Tema claro/oscuro y proporción del editor dividido (en `localStorage`) |
| Confianza en el `.latexmkrc` y el flag `-norc` | El banner que pregunta |
| El registro de comandos `ci` | El autocompletado que se alimenta de él |
| Detección de "proyecto grande" | El icono ⚠ y su tooltip |

### 4.2 Patrones concretos
- **Módulos espejo.** `workspace.svelte.js`, `compilation.svelte.js` y `config.svelte.js` solo guardan "el último
  estado que dijo Go" y se actualizan con `applyWorkspace()` o con eventos. No calculan reglas.
- **Estado visual, nunca en el núcleo.** Lo puramente visual no cruza el puente (ni barra lateral, ni ítem en hover,
  ni página actual). Si se persiste, va en `localStorage`.
- **Doble cerrojo en las reglas críticas.** Cuando la UI *ayuda* aplicando una regla (no programar la compilación
  automática si está apagada), el núcleo **también** la aplica (`RequestCompile('auto')` se rechaza). Un frontend
  con fallos, un estado desactualizado o un plugin futuro no pueden saltársela.
- **Validación siempre en el núcleo.** La UI puede filtrar por comodidad (solo aceptar archivos del árbol al
  soltar), pero la validación que importa es la de Go (`ResolveInFolder`, `LoadText`, la lista de plantillas
  permitidas).
- **La seguridad nunca depende del frontend.** El único binding de ejecución arbitraria (la terminal) solo recibe
  lo que el usuario escribió. La UI no construye comandos con datos de documentos, logs o nombres de archivo, y
  nunca renderiza HTML ajeno.
- **El núcleo se prueba sin interfaz.** Todo lo que toca el sistema se **inyecta**: el emisor de eventos, los
  diálogos, la función de compilación, el observador del sistema de archivos. Así los tests de la fachada corren
  sin el runtime de la UI y verifican la secuencia exacta de eventos.

### 4.3 Señales de alarma
- Un `if` en un componente que reproduce una regla del dominio sin que el núcleo la aplique.
- Un componente que lee o escribe archivos de configuración del núcleo.
- Un estado "fuente de verdad" en un store de la UI que el núcleo desconoce.
- Una cadena de comando construida en el frontend a partir de algo que no tecleó el usuario.

---

## 5. Deuda técnica y cuellos de botella como artefactos

La deuda no se "recuerda": se **registra**, se **analiza**, se **decide** y se **cierra** por escrito.

### 5.1 Registrar
MAP.md mantiene una lista **numerada** de deuda técnica. Cada entrada tiene:
- **Título del síntoma** ("Guardar y compilar están acoplados en `SaveContent`").
- **Por qué importa** (impacto en el usuario, en la CPU, en la memoria o en la seguridad).
- **Estado:** pendiente, parcialmente resuelto (con el detalle) o resuelto, tachado y con fecha:
  `1. ~~**Guardar y compilar están acoplados.**~~ **Resuelto el 2026-11-06:** …`

La entrada tachada **no se borra**. Es la memoria de por qué el código es como es.

### 5.2 Analizar antes de implementar
Para los problemas grandes (en este proyecto, la escalabilidad con tesis de más de 200 páginas), primero se pide
un **análisis sin código**:
1. **Diagnóstico verificado en el código**, no supuesto. Hay que corregir el enunciado si el código dice otra cosa;
   aquí, por ejemplo, el "decodificar el PDF bloquea el hilo" resultó ser "pintar operaciones indivisibles".
2. **Tres opciones por cuello de botella**, evaluadas con los mismos criterios:

   | | RAM (núcleo / UI) | CPU | Tráfico IPC | Viabilidad con la arquitectura actual |
   |---|---|---|---|---|
3. **Una recomendación** por problema y un **orden** de implementación de lo barato a lo estructural.
4. **Lo que necesita decisión humana**, en una lista explícita (p. ej. "¿modo borrador en las compilaciones
   automáticas?", "¿aceptas el cambio de contrato de `RequestCompile`?").

### 5.3 Decidir y planificar
Las decisiones del usuario se anotan **numeradas y fechadas** en MAP.md ("Decisiones del usuario (2026-11-08)"),
seguidas de un **plan por pasos con casillas**. Una sesión futura sabe así qué está decidido y qué no, sin
reinterpretar la conversación.

### 5.4 Implementar por fases, con prerrequisitos
- Se empieza por lo que **desbloquea** lo demás (aquí, el servicio de configuración antes que `autoCompile`).
- Los **prerrequisitos ocultos** salen a la luz y se arreglan primero. Ejemplo: antes de cancelar compilaciones
  hubo que matar el árbol de procesos completo, porque cancelar latexmk dejaba a xelatex huérfano escribiendo el
  mismo `.aux`.
- Cada mecanismo nuevo lleva **límites contra su propio fallo**. La cancelación, por ejemplo, solo afecta a
  compilaciones automáticas, solo a las jóvenes y nunca dos veces seguidas (contra la inanición).

### 5.5 Medir y cerrar
- Cada análisis incluye un **plan de medición**: documento de prueba, métricas y equipos. Los umbrales
  (p. ej. "más de 60 imágenes") se presentan como **estimaciones que se ajustan con datos**.
- Las pruebas de estrés reales van en una **rama dedicada** y en tests de integración opcionales
  (`CREATIVE_INK_LATEX_TEST=1`), para que `go test ./...` siga siendo rápido.
- Al resolver, se tacha la deuda, se actualiza la tabla "Cómo se comporta hoy" y se mueve lo que falta a
  "Pendientes".

### 5.6 Plantilla de entrada de deuda
```markdown
N. **<Síntoma en una frase>** (detectado el AAAA-MM-DD). Impacto: <usuario/CPU/RAM/seguridad>.
   Causa verificada en el código: <archivo:función y por qué>.
   Opciones: ver §<análisis>. Decisión: <pendiente | opción X, AAAA-MM-DD>.
   Estado: pendiente | parcialmente resuelto (<qué falta>) | ~~resuelto~~ **Resuelto el AAAA-MM-DD:** <cómo>.
```

---

## 6. La verificación como contrato

"Está hecho" significa que **pasó la verificación objetiva**, no que "debería funcionar". La lista mínima de este
proyecto:

| Comprobación | Comando | Qué protege |
|--------------|---------|-------------|
| Formato | `gofmt -l .` (debe estar vacío) | Diffs limpios |
| Análisis estático | `go vet ./...` | Errores de compilación y de uso |
| Multiplataforma | `GOOS=linux go vet ./...` y `GOOS=windows go vet ./...` | Archivos `_<os>.go` y APIs de cada sistema |
| Tests y carreras | `go test -race ./...` | Lógica y concurrencia (goroutines, mutex) |
| Estabilidad | `go test -run <Test> -count=20` | Tests de concurrencia sin falsos positivos |
| Build | `go build -o /tmp/<app>-check .` | El binario enlaza |
| UI | `node .claude/skills/ui-svelte-architect/scripts/check-svelte.mjs` | Componentes y runes compilan, avisos de accesibilidad |
| Diseño | Checklist A5 de la skill de UI (`grep` de colores fijos, `transition: all`…) | Coherencia visual y de temas |
| Integración real | Tests opcionales con variable de entorno (los ejecuta el humano) | La cadena de herramientas real |
| App real | `wails dev` (lo ejecuta el humano) | Lo que ningún test cubre |

Prácticas que hicieron esto fiable:
- **Tests de invariantes que cruzan capas.** Un test de Go lee `style.css` y comprueba que el color nativo de la
  ventana coincide con el token `--surface-base` del tema, para que nunca se desincronicen.
- **Tests que documentan el contrato real** cuando la petición no coincide con el código. Por ejemplo,
  `ResolveInFolder` solo acepta rutas absolutas, y el test lo dice en lugar de forzar lo contrario.
- **Errores encontrados en la verificación, corregidos antes de entregar,** y contados en el informe: un `Write`
  que devolvía mal la longitud, una coma que faltaba, una zona muerta de inicialización.
- **Honestidad sobre lo no verificado:** "no lo he probado en la app real" figura siempre en el informe cuando aplica.

---

## 7. Capturar el conocimiento fino (*gotchas*)

Cada sorpresa técnica se convierte en una línea permanente, en `CLAUDE.md` si afecta a todo, o en la skill de su
dominio:
- **Se verifica en el código fuente de la dependencia** (no en la memoria): por ejemplo, que `RendererWorker`
  pinta en `OffscreenCanvas` y devuelve un `ImageBitmap`, o que Wails convierte un `BackgroundColour` nulo en blanco.
- **Se formula como regla accionable:** "No vuelvas a poner `disableWorkerRendering: true`: el pintado volvería al
  hilo de CodeMirror".
- **Se elimina cuando deja de ser cierto,** igual que la documentación obsoleta.

---

## 8. Plantillas para arrancar un proyecto nuevo

### 8.1 Esqueleto de `CLAUDE.md`
```markdown
# <Proyecto> — Contexto para Claude
<Una línea de producto y plataformas.>
Mapa de arquitectura y skills: @docs/MAP.md

## Stack            (y lo prohibido)
## Ubicación del código
## Visión del MVP   (decisiones tomadas · decisiones PENDIENTES: no asumir)
## Skills del proyecto
## Reglas de trabajo (OBLIGATORIAS): comandos permitidos y prohibidos, idioma, commits, ramas
## Gotchas técnicos
## Estado actual    (tabla de secciones integradas · en curso · pendientes)
```

### 8.2 Esqueleto de `docs/MAP.md`
```markdown
## 1. Mapa de directorios (con "← qué hace" por archivo)
## 2. Reglas de capas (diagrama + prohibiciones)
## 3. Contrato IPC (tabla viva)
## 4.x Un apartado por subsistema (qué, por qué, límites, pendiente)
## 4.y Diseños acordados (decisiones fechadas + plan por pasos)
## 4.z Deuda técnica (numerada, tachada con fecha) y análisis de escalabilidad
## 5. Conocimiento requerido por área
## 6. Comandos que ejecuta el humano
```

### 8.3 Esqueleto de una skill
```markdown
---
name: <dominio-en-kebab-case>
description: <Qué cubre y CUÁNDO usarla, con las palabras con las que se pediría.>
---
# <Nombre>
Read first: CLAUDE.md y docs/MAP.md (ganan si discrepan).
## Project context (verify before relying on it)
## Principles
## Steps for <la tarea típica>
## Verification   (un script de solo lectura, sin instalar nada)
## Gotchas        (verificados en el código fuente)
```

### 8.4 Mensaje de commit
```
<tipo>(<ámbito>): <qué cambia, en español>

<Por qué y cómo, en 1–3 frases. Sin atribución a la IA.>
```
Tipos: `feat`, `fix`, `docs`, `test`, `build`, `chore`. Cada commit compila por sí solo.

---

## 9. Checklist de cierre de una sección

- [ ] Alcance cumplido, y lo que no se tocó, sin tocar.
- [ ] Verificación objetiva en verde (§6), con lo no verificado declarado.
- [ ] Contrato IPC actualizado: bindings, eventos, payloads y semántica.
- [ ] Mapa de directorios actualizado (archivos nuevos o borrados).
- [ ] Sección de MAP.md escrita o reescrita; nada obsoleto contradice al código.
- [ ] Deuda técnica: entradas nuevas registradas y las resueltas tachadas con fecha.
- [ ] `CLAUDE.md`: estado, pendientes y *gotchas* nuevos.
- [ ] Skill del dominio actualizada si se aprendió algo verificable.
- [ ] Commits propuestos en la rama correcta, en el formato acordado y compilables uno a uno.
- [ ] Comandos exactos para el humano (instalar, regenerar bindings, probar en la app).
