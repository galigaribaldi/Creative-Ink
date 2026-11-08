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
	// AutoCompile turns the editor's automatic compilation on or off
	// (`ci config autocompile on|off`). A pointer so that a missing field,
	// e.g. in an older settings.json, means "on" instead of "off".
	AutoCompile *bool `json:"autoCompile,omitempty"`
	// RecentFolders are the last opened project folders, newest first.
	RecentFolders []string `json:"recentFolders,omitempty"`
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

// AutoCompileEnabled is AutoCompile with its default (on).
func (s Settings) AutoCompileEnabled() bool {
	return s.AutoCompile == nil || *s.AutoCompile
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
