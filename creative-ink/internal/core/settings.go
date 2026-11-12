package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// settingsVersion allows migrating the file format later.
const settingsVersion = 1

// Settings are the per-user choices made on first run. They live in the
// OS config dir (macOS ~/Library/Application Support, Linux ~/.config,
// Windows %AppData%), never in the repository itself.
type Settings struct {
	Version         int    `json:"version"`
	RepositoryScope string `json:"repositoryScope"` // system.ScopeUser | system.ScopeShared
	RepositoryPath  string `json:"repositoryPath"`  // absolute
	// AutoCompile is the legacy (Section 17) form of the user's
	// compile.auto. It is only read, for migration: the config service
	// writes Config and clears this field.
	AutoCompile *bool `json:"autoCompile,omitempty"`
	// Config is the user layer of the configuration service
	// (internal/config): raw JSON values by key id ("compile.auto": false).
	Config map[string]json.RawMessage `json:"config,omitempty"`
	// RecentFolders are the last opened project folders, newest first.
	RecentFolders []string `json:"recentFolders,omitempty"`
	// TrustedProjects are the folders whose latexmk rc files the user trusts
	// (docs/MAP.md §4.6, step 3): real absolute path → SHA-256 of their
	// .latexmkrc/latexmkrc. If the files change (e.g. after a git pull) the
	// hash no longer matches and the folder is untrusted again.
	TrustedProjects map[string]string `json:"trustedProjects,omitempty"`
	// RestrictedProjects are folders the user explicitly refused to trust:
	// they always compile with latexmk -norc and are not asked about again.
	RestrictedProjects []string `json:"restrictedProjects,omitempty"`
}

// maxRecentFolders bounds Settings.RecentFolders.
const maxRecentFolders = 10

// AddRecentFolder puts folder first in RecentFolders, without duplicates.
func (s *Settings) AddRecentFolder(folder string) {
	recent := []string{folder}
	for _, f := range s.RecentFolders {
		if f != folder && len(recent) < maxRecentFolders {
			recent = append(recent, f)
		}
	}
	s.RecentFolders = recent
}

// legacyAutoCompileKey is the config id that AutoCompile maps to.
const legacyAutoCompileKey = "compile.auto"

// UserConfig is the user configuration layer, with the legacy autoCompile
// field folded in when Config does not set compile.auto itself.
func (s Settings) UserConfig() map[string]json.RawMessage {
	layer := make(map[string]json.RawMessage, len(s.Config)+1)
	for k, v := range s.Config {
		layer[k] = v
	}
	if _, ok := layer[legacyAutoCompileKey]; !ok && s.AutoCompile != nil {
		layer[legacyAutoCompileKey], _ = json.Marshal(*s.AutoCompile)
	}
	return layer
}

// SetUserConfig writes (raw != nil) or removes (raw == nil) a key of the
// user layer. Touching compile.auto drops the legacy field for good.
func (s *Settings) SetUserConfig(key string, raw json.RawMessage) {
	if key == legacyAutoCompileKey {
		s.AutoCompile = nil
	}
	if raw == nil {
		delete(s.Config, key)
		return
	}
	if s.Config == nil {
		s.Config = map[string]json.RawMessage{}
	}
	s.Config[key] = raw
}

// SettingsPath returns <user config dir>/creative-ink/settings.json.
func SettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "creative-ink", "settings.json"), nil
}

// LoadSettings reads path. ok is false when the file does not exist yet
// (first run) or describes no repository.
func LoadSettings(path string) (s Settings, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, fmt.Errorf("read settings %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, false, fmt.Errorf("parse settings %q: %w", path, err)
	}
	if s.RepositoryPath == "" || !filepath.IsAbs(s.RepositoryPath) {
		return Settings{}, false, nil
	}
	return s, true, nil
}

// ReadSettings reads path as is (a missing file is the zero value), even
// before the first-run setup chose a repository.
func ReadSettings(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		return Settings{}, fmt.Errorf("read settings %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("parse settings %q: %w", path, err)
	}
	return s, nil
}

// UpdateSettings reads path (a missing file counts as empty), applies
// mutate and saves it, so changing one field never drops the others.
func UpdateSettings(path string, mutate func(*Settings)) (Settings, error) {
	// Another Creative-ink process may update it at the same time (recents,
	// trust decisions, configuration): read-modify-write under a lock, or
	// one of the two changes is lost.
	unlock, err := LockFile(path + ".lock")
	if err != nil {
		return Settings{}, fmt.Errorf("bloquear la configuración: %w", err)
	}
	defer unlock()
	s, err := ReadSettings(path)
	if err != nil {
		return Settings{}, err
	}
	mutate(&s)
	if err := SaveSettings(path, s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SaveSettings writes path atomically (temp file + rename).
func SaveSettings(path string, s Settings) error {
	s.Version = settingsVersion
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return NewFileManager().SaveDocument(string(data)+"\n", path)
}
