package ci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
)

// GuideFile is the generated guide, inside Env.GuideDir
// (<UserConfigDir>/creative-ink/guide/).
const GuideFile = "ci-guia.tex"

var spanishMonths = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio",
	"agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// runGuide is `ci guide`: it writes the guide from the registry, compiles
// it with the orchestrator and asks the App to show the PDF.
func runGuide(ctx context.Context, args []string, env Env) (string, error) {
	if len(args) != 0 {
		return "", usage("ci guide")
	}
	if env.GuideDir == "" {
		return "", errors.New("no hay carpeta de configuración para la guía en este sistema")
	}
	if err := os.MkdirAll(env.GuideDir, 0o755); err != nil {
		return "", fmt.Errorf("crear %q: %w", env.GuideDir, err)
	}
	tex := filepath.Join(env.GuideDir, GuideFile)
	commands := Commands()
	if err := core.NewFileManager().SaveDocument(GuideTeX(commands, time.Now()), tex); err != nil {
		return "", err
	}

	compile := env.compileGuide
	if compile == nil {
		compile = compiler.CompileLaTeXContext
	}
	if logs, err := compile(ctx, env.GuideDir, GuideFile); err != nil {
		return logs, fmt.Errorf("compilar la guía: %w", err)
	}
	pdf := compiler.PDFPath(tex)
	shown := ""
	if env.ShowGuide != nil {
		if err := env.ShowGuide(pdf); err != nil {
			return "", fmt.Errorf("mostrar la guía: %w", err)
		}
		shown = "  Se muestra en la vista previa hasta la próxima compilación.\n"
	}
	return fmt.Sprintf("✓ Guía generada con %d comandos: %s\n%s", len(commands), pdf, shown), nil
}

// GuideTeX is the LaTeX source of the guide: every registered command with
// its usage, description and forms. It needs no package beyond the LaTeX
// kernel (works with BasicTeX), and XeLaTeX reads its UTF-8 directly.
func GuideTeX(commands []CommandDef, now time.Time) string {
	date := fmt.Sprintf("%d de %s de %d", now.Day(), spanishMonths[now.Month()-1], now.Year())
	var b strings.Builder
	b.WriteString(`\documentclass[11pt]{article}
\title{Guía de comandos \texttt{ci} de Creative-ink}
\author{Generada automáticamente desde el registro de comandos}
\date{` + date + `}
\begin{document}
\maketitle

Los comandos \texttt{ci} los resuelve Creative-ink por dentro: no usan la shell del
sistema ni \texttt{make}. Se aplican al archivo principal del proyecto o, si no hay,
al \texttt{.tex} abierto. En la pestaña \emph{Terminal CI} se escriben sin el prefijo
\texttt{ci} y la tecla Tab los completa.

No edites este archivo: \texttt{ci guide} lo vuelve a generar a partir del registro.
`)
	for _, c := range commands {
		fmt.Fprintf(&b, "\n\\section*{\\texttt{ci %s}}\n%s.\n\n\\noindent Uso: \\texttt{%s}",
			guideText(c.Name), guideText(strings.TrimSuffix(c.Description, ".")), guideText(c.Usage))
		if c.RequiresTrust {
			b.WriteString(`\\ \emph{Requiere confiar en la carpeta (\texttt{ci trust add}).}`)
		}
		b.WriteString("\n")
		if len(c.Subcommands) > 0 {
			b.WriteString("\\begin{description}\n")
			for _, sc := range c.Subcommands {
				fmt.Fprintf(&b, "  \\item[\\texttt{ci %s}] %s\n", guideText(sc.Syntax), guideText(sc.Description))
			}
			b.WriteString("\\end{description}\n")
		}
	}
	b.WriteString("\n\\end{document}\n")
	return b.String()
}

// guideText escapes registry text for LaTeX and maps the symbols Latin
// Modern lacks to math equivalents.
func guideText(s string) string {
	return strings.NewReplacer("→", `$\to$`, "★", `$\star$`).Replace(core.EscapeTeX(s))
}
