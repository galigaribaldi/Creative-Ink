// Package ci implements the reserved `ci` (Creative Ink) commands of the
// hybrid terminal: native Go replacements for the project's Makefile
// targets (compile variants, cleaning) and app settings. They never
// go through a system shell and do not need `make` to be installed.
//
// It has no Wails dependency: the App injects what it needs through Env.
package ci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"creative-ink/internal/compiler"
	"creative-ink/internal/config"
	"creative-ink/internal/core"
)

// tikzCacheDir is the TikZ externalization cache of the project (Makefile
// `tikz-clean`), relative to the main document's folder.
var tikzCacheDir = filepath.Join("Figures", "TiKz_Libraries", "externalized")

// tikzCachePatterns are the files `ci clean tikz` deletes there.
var tikzCachePatterns = []string{"*.pdf", "*.md5", "*.log", "*.dpth"}

// leftoverExts are removed by `ci clean all` after `latexmk -C`, which does
// not know about them (Makefile `limpiar-todo`).
var leftoverExts = []string{".brf", ".run.xml", ".idx", ".ilg", ".ind", ".blg", ".bbl", ".xdv"}

// chapterPattern restricts `ci compile solo <capítulo>`: the value is spliced
// into TeX code (\def\SoloCapitulo{…}) and into -jobname, so braces,
// backslashes, spaces and `..` are rejected.
var chapterPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`)

// ErrUsage marks a malformed `ci` command; the message explains the usage.
var ErrUsage = errors.New("uso incorrecto")

// Env is what the handler needs from the App.
type Env struct {
	// Target is the absolute path of the document to build: the project's
	// main file, or the active .tex ("" if none).
	Target string
	// Config is the configuration service (nil = `ci config` unavailable).
	Config ConfigService
	// Trust manages the trust of the folder latexmk runs in (nil = `ci
	// trust` unavailable). See docs/MAP.md §4.6, step 3.
	Trust TrustService
	// LatexmkRCArgs go before any latexmk arguments ("-norc -r …" when
	// the folder's .latexmkrc is not trusted; nil otherwise).
	LatexmkRCArgs []string
	// BuildStarted / BuildFinished let the App keep the preview in sync
	// (compile:started / compile:finished). On success (err == nil)
	// BuildFinished runs under the compile lock, right after the build wrote
	// Target's PDF; on failure it receives the error and the report.
	BuildStarted  func()
	BuildFinished func(duration time.Duration, err error, report string)
	// Output receives the full output and the progress of builds while
	// they run (the App spools it and emits terminal:progress). The zero
	// value discards them.
	Output compiler.BuildOutput

	// GuideDir is where `ci guide` writes and compiles the guide
	// (<UserConfigDir>/creative-ink/guide; "" = unavailable).
	GuideDir string
	// ShowGuide asks the App to show the compiled guide in the preview.
	ShowGuide func(pdf string) error
	// OpenTerminal opens the OS's own terminal in the open folder and
	// returns its name (nil = `ci terminal` unavailable).
	OpenTerminal func() (string, error)
	// Location is where the open folder lives (zero Kind = no project),
	// for `ci status`.
	Location core.ProjectLocation

	// Overridable in tests.
	runPasses    func(ctx context.Context, workdir string, passes []compiler.Pass, after func() string, out compiler.BuildOutput) (string, error)
	compileGuide func(ctx context.Context, workdir, filename string) (string, error)
}

// TrustService is the trust of the compile target's folder.
type TrustService interface {
	Status() (core.TrustInfo, error)
	Trust() (core.TrustInfo, error)   // trust the current rc files
	Untrust() (core.TrustInfo, error) // restrict: always -norc
}

// ConfigService is the configuration service as the App exposes it.
type ConfigService interface {
	// List resolves every key (effective value and the layer it comes from).
	List() []config.Entry
	// Set writes value to scope ("" removes the scope's value) and returns
	// the key's new effective entry, which may come from another layer.
	Set(key, value string, scope config.Scope) (config.Entry, error)
}

// IsCommand reports whether a terminal line is a reserved `ci` command
// (first word "ci", ignoring leading spaces).
func IsCommand(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "ci"
}

// ── config ─────────────────────────────────────────────────────────────────

