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
	// AutoCompile reads and SetAutoCompile persists the setting.
	AutoCompile    func() (bool, error)
	SetAutoCompile func(on bool) error
	// BuildStarted / BuildFinished let the App keep the preview in sync
	// (compile:started / compile:finished). On success (err == nil)
	// BuildFinished runs under the compile lock, right after the build wrote
	// Target's PDF; on failure it receives the error and the report.
	BuildStarted  func()
	BuildFinished func(duration time.Duration, err error, report string)

	// Overridable in tests.
	runPasses func(ctx context.Context, workdir string, passes []compiler.Pass, after func() string) (string, error)
}

// IsCommand reports whether a terminal line is a reserved `ci` command
// (first word "ci", ignoring leading spaces).
func IsCommand(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "ci"
}

// Run executes a `ci …` line and returns the text for the terminal. A
// non-nil error means the command failed (the terminal shows it with exit
// code 1); its output is still meaningful.
func Run(ctx context.Context, line string, env Env) (string, error) {
	if env.runPasses == nil {
		env.runPasses = compiler.RunPasses
	}

	args := strings.Fields(line)[1:] // drop "ci"
	if len(args) == 0 || args[0] == "help" || args[0] == "ayuda" {
		return helpText, nil
	}
	switch args[0] {
	case "config":
		return runConfig(args[1:], env)
	case "compile":
		return runCompile(ctx, args[1:], env)
	case "clean":
		return runClean(ctx, args[1:], env)
	}
	return helpText, usage("subcomando desconocido %q", args[0])
}

// ── config ─────────────────────────────────────────────────────────────────

func runConfig(args []string, env Env) (string, error) {
	if len(args) == 0 || args[0] != "autocompile" || len(args) > 2 {
		return "", usage("ci config autocompile [on|off]")
	}
	if len(args) == 1 {
		on, err := env.AutoCompile()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("autocompile: %s\n", onOff(on)), nil
	}
	var on bool
	switch args[1] {
	case "on":
		on = true
	case "off":
		on = false
	default:
		return "", usage("ci config autocompile [on|off]")
	}
	if err := env.SetAutoCompile(on); err != nil {
		return "", err
	}
	return fmt.Sprintf("✓ autocompile: %s (guardado en settings.json)\n", onOff(on)), nil
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
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
	start := time.Now()
	report, err := env.runPasses(ctx, dir, passes, func() string {
		if updatesPreview && env.BuildFinished != nil {
			env.BuildFinished(time.Since(start), nil, "")
		}
		return fmt.Sprintf("→ PDF: %s (%.1f s en total)\n", resultPDF, time.Since(start).Seconds())
	})
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
		passes := []compiler.Pass{{Tool: "latexmk", Args: []string{"-C", stem + ".tex"}, ContinueOnError: true}}
		return env.runPasses(ctx, dir, passes, func() string {
			removed := 0
			for _, ext := range leftoverExts {
				if os.Remove(filepath.Join(dir, stem+ext)) == nil {
					removed++
				}
			}
			return fmt.Sprintf("✓ %d archivos auxiliares adicionales eliminados\n", removed)
		})
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

const helpText = `Creative Ink — comandos internos (no usan la shell ni make):

  ci config autocompile [on|off]   Ver o cambiar la compilación automática
  ci compile rapido                Una pasada de XeLaTeX
  ci compile borrador              Sin imágenes (el preámbulo debe usar \borrador)
  ci compile completo              XeLaTeX → BibTeX → XeLaTeX → XeLaTeX
  ci compile solo <capítulo>       Un capítulo (\SoloCapitulo) → <doc>_<capítulo>.pdf
  ci clean tikz                    Borra la caché de Figures/TiKz_Libraries/externalized
  ci clean all                     latexmk -C y auxiliares restantes (incluye el PDF)

Se aplican al archivo principal (★) o, si no hay, al .tex abierto.
Cualquier otra línea se ejecuta en la shell del sistema.
`
