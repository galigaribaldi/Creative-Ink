package ci

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
)

// CommandDef is one `ci` command of the registry (docs/MAP.md §4.6, step
// 4). `ci help`, the generated guide (`ci guide`) and the Terminal CI's
// Tab completion all come from here, so they never drift from what runs.
type CommandDef struct {
	Name        string // first word after `ci`: "compile"
	Usage       string // "ci compile <rapido|borrador|completo|solo <capítulo>>"
	Description string
	// Subcommands are the concrete forms shown in the help and the guide
	// and offered by Tab completion.
	Subcommands []Subcommand
	// RequiresTrust: only runs in a folder whose .latexmkrc is trusted
	// (future commands that execute project code, e.g. --shell-escape).
	RequiresTrust bool
	Run           func(ctx context.Context, args []string, env Env) (string, error)
}

// Subcommand is one form of a command.
type Subcommand struct {
	Syntax      string // without "ci ": "compile solo <capítulo>"
	Description string
}

// CommandInfo is a CommandDef without its handler: what the UI receives
// (ListTerminalCommands) to complete and explain commands.
type CommandInfo struct {
	Name          string           `json:"name"`
	Usage         string           `json:"usage"`
	Description   string           `json:"description"`
	RequiresTrust bool             `json:"requiresTrust"`
	Subcommands   []SubcommandInfo `json:"subcommands"`
}

// SubcommandInfo adds the text Tab inserts for a form.
type SubcommandInfo struct {
	Syntax      string `json:"syntax"`
	Description string `json:"description"`
	// Completion is Syntax up to its first placeholder ("compile solo "),
	// or Syntax itself when it has none ("compile completo").
	Completion string `json:"completion"`
}

// ErrNeedsTrust is returned for a RequiresTrust command in a folder whose
// .latexmkrc is not trusted.
var ErrNeedsTrust = errors.New("este comando necesita que confíes en la carpeta (ci trust add)")

// registry is filled in init: help's handler reads the registry itself.
var registry []CommandDef

func init() {
	registry = []CommandDef{
		{
			Name: "help", Usage: "ci help", Description: "Lista los comandos internos de Creative Ink",
			Subcommands: []Subcommand{{"help", "Esta ayuda"}},
			Run:         func(context.Context, []string, Env) (string, error) { return helpText(), nil },
		},
		{
			Name:        "compile",
			Usage:       "ci compile <rapido|borrador|completo|solo <capítulo>>",
			Description: "Compilaciones manuales del documento (traducción del Makefile de la tesis)",
			Subcommands: []Subcommand{
				{"compile rapido", "Una pasada de XeLaTeX"},
				{"compile borrador", `Sin imágenes (el preámbulo debe usar \borrador)`},
				{"compile completo", "XeLaTeX → BibTeX → XeLaTeX → XeLaTeX"},
				{"compile solo <capítulo>", `Un capítulo (\SoloCapitulo) → <doc>_<capítulo>.pdf`},
			},
			Run: runCompile,
		},
		{
			Name:        "clean",
			Usage:       "ci clean <tikz|all>",
			Description: "Borra archivos generados",
			Subcommands: []Subcommand{
				{"clean tikz", "Borra la caché de Figures/TiKz_Libraries/externalized"},
				{"clean all", "latexmk -C y auxiliares restantes (incluye el PDF)"},
			},
			Run: runClean,
		},
		{
			Name:        "config",
			Usage:       "ci config [list|get|set|unset] …",
			Description: "Configuración por capas: por defecto < usuario < proyecto",
			Subcommands: []Subcommand{
				{"config list", "Ver la configuración (valor y de dónde sale)"},
				{"config autocompile <on|off>", "Compilación automática (por defecto off); --project solo para este proyecto"},
				{"config get <clave>", "Ver una clave"},
				{"config set <clave> <valor>", "Cambiar un valor (--project para el proyecto)"},
				{"config unset <clave>", "Quitar el valor de una capa (--project para el proyecto)"},
			},
			Run: func(_ context.Context, args []string, env Env) (string, error) { return runConfig(args, env) },
		},
		{
			Name:        "trust",
			Usage:       "ci trust [status|add|remove]",
			Description: "Confianza en el .latexmkrc de la carpeta (sin ella: latexmk -norc)",
			Subcommands: []Subcommand{
				{"trust status", "Estado, archivos y hash actual frente al confiado"},
				{"trust add", "Confiar en el contenido actual del .latexmkrc"},
				{"trust remove", "Restringir la carpeta: siempre -norc, sin volver a preguntar"},
			},
			Run: func(_ context.Context, args []string, env Env) (string, error) { return runTrust(args, env) },
		},
		{
			Name:        "guide",
			Usage:       "ci guide",
			Description: "Genera y compila la guía de comandos en PDF y la muestra en la vista previa",
			Subcommands: []Subcommand{{"guide", "Guía de todos los comandos (se regenera del registro)"}},
			Run:         runGuide,
		},
		{
			Name:        "terminal",
			Usage:       "ci terminal",
			Description: "Abre la terminal del sistema en la carpeta del proyecto",
			Subcommands: []Subcommand{{"terminal", "Terminal nativa (sudo, comandos interactivos, tu shell)"}},
			Run:         runTerminal,
		},
	}
}

