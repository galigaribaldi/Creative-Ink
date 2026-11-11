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
			"Sugerencia (opcional): copia el proyecto al repositorio para tenerlo junto a los demás.", folder),
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

// GitCopyNotice (strong) is for a copy of a git project inside the
// repository (step "Importar proyecto", phase 2): the user works on a copy
// and must save in git to keep the original in sync.
func GitCopyNotice(original string) LocationNotice {
	return LocationNotice{
		ID:    NoticeGitCopy,
		Level: NoticeWarning,
		Title: "Estás trabajando con una COPIA de un repositorio git",
		Text: fmt.Sprintf("Este proyecto se copió de %s, que está bajo git. Los cambios que hagas aquí NO llegan al original. "+
			"Para no perderlos, guárdalos en git desde esta copia y súbelos al remoto; después, en el original, tráelos con git pull:", original),
		Steps: append(slices.Clone(gitSaveSteps[:4]), "En el original: git pull   ← trae los cambios de la copia"),
	}
}

// GitCopyNoteFile is the note written into such a copy (COPIA-DE-GIT.md)
// so the warning travels with the folder, even outside Creative-ink.
func GitCopyNoteFile(original string) string {
	n := GitCopyNotice(original)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", n.Title, n.Text)
	b.WriteString("```sh\n")
	for _, s := range n.Steps {
		b.WriteString(s + "\n")
	}
	b.WriteString("```\n\nCreado por Creative-ink al importar el proyecto. Puedes borrar este archivo.\n")
	return b.String()
}
