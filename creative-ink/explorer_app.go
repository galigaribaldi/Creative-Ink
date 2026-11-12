package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"creative-ink/internal/core"
)

// Creating files and folders from the explorer (docs/MAP.md §4.4, #9, block
// D): "Nuevo archivo" and "Nueva carpeta" in the explorer's header and its
// context menu. The name is normalized like everything the editor creates
// (core.Slugify: "Capítulo Dos" → Capitulo-Dos), it must stay inside the
// open folder and an existing entry is never replaced.

var errBadName = errors.New("el nombre no tiene caracteres válidos (y no puede llevar / ni \\)")

// CreateFolder creates <parent>/<Slugify(name)>/ inside the open folder.
func (a *App) CreateFolder(parent, name string) (WorkspaceState, error) {
	dir, err := a.newEntryPath(parent, name, false)
	if err != nil {
		return WorkspaceState{}, err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return WorkspaceState{}, fmt.Errorf("ya existe %s: elige otro nombre", filepath.Base(dir))
		}
		return WorkspaceState{}, err
	}
	return a.workspaceState()
}

// CreateFile creates an empty text file in parent (.tex if the name has no
// extension) and makes it the active document, like ⌘N's dialog did.
func (a *App) CreateFile(parent, name string) (WorkspaceState, error) {
	path, err := a.newEntryPath(parent, name, true)
	if err != nil {
		return WorkspaceState{}, err
	}
	if err := core.CreateEmptyFile(path); err != nil {
		return WorkspaceState{}, err
	}
	folder, _ := a.workspace()
	if err := a.setWorkspace(folder, path); err != nil {
		return WorkspaceState{}, err
	}
	return a.workspaceState()
}

// newEntryPath validates parent (a folder inside the open one) and builds
// the normalized path of the new entry.
func (a *App) newEntryPath(parent, name string, file bool) (string, error) {
	folder, _ := a.workspace()
	switch {
	case folder == "":
		return "", core.ErrNoFolder
	case a.openSingleFile() != "":
		return "", errSingleFile
	}
	dir, err := core.ResolveInFolder(folder, parent)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%q no es una carpeta del proyecto", filepath.Base(parent))
	}
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, `/\`) {
		return "", errBadName
	}
	ext := ""
	if file {
		ext = strings.ToLower(filepath.Ext(name))
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if ext == "" {
			ext = ".tex"
		}
	}
	stem := core.Slugify(name)
	if stem == "" {
		return "", errBadName
	}
	path := filepath.Join(dir, stem+ext)
	if file && !core.IsEditableFile(path) {
		return "", core.ErrNotEditable
	}
	return core.ResolveInFolder(folder, path)
}

// SearchProject is the sidebar's "Buscar": the matching lines of the text
// files of the open folder (only the loose file in single-file mode). A new
// search cancels the one still running, so typing never piles them up.
func (a *App) SearchProject(q core.SearchQuery) (core.SearchResult, error) {
	folder, _ := a.workspace()
	if folder == "" {
		return core.SearchResult{Files: []core.SearchFileResult{}}, core.ErrNoFolder
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.searchMu.Lock()
	if a.searchCancel != nil {
		a.searchCancel()
	}
	a.searchCancel = cancel
	a.searchMu.Unlock()
	defer cancel()
	res, err := core.SearchFolder(ctx, folder, a.openSingleFile(), q)
	if errors.Is(err, context.Canceled) {
		return core.SearchResult{Files: []core.SearchFileResult{}}, nil // superseded by a newer search
	}
	return res, err
}
