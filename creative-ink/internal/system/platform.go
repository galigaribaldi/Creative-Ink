package system

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RepositoryName is the central folder Creative-ink creates, inside the
// user's (or the shared) Documents folder, like an Obsidian vault. Every
// name the editor creates uses "-" instead of spaces: some TeX tools that go
// through a shell (minted, TikZ externalization, epstopdf) break on spaces.
const RepositoryName = "My-Creative-Ink-Repository"

// LegacyRepositoryName is the name used before 2026-11-08; repositories
// with it are renamed to RepositoryName at startup (see App.migrate…).
const LegacyRepositoryName = "My Creative Ink Repository"

// Repository scopes chosen on first run.
const (
	ScopeUser   = "user"   // only the current user (their Documents)
	ScopeShared = "shared" // every user of the machine (public Documents)
)

// RepositoryOption is one place where the repository can live.
type RepositoryOption struct {
	Scope     string `json:"scope"`
	Path      string `json:"path"` // absolute; "" if unavailable
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"` // why it is unavailable
}

// RepositoryOptions is the per-OS proposal shown on first run and in the
// system diagnosis. CurrentScope/CurrentPath are filled by the App from the
// saved settings ("" while not configured).
type RepositoryOptions struct {
	OS           string           `json:"os"`     // runtime.GOOS
	OSName       string           `json:"osName"` // macOS, Linux, Windows
	User         RepositoryOption `json:"user"`
	Shared       RepositoryOption `json:"shared"`
	Warnings     []string         `json:"warnings"`
	CurrentScope string           `json:"currentScope"`
	CurrentPath  string           `json:"currentPath"`
}

// locator groups the OS-specific lookups so tests can replace them.
type locator struct {
	goos       string
	userDocs   func() (string, error)
	sharedDocs func() (path string, reason string)
	warnings   func(userDocs string) []string
}

// hostLocator uses the functions of documents_<os>.go.
var hostLocator = locator{
	goos:       runtime.GOOS,
	userDocs:   userDocumentsDir,
	sharedDocs: sharedDocumentsDir,
	warnings:   storageWarnings,
}

// DetectRepositoryOptions proposes where the repository should live on this
// machine, for the current user and, where the OS has one, for all users.
func DetectRepositoryOptions() RepositoryOptions {
	return hostLocator.options()
}

func (l locator) options() RepositoryOptions {
	opts := RepositoryOptions{
		OS:       l.goos,
		OSName:   osName(l.goos),
		User:     RepositoryOption{Scope: ScopeUser},
		Shared:   RepositoryOption{Scope: ScopeShared},
		Warnings: []string{},
	}

	if docs, err := l.userDocs(); err != nil {
		opts.User.Reason = "No se pudo localizar la carpeta de Documentos: " + err.Error()
	} else {
		opts.User.Path = filepath.Join(docs, RepositoryName)
		opts.User.Available = true
		opts.Warnings = append(opts.Warnings, l.warnings(docs)...)
	}

	if shared, reason := l.sharedDocs(); shared != "" {
		opts.Shared.Path = filepath.Join(shared, RepositoryName)
		opts.Shared.Available = true
	} else {
		opts.Shared.Reason = reason
	}
	return opts
}

func osName(goos string) string {
	switch goos {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	}
	return goos
}

// parseXDGDocumentsDir reads XDG_DOCUMENTS_DIR from the content of
// user-dirs.dirs (freedesktop.org spec). It returns "" when the key is
// missing or disabled (set to $HOME itself), so the caller falls back.
//
//	XDG_DOCUMENTS_DIR="$HOME/Documentos"
func parseXDGDocumentsDir(content, home string) string {
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		value, ok := strings.CutPrefix(line, "XDG_DOCUMENTS_DIR=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch {
		case strings.HasPrefix(value, "$HOME"):
			value = filepath.Join(home, strings.TrimPrefix(value, "$HOME"))
		case !filepath.IsAbs(value):
			return "" // the spec only allows $HOME/… or absolute paths
		}
		value = filepath.Clean(value)
		if value == filepath.Clean(home) {
			return "" // "$HOME" means the directory is disabled
		}
		return value
	}
	return ""
}

// xdgDocumentsDir resolves the Linux Documents folder as the file manager
// shows it (it may be localised, e.g. ~/Documentos), defaulting to ~/Documents.
func xdgDocumentsDir(home string) string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" || !filepath.IsAbs(configHome) {
		configHome = filepath.Join(home, ".config")
	}
	if data, err := os.ReadFile(filepath.Join(configHome, "user-dirs.dirs")); err == nil {
		if dir := parseXDGDocumentsDir(string(data), home); dir != "" {
			return dir
		}
	}
	return filepath.Join(home, "Documents")
}
