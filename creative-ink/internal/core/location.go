package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LocationKind says where an open folder lives relative to the central
// repository (docs/MAP.md §4.7).
type LocationKind string

const (
	LocationRepository LocationKind = "repository" // <repo>/<Project>/
	LocationQuickDoc   LocationKind = "quickdoc"   // <repo>/tmp-Workspaces/unnamed-tmpN/
	LocationExternal   LocationKind = "external"   // anywhere else
)

// ProjectLocation describes the open folder's location. Go computes it once
// per folder change, before the folder is opened (OpenFolderDialog,
// OpenProject, SaveAsDialog), and the UI only shows its notices: the
// Terminal CI and the Shell get a banner and a line in their history.
type ProjectLocation struct {
	Kind   LocationKind `json:"kind"`
	Folder string       `json:"folder"`
	// GitRoot is the folder holding the .git (the project itself or a
	// parent; "" when it is not in a git repository). Only checked for
	// external folders.
	GitRoot string `json:"gitRoot"`
	// SyncService is the cloud service that syncs the folder ("Dropbox",
	// "iCloud Drive"…; "" when none was recognised).
	SyncService string `json:"syncService"`
	// Notices, most important first (see location_messages.go). Empty for
	// projects of the repository.
	Notices []LocationNotice `json:"notices"`
}

// ErrBroadFolder rejects a folder that is not a project of its own: the
// home folder, Desktop, Downloads, a disk's root, the repository's root…
// Opening one would scan, watch and list everything below it.
var ErrBroadFolder = errors.New("no es una carpeta de proyecto")

// ClassifyLocation computes folder's ProjectLocation. repoRoot is the
// central repository ("" if not set up: every folder is external).
func ClassifyLocation(repoRoot, folder string) ProjectLocation {
	loc := ProjectLocation{Kind: LocationExternal, Folder: folder, Notices: []LocationNotice{}}
	if repoRoot != "" && isInside(repoRoot, folder) {
		loc.Kind = LocationRepository
		if isInside(filepath.Join(repoRoot, QuickDocsDirName), folder) {
			loc.Kind = LocationQuickDoc
		}
		return loc
	}
	home, _ := os.UserHomeDir()
	loc.GitRoot = FindGitRoot(folder, home)
	loc.SyncService = SyncService(realPath(folder), home)
	loc.Notices = externalNotices(loc)
	return loc
}