const configUsage = `ci config [list]
  ci config get <clave>
  ci config set <clave> <valor> [--project]
  ci config unset <clave> [--project]
  ci config <clave> [valor] [--project]     (p. ej. ci config autocompile on)`

func runConfig(args []string, env Env) (string, error) {
	if env.Config == nil {
		return "", errors.New("la configuración no está disponible")
	}
	scope := config.ScopeUser
	var rest []string
	for _, a := range args {
		switch a {
		case "--project", "--proyecto":
			scope = config.ScopeProject
		case "--user", "--usuario":
			scope = config.ScopeUser
		default:
			rest = append(rest, a)
		}
	}

	switch {
	case len(rest) == 0 || (rest[0] == "list" && len(rest) == 1):
		return listConfig(env.Config.List()), nil
	case rest[0] == "get" && len(rest) == 2:
		return getConfig(env, rest[1])
	case rest[0] == "set" && len(rest) == 3:
		return setConfig(env, rest[1], rest[2], scope)
	case rest[0] == "unset" && len(rest) == 2:
		return setConfig(env, rest[1], "", scope)
	case rest[0] != "get" && rest[0] != "set" && rest[0] != "unset" && rest[0] != "list" && len(rest) <= 2:
		if len(rest) == 1 {
			return getConfig(env, rest[0])
		}
		return setConfig(env, rest[0], rest[1], scope)
	}
	return "", usage("%s", configUsage)
}

func listConfig(entries []config.Entry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%-14s = %-4s (%s)  %s\n", e.ID, config.Format(e.Value), scopeLabel(e.Scope), e.Description)
	}
	b.WriteString("\nCambiar: ci config set <clave> <valor> [--project] · Quitar: ci config unset <clave> [--project]\n")
	return b.String()
}

func getConfig(env Env, name string) (string, error) {
	k, err := config.Lookup(name)
	if err != nil {
		return "", usage("%v", err)
	}
	for _, e := range env.Config.List() {
		if e.ID == k.ID {
			return fmt.Sprintf("%s = %s (%s)\n  %s\n", e.ID, config.Format(e.Value), scopeLabel(e.Scope), e.Description), nil
		}
	}
	return "", usage("clave %q", name)
}

func setConfig(env Env, name, value string, scope config.Scope) (string, error) {
	e, err := env.Config.Set(name, value, scope)
	switch {
	case errors.Is(err, config.ErrUnknownKey), errors.Is(err, config.ErrValue), errors.Is(err, config.ErrScope):
		return "", usage("%v", err)
	case err != nil:
		return "", err
	}
	var b strings.Builder
	if value == "" {
		fmt.Fprintf(&b, "✓ %s: se quitó el valor de %s → ahora %s (%s)\n", e.ID, scopeLabel(scope), config.Format(e.Value), scopeLabel(e.Scope))
	} else {
		fmt.Fprintf(&b, "✓ %s = %s (%s)\n", e.ID, value, scopeLabel(scope))
	}
	if scope == config.ScopeUser && e.Scope == config.ScopeProject {
		fmt.Fprintf(&b, "⚠ El proyecto la fija en %s y manda sobre tu valor de usuario. Para usar el tuyo: ci config unset %s --project\n",
			config.Format(e.Value), e.ID)
	}
	return b.String(), nil
}

func scopeLabel(s config.Scope) string {
	switch s {
	case config.ScopeUser:
		return "usuario · settings.json"
	case config.ScopeProject:
		return "proyecto · .creative-ink-meta"
	}
	return "por defecto"
}

// ── trust ──────────────────────────────────────────────────────────────────

func runTrust(args []string, env Env) (string, error) {
	if env.Trust == nil {
		return "", errors.New("la confianza no está disponible: abre un proyecto")
	}
	var (
		info core.TrustInfo
		err  error
		note string
	)
	switch {
	case len(args) == 0 || (len(args) == 1 && args[0] == "status"):
		info, err = env.Trust.Status()
	case len(args) == 1 && args[0] == "add":
		info, err = env.Trust.Trust()
		note = "✓ Confías en la configuración de latexmk de esta carpeta: se compila con ella.\n"
		if err == nil && len(info.RCFiles) == 0 {
			note = "No hay .latexmkrc ni latexmkrc en esta carpeta: no hay nada que confiar.\n"
		}
	case len(args) == 1 && args[0] == "remove":
		info, err = env.Trust.Untrust()
		note = "✓ Carpeta restringida: latexmk se ejecuta con -norc (sin su .latexmkrc) y no se volverá a preguntar.\n"
	default:
		return "", usage("ci trust [status|add|remove]")
	}
	if err != nil {
		return "", err
	}
	return note + formatTrust(info), nil
}

