package ci

import (
	"fmt"
	"path/filepath"
	"strings"

	"creative-ink/internal/config"
	"creative-ink/internal/core"
)

// runStatus is `ci status`: one screen with what the editor knows about
// the open project, ending with the location notices (the same default
// messages the terminal banner shows).
func runStatus(args []string, env Env) (string, error) {
	if len(args) > 0 {
		return "", usage("ci status no lleva argumentos")
	}
	loc := env.Location
	if loc.Kind == "" {
		return "No hay ningún proyecto abierto. Crea o abre uno desde la pantalla de inicio.\n", nil
	}
	var b strings.Builder
	row := func(label, value string) { fmt.Fprintf(&b, "%-24s %s\n", label+":", value) }

	row("Carpeta", loc.Folder)
	switch loc.Kind {
	case core.LocationRepository:
		row("Ubicación", "proyecto de My-Creative-Ink-Repository")
	case core.LocationQuickDoc:
		row("Ubicación", "documento rápido (tmp-Workspaces)")
	default:
		row("Ubicación", "⚠ fuera de My-Creative-Ink-Repository")
	}
	if loc.Kind == core.LocationExternal {
		switch {
		case loc.GitRoot == "":
			row("Git", "no")
		case loc.GitRoot == loc.Folder:
			row("Git", "sí (esta carpeta es el repositorio)")
		default:
			row("Git", "sí, dentro de "+loc.GitRoot)
		}
		if loc.SyncService != "" {
			row("Sincronización", "⚠ "+loc.SyncService)
		}
	}
	if env.Target != "" {
		rel, err := filepath.Rel(loc.Folder, env.Target)
		if err != nil {
			rel = env.Target
		}
		row("Objetivo de compilación", filepath.ToSlash(rel))
	} else {
		row("Objetivo de compilación", "ninguno (abre un .tex)")
	}
	if env.Trust != nil {
		if info, err := env.Trust.Status(); err == nil {
			row("Confianza (.latexmkrc)", trustLabel(info))
		}
	}
	if env.Config != nil {
		for _, e := range env.Config.List() {
			if e.ID == config.CompileAuto {
				row("Compilación automática", fmt.Sprintf("%s (%s)", config.Format(e.Value), scopeLabel(e.Scope)))
			}
		}
	}
	for _, n := range loc.Notices {
		b.WriteString("\n" + FormatNotice(n))
	}
	return b.String(), nil
}

func trustLabel(info core.TrustInfo) string {
	switch {
	case len(info.RCFiles) == 0:
		return "no hay .latexmkrc"
	case info.Status == core.TrustTrusted:
		return "de confianza"
	case info.Status == core.TrustRestricted:
		return "restringida (-norc)"
	}
	return "⚠ sin confianza (-norc · ci trust add)"
}

// FormatNotice renders a location notice as terminal text: "⚠" for the
// strong ones, "ℹ" for the light ones, then the steps indented.
func FormatNotice(n core.LocationNotice) string {
	mark := "ℹ"
	if n.Level == core.NoticeWarning {
		mark = "⚠"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n  %s\n", mark, n.Title, n.Text)
	for _, s := range n.Steps {
		fmt.Fprintf(&b, "    %s\n", s)
	}
	return b.String()
}