// runTerminal is `ci terminal`: the App opens the OS's terminal.
func runTerminal(_ context.Context, args []string, env Env) (string, error) {
	if len(args) > 0 {
		return "", usage("ci terminal no lleva argumentos")
	}
	if env.OpenTerminal == nil {
		return "", errors.New("la terminal del sistema no está disponible")
	}
	name, err := env.OpenTerminal()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("✓ %s abierta en la carpeta del proyecto\n", name), nil
}

// Commands returns the registry, in display order.
func Commands() []CommandDef { return slices.Clone(registry) }

// lookup finds a command by name ("ayuda" is an alias of "help").
func lookup(name string) (CommandDef, bool) {
	if name == "ayuda" {
		name = "help"
	}
	for _, c := range registry {
		if c.Name == name {
			return c, true
		}
	}
	return CommandDef{}, false
}

// Info is the registry's metadata for the UI.
func Info() []CommandInfo {
	out := make([]CommandInfo, 0, len(registry))
	for _, c := range registry {
		info := CommandInfo{Name: c.Name, Usage: c.Usage, Description: c.Description, RequiresTrust: c.RequiresTrust}
		for _, sc := range c.Subcommands {
			info.Subcommands = append(info.Subcommands, SubcommandInfo{
				Syntax: sc.Syntax, Description: sc.Description, Completion: completion(sc.Syntax),
			})
		}
		out = append(out, info)
	}
	return out
}

// completion cuts a syntax at its first placeholder ("<…>" or "[…]"),
// keeping the space so the user types the argument next.
func completion(syntax string) string {
	if i := strings.IndexAny(syntax, "<["); i >= 0 {
		return syntax[:i]
	}
	return syntax
}

// Run executes a `ci …` line and returns the text for the terminal. A
// non-nil error means the command failed (the terminal shows it with exit
// code 1); its output is still meaningful.
func Run(ctx context.Context, line string, env Env) (string, error) {
	if env.runPasses == nil {
		env.runPasses = compiler.RunPasses
	}
	args := strings.Fields(line)[1:] // drop "ci"
	if len(args) == 0 {
		return helpText(), nil
	}
	cmd, ok := lookup(args[0])
	if !ok {
		return helpText(), usage("subcomando desconocido %q", args[0])
	}
	if cmd.RequiresTrust {
		if env.Trust == nil {
			return "", ErrNeedsTrust
		}
		if info, err := env.Trust.Status(); err != nil || info.Status != core.TrustTrusted {
			return "", ErrNeedsTrust
		}
	}
	return cmd.Run(ctx, args[1:], env)
}

// helpText is `ci help`, generated from the registry.
func helpText() string {
	var b strings.Builder
	b.WriteString("Creative Ink — comandos internos (no usan la shell ni make):\n\n")
	for _, c := range registry {
		for _, sc := range c.Subcommands {
			fmt.Fprintf(&b, "  %-32s %s\n", "ci "+sc.Syntax, sc.Description)
		}
	}
	b.WriteString(`
Se aplican al archivo principal (★) o, si no hay, al .tex abierto.
En la Terminal CI se escriben sin "ci"; Tab completa. ci guide genera esta guía en PDF.
La salida completa de cada comando se guarda en disco: "Salida completa" la muestra.
`)
	return b.String()
}
