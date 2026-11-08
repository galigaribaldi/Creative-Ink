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

// SaveSettings writes path atomically (temp file + rename).
func SaveSettings(path string, s Settings) error {
	s.Version = settingsVersion
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return NewFileManager().SaveDocument(string(data)+"\n", path)
}