func formatTrust(info core.TrustInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Carpeta:  %s\n", info.Folder)
	if len(info.RCFiles) == 0 {
		b.WriteString("Estado:   de confianza (no hay .latexmkrc: ningún código del proyecto se ejecuta)\n")
		return b.String()
	}
	switch info.Status {
	case core.TrustTrusted:
		b.WriteString("Estado:   de confianza: latexmk usa su configuración\n")
	case core.TrustRestricted:
		b.WriteString("Estado:   restringida por ti: latexmk se ejecuta con -norc\n")
	default:
		b.WriteString("Estado:   sin confianza: latexmk se ejecuta con -norc (ci trust add para confiar)\n")
	}
	fmt.Fprintf(&b, "Archivos: %s\n", strings.Join(info.RCFiles, ", "))
	fmt.Fprintf(&b, "Hash actual:   %s\n", info.CurrentHash)
	trusted := info.TrustedHash
	if trusted == "" {
		trusted = "(ninguno)"
	} else if trusted != info.CurrentHash {
		trusted += "  ≠ actual: el archivo cambió desde que confiaste"
	}
	fmt.Fprintf(&b, "Hash confiado: %s\n", trusted)
	return b.String()
}

// ── compile ────────────────────────────────────────────────────────────────

func runCompile(ctx context.Context, args []string, env Env) (string, error) {
	if len(args) == 0 {
		return "", usage("ci compile <rapido|borrador|completo|solo <capítulo>>")
	}
	dir, stem, err := target(env)
	if err != nil {
		return "", err
	}
	texFile := stem + ".tex"
	// -synctex=1 keeps PDF ↔ source navigation working after a manual build.
	xelatex := func(continueOnError bool, extra ...string) compiler.Pass {
		return compiler.Pass{
			Tool:            "xelatex",
			Args:            append([]string{"-interaction=nonstopmode", "-synctex=1"}, extra...),
			ContinueOnError: continueOnError,
		}
	}

	var passes []compiler.Pass
	updatesPreview := true // the build writes <stem>.pdf, the PDF the preview shows
	resultPDF := stem + ".pdf"

	switch args[0] {
	case "rapido":
		// Makefile `rapido`: one XeLaTeX pass (minor changes, no bibliography).
		passes = []compiler.Pass{xelatex(false, texFile)}
	case "borrador":
		// Makefile `borrador`: two passes with \borrador defined (the preamble
		// draws figures as empty boxes). Only works if the document checks it.
		code := `\def\borrador{}\input{` + stem + `}`
		passes = []compiler.Pass{xelatex(true, code), xelatex(false, code)}
	case "completo":
		// Makefile `pdf`: XeLaTeX → BibTeX → XeLaTeX → XeLaTeX.
		passes = []compiler.Pass{
			xelatex(true, texFile),
			{Tool: "bibtex", Args: []string{stem}},
			xelatex(true, texFile),
			xelatex(false, texFile),
		}
	case "solo":
		if len(args) != 2 {
			return "", usage("ci compile solo <capítulo>   (p. ej. 6-Analisis-SocioEspacial/1-Analisis)")
		}
		chapter := args[1]
		if !chapterPattern.MatchString(chapter) || strings.Contains(chapter, "..") {
			return "", usage("capítulo inválido %q: solo letras, números, '.', '_', '-' y '/'", chapter)
		}
		// Makefile `solo` compiles over <stem>.pdf and then renames it, which
		// deletes the full PDF. -jobname writes tesis_<capítulo>.pdf (and its
		// own .aux) instead, leaving the full document untouched.
		job := stem + "_" + chapterName(chapter)
		code := `\def\SoloCapitulo{` + chapter + `}\input{` + stem + `}`
		passes = []compiler.Pass{
			xelatex(true, "-jobname="+job, code),
			xelatex(false, "-jobname="+job, code),
		}
		updatesPreview = false
		resultPDF = job + ".pdf"
	default:
		return "", usage("ci compile <rapido|borrador|completo|solo <capítulo>>")
	}

	if updatesPreview && env.BuildStarted != nil {
		env.BuildStarted()
	}
	out := env.Output
	// "página 143 de 212": the previous build of the same job knows the total.
	out.ExpectedPages = compiler.PagesFromLog(filepath.Join(dir, strings.TrimSuffix(resultPDF, ".pdf")+".log"))
	start := time.Now()
	report, err := env.runPasses(ctx, dir, passes, func() string {
		if updatesPreview && env.BuildFinished != nil {
			env.BuildFinished(time.Since(start), nil, "")
		}
		return fmt.Sprintf("→ PDF: %s (%.1f s en total)\n", resultPDF, time.Since(start).Seconds())
	}, out)
	// A failed build must end the "compiling" state in the UI too.
	if err != nil && updatesPreview && env.BuildFinished != nil {
		env.BuildFinished(time.Since(start), err, report)
	}
	return report, err
}

