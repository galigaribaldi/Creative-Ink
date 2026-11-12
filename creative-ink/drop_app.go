package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"

	"creative-ink/internal/core"
)

// Dropping files and folders from the system (docs/MAP.md §4.7). Wails
// gives Go the real paths (runtime.OnFileDrop, wired in startup); Go never
// switches folders by itself: it classifies the drop, keeps the path under
// an id and tells the UI, which saves pending edits first and then opens or
// shows the "¿Qué desea hacer?" summary (InspectDrop).

// EventFileDrop is drop:paths.
const EventFileDrop = "drop:paths"

// Drop kinds.
const (
	DropOpen     = "open"     // a text file of the open folder: the UI opens it in the pane under the cursor
	DropInPlace  = "in-place" // a project of the repository (or a file of one): opened directly
	DropChoose   = "choose"   // outside the repository: copy or open in place (InspectDrop)
	DropRejected = "rejected" // Message says why
)

// FileDropEvent is drop:paths. Path is only sent for DropOpen (a file of
// the open folder, validated again by OpenFile).
type FileDropEvent struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Extra   int    `json:"extra"` // further items dropped and ignored (one at a time)
	Message string `json:"message"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

var errDropUnknown = errors.New("ese arrastre ya no existe: vuelve a arrastrar el archivo o la carpeta")

// onFileDrop is runtime.OnFileDrop's callback (Wails' event goroutine).
func (a *App) onFileDrop(x, y int, paths []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[drop] panic: %v", r)
		}
	}()
	if ev, ok := a.classifyDrop(x, y, paths); ok {
		a.emit(EventFileDrop, ev)
	}
}

// classifyDrop decides what a drop means. ok is false for drops without
// paths: on macOS, Wails also reports the app's own drags (explorer →
// editor) with an empty path, and those are handled by the page.
func (a *App) classifyDrop(x, y int, paths []string) (FileDropEvent, bool) {
	var clean []string
	for _, p := range paths {
		if p != "" {
			clean = append(clean, filepath.Clean(p))
		}
	}
	if len(clean) == 0 {
		return FileDropEvent{}, false
	}
	path := clean[0]
	ev := FileDropEvent{Name: filepath.Base(path), Extra: len(clean) - 1, X: x, Y: y}
	reject := func(msg string) (FileDropEvent, bool) {
		ev.Kind, ev.Message = DropRejected, msg
		return ev, true
	}
	a.importMu.Lock()
	busy := a.importCancel != nil
	a.importMu.Unlock()
	if busy {
		return reject("Hay una importación en curso: espera a que termine o cancélala.")
	}
	info, err := os.Stat(path)
	if err != nil {
		return reject("No se pudo leer " + ev.Name + ".")
	}
	ev.IsDir = info.IsDir()
	root := a.repository()
	folder, _ := a.workspace()
	single := a.openSingleFile()

	if ev.IsDir {
		switch {
		case path == folder && single == "":
			return reject("Esa carpeta ya está abierta.")
		case core.CheckProjectFolder(root, path) != nil:
			return reject(core.CheckProjectFolder(root, path).Error() + ".")
		}
		ev.ID = a.rememberDrop(core.ImportPreview{Source: path})
		ev.Kind = DropChoose
		if root != "" && isInsideDir(root, path) {
			ev.Kind = DropInPlace // a project of the repository: just open it
		}
		return ev, true
	}

	if !info.Mode().IsRegular() || !core.IsEditableFile(path) {
		return reject("Solo se abren archivos de texto (.tex, .bib, .md…): " + ev.Name + " no lo es.")
	}
	if folder != "" && single == "" {
		if clean, err := core.ResolveInFolder(folder, path); err == nil {
			ev.Kind, ev.Path = DropOpen, clean
			return ev, true
		}
	}
	ev.ID = a.rememberDrop(core.ImportPreview{Source: filepath.Dir(path), File: path})
	ev.Kind = DropChoose
	if repositoryProjectOf(root, path) != "" {
		ev.Kind = DropInPlace
	}
	return ev, true
}

func (a *App) rememberDrop(p core.ImportPreview) string {
	p.ID = newImportID()
	a.rememberImport(p)
	return p.ID
}

// InspectDrop analyses a dropped folder or file for the "¿Qué desea
// hacer?" summary: the same analysis and notices as "Importar proyecto"
// (size, git, references outside…). The path stays in Go under id.
func (a *App) InspectDrop(id string) (core.ImportPreview, error) {
	pending, ok := a.pendingImport(id)
	if !ok {
		return core.ImportPreview{}, errDropUnknown
	}
	root := a.repository()
	var p core.ImportPreview
	var err error
	if pending.File != "" {
		p, err = core.InspectFileImport(root, pending.File)
	} else {
		a.cleanStaleImports(root)
		p, err = core.InspectImport(context.Background(), root, pending.Source)
	}
	if err != nil {
		return core.ImportPreview{}, err
	}
	p.ID = id
	return a.finishPreview(root, p), nil
}

// OpenBlankDocument closes the start screen with an empty document: the
// newest untouched quick document if there is one (so closing it again and
// again does not pile them up), or a new one.
func (a *App) OpenBlankDocument() (WorkspaceState, error) {
	root := a.repository()
	if root == "" {
		return WorkspaceState{}, errors.New("el repositorio central no está configurado")
	}
	if dir, tex := core.ReusableQuickDoc(root, starterDocument); dir != "" {
		return a.openProjectFolder(dir, tex)
	}
	return a.CreateQuickDocument()
}
