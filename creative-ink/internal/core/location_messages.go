package core

import (
	"fmt"
	"slices"
	"strings"
)

// Default messages about where a project lives (docs/MAP.md §4.7). They
// are written here, in Go, so the Terminal CI, the Shell, `ci status` and
// the start screen all say the same thing. The UI only shows them.

// NoticeLevel is how loud a notice is.
type NoticeLevel string

const (
	// NoticeInfo: light, does not affect the files' integrity.
	NoticeInfo NoticeLevel = "info"
	// NoticeWarning: strong, shown with the warning colour.
	NoticeWarning NoticeLevel = "warning"
)

// LocationNotice is one default message.
type LocationNotice struct {
	ID    string      `json:"id"` // stable: "external", "external-git", "synced", "git-copy"
	Level NoticeLevel `json:"level"`
	Title string      `json:"title"`
	Text  string      `json:"text"`
	// Steps are optional instructions, one per line (e.g. how to save in git).
	Steps []string `json:"steps"`
}

// Notice IDs.
const (
	NoticeExternal    = "external"
	NoticeExternalGit = "external-git"
	NoticeSynced      = "synced"
	NoticeGitCopy     = "git-copy"
)

// gitSaveSteps explain how to keep changes in git, from the Shell tab or
// the system terminal.
var gitSaveSteps = []string{
	"git status                     ← qué archivos cambiaron",
	"git add -A                     ← prepara todos los cambios",
	`git commit -m "Describe el cambio"  ← los guarda en el historial local`,
	"git push                       ← los sube al remoto (GitHub, GitLab…)",
	"Para credenciales o el editor del commit, usa «Terminal del sistema» (pestaña Shell).",
}

// externalNotices are the notices for an external folder: light without
// git, strong with git, plus the sync warning when it applies.
func externalNotices(loc ProjectLocation) []LocationNotice {
	var notices []LocationNotice
	if loc.GitRoot != "" {
		notices = append(notices, ExternalGitNotice(loc.Folder, loc.GitRoot))
	} else {
		notices = append(notices, ExternalNotice(loc.Folder))
	}
	if loc.SyncService != "" {
		notices = append(notices, SyncedNotice(loc.SyncService))
	}
	return notices
}

// ExternalNotice (light): a folder outside the repository without git.
// Copying it into the repository is only a suggestion.
func ExternalNotice(folder string) LocationNotice {
	return LocationNotice{
		ID:    NoticeExternal,
		Level: NoticeInfo,
		Title: "Fuera de My-Creative-Ink-Repository",
		Text: fmt.Sprintf("Estás trabajando en %s, fuera del repositorio de Creative-ink. "+
			"Todo funciona igual: la compilación, la Terminal CI y la Shell. "+
			"Sugerencia (opcional): cópialo al repositorio para tenerlo junto a los demás (Inicio → Importar proyecto…).", folder),
		Steps: []string{},
	}
}

// ExternalGitNotice (strong): a folder outside the repository that is
// under git. Copying it is NOT recommended: the copy and the original
// would drift apart, and commits made in one would not reach the other.
func ExternalGitNotice(folder, gitRoot string) LocationNotice {
	where := "Esta carpeta es un repositorio git"
	if gitRoot != folder {
		where = fmt.Sprintf("Esta carpeta está dentro del repositorio git %s", gitRoot)
	}
	return LocationNotice{
		ID:    NoticeExternalGit,
		Level: NoticeWarning,
		Title: "Repositorio git fuera de Creative-ink",
		Text: where + ". Trabaja aquí, en su ubicación: NO se recomienda copiarla al repositorio de Creative-ink, " +
			"porque la copia y el original se separarían y tus commits quedarían en uno solo. " +
			"La compilación, la Terminal CI y la Shell funcionan igual. Guarda tus cambios con git:",
		Steps: gitSaveSteps,
	}
}

// SyncedNotice (strong): the folder is synced by a cloud service.
func SyncedNotice(service string) LocationNotice {
	return LocationNotice{
		ID:    NoticeSynced,
		Level: NoticeWarning,
		Title: "Carpeta sincronizada con " + service,
		Text: fmt.Sprintf("%s subirá cada .aux, .log y .pdf de cada compilación, y puede bloquear archivos mientras "+
			"LaTeX los escribe (errores raros o compilaciones lentas). Si pasa, trabaja en una carpeta local.", service),
		Steps: []string{},
	}
}

// GitCopyNoteName is the note written into a copy of a git project.
const GitCopyNoteName = "COPIA-DE-GIT.md"

// GitCopyNotice (strong) is shown every time a copy of a git project is
// opened: the user works on a copy, and has to save in git (or copy the
// files back) for the original to get the changes. Two variants: the whole
// repository copied with its history, or a part of a repository (no .git).
func GitCopyNotice(info ImportInfo) LocationNotice {
	n := LocationNotice{ID: NoticeGitCopy, Level: NoticeWarning, Title: "Estás trabajando con una COPIA de un proyecto git"}
	if info.WithHistory {
		n.Text = fmt.Sprintf("Este proyecto se copió de %s con su historial (.git). Los cambios que hagas aquí NO llegan al original: "+
			"guárdalos en git y súbelos al remoto desde esta copia; después, en el original, tráelos con git pull:", info.From)
		n.Steps = append(slices.Clone(gitSaveSteps[:4]), "En el original: git pull   ← trae los cambios de la copia")
		return n
	}
	n.Text = fmt.Sprintf("Este proyecto se copió de %s, que forma parte del repositorio git %s. La copia NO tiene historial: "+
		"los cambios que hagas aquí no llegan a git. Para conservarlos en el repositorio:", info.From, info.GitRoot)
	n.Steps = []string{
		"1. Copia los archivos cambiados de vuelta a " + info.From,
		"2. Allí: git add -A  y  git commit -m \"Describe el cambio\"  y  git push",
		"O trabaja directamente en el original: Inicio → Importar proyecto… → Abrir en su ubicación.",
	}
	return n
}