// chapterName mirrors the Makefile's `dirname $(CAP)`: the chapter folder
// ("6-Analisis-SocioEspacial/1-Analisis" → "6-Analisis-SocioEspacial"), or
// the name itself when there is no folder.
func chapterName(chapter string) string {
	chapter = strings.Trim(chapter, "/")
	if i := strings.Index(chapter, "/"); i > 0 {
		return chapter[:i]
	}
	return chapter
}

// ── clean ──────────────────────────────────────────────────────────────────

func runClean(ctx context.Context, args []string, env Env) (string, error) {
	if len(args) != 1 {
		return "", usage("ci clean <tikz|all>")
	}
	dir, stem, err := target(env)
	if err != nil {
		return "", err
	}
	switch args[0] {
	case "tikz":
		var report string
		compiler.WithCompileLock(func() { report = cleanTikz(dir) })
		return report, nil
	case "all":
		// Makefile `limpiar-todo`: latexmk -C (keeps going if it fails), then
		// the leftovers latexmk does not know about. Deletes the PDF too.
		// latexmk reads the folder's .latexmkrc: -norc when it is not trusted.
		latexmkArgs := append(append([]string{}, env.LatexmkRCArgs...), "-C", stem+".tex")
		passes := []compiler.Pass{{Tool: "latexmk", Args: latexmkArgs, ContinueOnError: true}}
		return env.runPasses(ctx, dir, passes, func() string {
			removed := 0
			for _, ext := range leftoverExts {
				if os.Remove(filepath.Join(dir, stem+ext)) == nil {
					removed++
				}
			}
			return fmt.Sprintf("✓ %d archivos auxiliares adicionales eliminados\n", removed)
		}, env.Output)
	}
	return "", usage("ci clean <tikz|all>")
}

func cleanTikz(dir string) string {
	cache := filepath.Join(dir, tikzCacheDir)
	if info, err := os.Stat(cache); err != nil || !info.IsDir() {
		return fmt.Sprintf("No hay caché TikZ en %s\n", tikzCacheDir)
	}
	removed := 0
	for _, pattern := range tikzCachePatterns {
		matches, _ := filepath.Glob(filepath.Join(cache, pattern))
		for _, m := range matches {
			if os.Remove(m) == nil {
				removed++
			}
		}
	}
	return fmt.Sprintf("✓ %d archivos de la caché TikZ eliminados. La próxima compilación los regenera.\n", removed)
}

// ── helpers ────────────────────────────────────────────────────────────────

// target splits Env.Target into its folder and file stem.
func target(env Env) (dir, stem string, err error) {
	if env.Target == "" {
		return "", "", errors.New("no hay documento: abre un .tex o marca el archivo principal (★)")
	}
	return filepath.Dir(env.Target), strings.TrimSuffix(filepath.Base(env.Target), filepath.Ext(env.Target)), nil
}

func usage(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrUsage, fmt.Sprintf(format, a...))
}
