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

// ActiveDocument is SetActiveDocument's answer.
type ActiveDocument struct {
	CompileTarget string `json:"compileTarget"`
	AffectsBuild  bool   `json:"affectsBuild"`
}

// SetActiveDocument makes path (a text file of the open folder, already
// shown in an editor pane) Go's active document, without reading it: the
// user focused that pane (docs/MAP.md §4.4, #9, several panes). What is
// compiled follows the last focused .tex (its root document); a main file
// (★) still wins. When the target changes, it is compiled (or its PDF on
// disk shown) like opening a file.
func (a *App) SetActiveDocument(path string) (ActiveDocument, error) {
	folder, _ := a.workspace()
	clean, err := a.validateEditablePath(folder, path)
	if err != nil {
		return ActiveDocument{}, err
	}
	before := a.compileTarget()
	if err := a.setWorkspaceMode(folder, clean, a.openSingleFile()); err != nil {
		return ActiveDocument{}, err
	}
	after := a.compileTarget()
	if after != "" && after != before {
		a.compileOnOpen()
	}
	return ActiveDocument{CompileTarget: after, AffectsBuild: core.AffectsBuild(after, clean)}, nil
}

// MoveResult is MoveEntry's answer: the new state, and the paths the UI
// renames in its open panes and tabs (a file, or a folder and everything
// below it).
type MoveResult struct {
	State WorkspaceState `json:"state"`
	From  string         `json:"from"`
	To    string         `json:"to"`
}

// MoveEntry moves a file or folder of the open folder into another folder
// of it (drag & drop in the explorer). It never replaces an entry, never
// moves a folder into itself, and keeps what depends on the path: the main
// file (.creative-ink-meta) and Go's active document. The UI saves pending
// edits before calling it, so no late autosave can recreate the old path.
func (a *App) MoveEntry(from, toDir string) (MoveResult, error) {
	folder, _ := a.workspace()
	switch {
	case folder == "":
		return MoveResult{}, core.ErrNoFolder
	case a.openSingleFile() != "":
		return MoveResult{}, errSingleFile
	}
	src, err := core.ResolveInFolder(folder, from)
	if err != nil {
		return MoveResult{}, err
	}
	dir, err := core.ResolveInFolder(folder, toDir)
	if err != nil {
		return MoveResult{}, err
	}
	if src == folder {
		return MoveResult{}, errors.New("no se puede mover la carpeta del proyecto")
	}
	if _, err := os.Lstat(src); err != nil {
		return MoveResult{}, fmt.Errorf("%s ya no existe", filepath.Base(src))
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return MoveResult{}, fmt.Errorf("%q no es una carpeta del proyecto", filepath.Base(toDir))
	}
	if dir == src || isInsideDir(src, dir) {
		return MoveResult{}, errors.New("no se puede mover una carpeta dentro de sí misma")
	}
	dest := filepath.Join(dir, filepath.Base(src))
	if dest == src {
		state, err := a.workspaceState()
		return MoveResult{State: state, From: src, To: src}, err // already there
	}
	if _, err := os.Lstat(dest); err == nil {
		return MoveResult{}, fmt.Errorf("ya existe %s en %s: no se reemplaza", filepath.Base(src), filepath.Base(dir))
	}
	if err := os.Rename(src, dest); err != nil {
		return MoveResult{}, fmt.Errorf("mover %s: %w", filepath.Base(src), err)
	}

	// Paths that pointed into what moved now point to its new place.
	moved := func(p string) string {
		if p == src {
			return dest
		}
		if isInsideDir(src, p) {
			rel, _ := filepath.Rel(src, p)
			return filepath.Join(dest, rel)
		}
		return p
	}
	a.mu.Lock()
	a.docPath, a.texDoc, a.texTarget = moved(a.docPath), moved(a.texDoc), moved(a.texTarget)
	oldMain := a.mainFile
	a.mainFile = moved(a.mainFile)
	newMain := a.mainFile
	a.mu.Unlock()
	if newMain != oldMain && newMain != "" {
		meta, _ := core.LoadProjectMeta(folder)
		if rel, err := core.RelativeMainFile(folder, newMain); err == nil {
			meta.MainFile = rel
			if err := core.SaveProjectMeta(folder, meta); err != nil {
				return MoveResult{}, fmt.Errorf("guardar el archivo principal: %w", err)
			}
		}
	}
	state, err := a.workspaceState()
	return MoveResult{State: state, From: src, To: dest}, err
}