// FindGitRoot returns the nearest folder, from dir up, that holds a .git
// entry (a directory, or a file in worktrees and submodules); "" if none.
// The home folder and the disk's root are never taken as the root: a
// dotfiles repository in ~ would make every project look versioned.
func FindGitRoot(dir, home string) string {
	for d := filepath.Clean(dir); ; {
		if d == filepath.Clean(home) || isFilesystemRoot(d) {
			return ""
		}
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// syncFolderNames are path components that mark a cloud-synced folder on
// any OS (compared case-insensitively). "OneDrive - Empresa" is OneDrive
// for work or school on Windows.
var syncFolderNames = map[string]string{
	"dropbox":      "Dropbox",
	"onedrive":     "OneDrive",
	"google drive": "Google Drive",
	"googledrive":  "Google Drive",
	"mi unidad":    "Google Drive",
	"my drive":     "Google Drive",
	"icloud drive": "iCloud Drive",
	"iclouddrive":  "iCloud Drive",
	"box":          "Box",
	"box sync":     "Box",
	"pcloud drive": "pCloud",
	"mega":         "MEGA",
	"nextcloud":    "Nextcloud",
	"owncloud":     "ownCloud",
}

// SyncService recognises a cloud-synced folder by its real path: macOS
// File Provider folders (~/Library/Mobile Documents for iCloud,
// ~/Library/CloudStorage/<Service>-<account>), iCloud's "Desktop and
// Documents" sync, and the usual folder names on Linux and Windows
// (~/Dropbox, C:\Users\u\OneDrive - Empresa). It only reads paths.
func SyncService(path, home string) string {
	slash := filepath.ToSlash(path)
	if strings.Contains(slash, "/Library/Mobile Documents/") {
		return "iCloud Drive"
	}
	if i := strings.Index(slash, "/Library/CloudStorage/"); i >= 0 {
		provider := strings.SplitN(slash[i+len("/Library/CloudStorage/"):], "/", 2)[0]
		for prefix, name := range map[string]string{"Dropbox": "Dropbox", "OneDrive": "OneDrive", "GoogleDrive": "Google Drive", "Box": "Box", "pCloud": "pCloud"} {
			if strings.HasPrefix(provider, prefix) {
				return name
			}
		}
		return "un servicio de sincronización"
	}
	if home != "" {
		cloudDocs := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
		for _, sub := range []string{"Desktop", "Documents"} {
			if isInside(filepath.Join(home, sub), path) && dirExists(filepath.Join(cloudDocs, sub)) {
				return "iCloud Drive"
			}
		}
	}
	for _, part := range strings.Split(slash, "/") {
		lower := strings.ToLower(part)
		if name, ok := syncFolderNames[lower]; ok {
			return name
		}
		if strings.HasPrefix(lower, "onedrive - ") {
			return "OneDrive"
		}
	}
	return ""
}

// broadHomeFolders are folders of the home directory that hold many
// unrelated things (English and Spanish names; XDG uses the system language).
var broadHomeFolders = []string{
	"Desktop", "Escritorio", "Downloads", "Descargas", "Documents", "Documentos",
	"Dropbox", "OneDrive", "Google Drive", "iCloud Drive",
}

// CheckProjectFolder rejects folders that are not a project of their own
// (ErrBroadFolder, with the reason): the repository's root (the launcher),
// its .creative-ink and tmp-Workspaces folders, the home folder, its
// Desktop, Downloads and Documents, and the root of a disk or volume.
func CheckProjectFolder(repoRoot, folder string) error {
	folder = filepath.Clean(folder)
	reject := func(why string) error { return &broadFolderError{why: why} }
	if repoRoot != "" {
		switch {
		case folder == filepath.Clean(repoRoot):
			return reject("es la raíz del repositorio (el lanzador): crea o abre un proyecto desde la pantalla de inicio")
		case isInside(filepath.Join(repoRoot, MetaDirName), folder):
			return reject("es la carpeta interna del editor (" + MetaDirName + ")")
		case folder == filepath.Join(repoRoot, QuickDocsDirName):
			return reject("es la carpeta de los documentos rápidos: abre uno de ellos")
		}
	}
	if isFilesystemRoot(folder) || isVolumeRoot(folder) {
		return reject("es la raíz de un disco")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if folder == filepath.Clean(home) {
			return reject("es tu carpeta personal")
		}
		for _, name := range broadHomeFolders {
			if strings.EqualFold(folder, filepath.Join(home, name)) {
				return reject("es tu carpeta " + name + ", con muchas cosas además del proyecto: elige o crea una subcarpeta")
			}
		}
	}
	return nil
}

type broadFolderError struct{ why string }

func (e *broadFolderError) Error() string {
	return "esa carpeta " + ErrBroadFolder.Error() + ": " + e.why
}
func (e *broadFolderError) Unwrap() error { return ErrBroadFolder }

// isVolumeRoot recognises mounted disks: /Volumes/<disk> (macOS),
// /mnt/<disk> and /media/<user>/<disk> or /run/media/<user>/<disk> (Linux).
func isVolumeRoot(dir string) bool {
	slash := filepath.ToSlash(dir)
	parent := filepath.ToSlash(filepath.Dir(dir))
	grand := filepath.ToSlash(filepath.Dir(filepath.Dir(dir)))
	return parent == "/Volumes" || parent == "/mnt" || grand == "/media" || grand == "/run/media" ||
		(slash != "/" && (parent == "/media" || parent == "/run/media"))
}

func isFilesystemRoot(dir string) bool {
	return filepath.Dir(dir) == dir
}

// isInside reports whether path is root or below it (no symlink resolution:
// both come from Go's own state or a native dialog).
func isInside(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func realPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