// GitCopyNoteFile is the content of COPIA-DE-GIT.md, so the warning
// travels with the folder, even outside Creative-ink.
func GitCopyNoteFile(info ImportInfo) string {
	n := GitCopyNotice(info)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", n.Title, n.Text)
	b.WriteString("```sh\n")
	for _, s := range n.Steps {
		b.WriteString(s + "\n")
	}
	b.WriteString("```\n\nCreado por Creative-ink al importar el proyecto. Puedes borrar este archivo; el aviso de la terminal sale de .creative-ink-meta.\n")
	return b.String()
}

// ImportNotices are the default messages of the import summary, most
// important first. The size is always reported (decided on 2026-11-10).
func ImportNotices(p ImportPreview) []LocationNotice {
	var out []LocationNotice
	if p.Collection {
		out = append(out, LocationNotice{
			ID: "import-collection", Level: NoticeWarning, Title: "Colección de documentos",
			Text: fmt.Sprintf("Esta carpeta contiene %d documentos en subcarpetas distintas: no se copia como un todo. "+
				"Ábrela en su ubicación, o importa la carpeta de uno de ellos.", p.DocumentCount),
			Steps: []string{},
		})
	}
	switch {
	case p.HasGit:
		out = append(out, LocationNotice{
			ID: "import-git", Level: NoticeWarning, Title: "Repositorio git",
			Text: "Esta carpeta es un repositorio git. NO se recomienda copiarla: la copia y el original se separarían y tus commits " +
				"quedarían en uno solo. Recomendado: «Abrir en su ubicación». Si la copias, se incluye su historial (.git), se añade " +
				GitCopyNoteName + " y la terminal te lo recordará al abrirla.",
			Steps: []string{},
		})
	case p.GitRoot != "":
		out = append(out, LocationNotice{
			ID: "import-git-part", Level: NoticeWarning, Title: "Parte de un repositorio git",
			Text: fmt.Sprintf("Esta carpeta forma parte del repositorio git %s. NO se recomienda copiarla: la copia no tendrá historial "+
				"y sus cambios no llegarán a git. Recomendado: «Abrir en su ubicación». Si la copias, se añade %s y la terminal "+
				"te lo recordará al abrirla.", p.GitRoot, GitCopyNoteName),
			Steps: []string{},
		})
	}
	if !p.Collection {
		size := LocationNotice{
			ID: "import-size", Level: NoticeInfo, Title: "Tamaño de la copia",
			Text: fmt.Sprintf("Se copiarán %d archivos (%s)", p.Files, FormatBytes(p.Bytes)), Steps: []string{},
		}
		if p.HasGit {
			size.Text += fmt.Sprintf(", más el historial git: %d archivos (%s)", p.GitFiles, FormatBytes(p.GitBytes))
		}
		size.Text += ". El original no se modifica."
		if p.Large {
			size.Level = NoticeWarning
			size.Title = "Copia muy grande: puede tardar"
			size.Text += " Son más de 2 GB: la copia puede tardar varios minutos y ocupará ese espacio en el disco. Puedes cancelarla en cualquier momento."
		}
		out = append(out, size)
	}
	if p.ExternalRefCount > 0 {
		examples := []string{}
		for _, r := range p.ExternalRefs {
			if len(examples) < 3 {
				examples = append(examples, r.Ref)
			}
		}
		out = append(out, LocationNotice{
			ID: "import-external", Level: NoticeWarning, Title: "Archivos fuera de la carpeta",
			Text: fmt.Sprintf("%d referencias apuntan fuera de esta carpeta (p. ej. %s). Esos archivos no se copian: en la copia, "+
				"el documento no compilará igual hasta que los añadas a mano.", p.ExternalRefCount, strings.Join(examples, ", ")),
			Steps: []string{},
		})
	}
	if len(p.RCFiles) > 0 {
		out = append(out, LocationNotice{
			ID: "import-rc", Level: NoticeInfo, Title: "Configuración de latexmk",
			Text: "Se copia " + strings.Join(p.RCFiles, " y ") + " pero sin confianza: la primera compilación usa -norc y " +
				"te preguntará si confías en él (es código que latexmk ejecuta).",
			Steps: []string{},
		})
	}
	if len(p.NotUTF8) > 0 {
		out = append(out, LocationNotice{
			ID: "import-encoding", Level: NoticeInfo, Title: "Archivos que no son UTF-8",
			Text: fmt.Sprintf("%d archivos de texto no están en UTF-8 (p. ej. %s): se copian, pero el editor no los abrirá "+
				"hasta convertirlos.", len(p.NotUTF8), p.NotUTF8[0]),
			Steps: []string{},
		})
	}
	if len(p.Symlinks) > 0 {
		out = append(out, LocationNotice{
			ID: "import-symlinks", Level: NoticeInfo, Title: "Enlaces simbólicos",
			Text:  fmt.Sprintf("%d enlaces simbólicos no se copian (podrían apuntar fuera de la carpeta).", len(p.Symlinks)),
			Steps: []string{},
		})
	}
	return out
}

// FormatBytes writes a size for people, with a decimal comma: "476 KB",
// "31,2 MB", "1,8 GB".
func FormatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return strings.Replace(fmt.Sprintf("%.1f GB", float64(n)/(1<<30)), ".", ",", 1)
	case n >= 1<<20:
		return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".", ",", 1)
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", (n+1023)>>10)
	}
	return fmt.Sprintf("%d bytes", n)
}
