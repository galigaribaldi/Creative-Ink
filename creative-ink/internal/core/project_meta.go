package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MetaFileName stores per-project settings inside the project folder. It is
// hidden (leading dot), so ScanTree never lists it.
const MetaFileName = ".creative-ink-meta"

const projectMetaVersion = 1

// ProjectMeta is the content of MetaFileName.
type ProjectMeta struct {
	Version int `json:"version"`
	// MainFile is the root document compiled for the whole project, relative
	// to the folder with forward slashes ("tesis.tex", "src/main.tex"), so
	// the project can be moved or shared across machines and OSes.
	MainFile string `json:"mainFile,omitempty"`
	// Config is the project layer of the configuration service
	// (internal/config): raw JSON values by key id. It overrides the user's.
	Config map[string]json.RawMessage `json:"config,omitempty"`
}

// LoadProjectMeta reads folder/.creative-ink-meta. A missing file is not an
// error: it returns the zero value.
func LoadProjectMeta(folder string) (ProjectMeta, error) {
	data, err := os.ReadFile(filepath.Join(folder, MetaFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return ProjectMeta{}, nil
	}
	if err != nil {
		return ProjectMeta{}, fmt.Errorf("read project settings: %w", err)
	}
	var meta ProjectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return ProjectMeta{}, fmt.Errorf("parse %s: %w", MetaFileName, err)
	}
	return meta, nil
}

// SaveProjectMeta writes folder/.creative-ink-meta atomically.
func SaveProjectMeta(folder string, meta ProjectMeta) error {
	meta.Version = projectMetaVersion
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return NewFileManager().SaveDocument(string(data)+"\n", filepath.Join(folder, MetaFileName))
}

// MainFilePath resolves meta.MainFile to an absolute path, or "" when unset,
// not a .tex, outside the folder (a hand-edited "../x.tex") or missing on
// disk. Callers then fall back to the active document.
func (m ProjectMeta) MainFilePath(folder string) string {
	if m.MainFile == "" || filepath.IsAbs(filepath.FromSlash(m.MainFile)) {
		return ""
	}
	path := filepath.Join(folder, filepath.FromSlash(m.MainFile))
	clean, err := ResolveInFolder(folder, path)
	if err != nil || !IsTeXFile(clean) || !isRegularFile(clean) {
		return ""
	}
	return clean
}

// RelativeMainFile converts an absolute main file inside folder to the
// portable form stored in ProjectMeta.
func RelativeMainFile(folder, path string) (string, error) {
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
