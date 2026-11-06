package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// FileManager reads and writes documents on disk.
//
// Writes are serialised (one at a time) and atomic: content goes to a
// sibling temp file that is then renamed over the target. An autosave
// interrupted mid-write never leaves a truncated .tex behind.
type FileManager struct {
	mu sync.Mutex
}

// NewFileManager creates a FileManager.
func NewFileManager() *FileManager {
	return &FileManager{}
}

// SaveDocument writes content to path, creating parent directories if needed.
func (m *FileManager) SaveDocument(content string, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}

// LoadDocument returns the content of path, or "" if the file does not exist yet.
func (m *FileManager) LoadDocument(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return string(data), nil
}
