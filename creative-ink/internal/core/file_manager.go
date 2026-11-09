package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"
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

// MaxEditableBytes is the largest file the editor opens: CodeMirror and the
// IPC bridge carry the whole text, and no LaTeX source is this big.
const MaxEditableBytes = 8 << 20

var (
	// ErrFileTooLarge is returned by LoadText for files over MaxEditableBytes.
	ErrFileTooLarge = errors.New("el archivo es demasiado grande para el editor (más de 8 MB)")
	// ErrNotUTF8 is returned by LoadText for binary or non-UTF-8 files. They
	// are never opened: the bridge would replace the invalid bytes and the
	// next autosave would corrupt the file (e.g. an old Latin-1 .bib).
	ErrNotUTF8 = errors.New("el archivo no es texto UTF-8; conviértelo antes de editarlo (por ejemplo, iconv -f latin1 -t utf-8)")
)

// LoadText reads an existing plain-text file for the editor. Unlike
// LoadDocument, a missing file is an error, and files that are too large,
// binary or not valid UTF-8 are rejected so an autosave can never mangle them.
func (m *FileManager) LoadText(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("el archivo %q no existe", filepath.Base(path))
	}
	if info.Size() > MaxEditableBytes {
		return "", ErrFileTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", ErrNotUTF8
	}
	return string(data), nil
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
