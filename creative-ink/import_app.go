package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"creative-ink/internal/core"
)

// Importing a project (docs/MAP.md §4.7, phase 2). InspectImport opens the
// folder dialog and keeps the chosen path under an id: the UI only ever
// sends that id back, never a path. ImportProject copies in a goroutine and
// reports with import:progress / import:finished; the original folder is
// never modified.

// Import events.
const (
	EventImportProgress = "import:progress"
	EventImportFinished = "import:finished"
)

// importSpaceMargin is left free on the disk after a copy.
const importSpaceMargin = 256 << 20

// importProgressEvery throttles import:progress (plus the final one).
const importProgressEvery = 250 * time.Millisecond

// maxPendingImports bounds the previews kept in memory.
const maxPendingImports = 4

var (
	errImportUnknown = errors.New("ese análisis de importación ya no existe: vuelve a elegir la carpeta")
	errImportBusy    = errors.New("ya hay una importación en curso")
)

// ImportOptions are the choices of the import summary.
type ImportOptions struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MainFile   string `json:"mainFile"`   // one of the preview's mainCandidates ("" = none)
	IncludeGit bool   `json:"includeGit"` // copy the .git history (only if the folder has one)
}

// ImportProgressEvent is import:progress.
type ImportProgressEvent struct {
	ID         string `json:"id"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	TotalFiles int    `json:"totalFiles"`
	TotalBytes int64  `json:"totalBytes"`
}

// ImportFinishedEvent is import:finished. On success the UI opens Folder
// with OpenProject (it is a project of the repository now).
type ImportFinishedEvent struct {
	ID       string `json:"id"`
	Success  bool   `json:"success"`
	Canceled bool   `json:"canceled"`
	Error    string `json:"error"`
	Folder   string `json:"folder"`
}

// InspectImport asks for a folder (native dialog) and analyses it without
// changing anything: size, documents, git, references outside it… The
// summary's notices are Go's default messages. Canceled if the dialog is
// closed.
func (a *App) InspectImport() (core.ImportPreview, error) {
	root := a.repository()
	if root == "" {
		return core.ImportPreview{}, errors.New("el repositorio central no está configurado")
	}
	start, _ := a.workspace()
	if start == "" || isInsideDir(root, start) {
		start = filepath.Dir(root) // usually ~/Documents
	} else {
		start = filepath.Dir(start)
	}
	dir, err := a.pickFolder(start)
	if err != nil {
		return core.ImportPreview{}, fmt.Errorf("importar: %w", err)
	}
	if dir == "" {
		return core.ImportPreview{Canceled: true}, nil
	}
	dir = filepath.Clean(dir)
	if err := core.CheckProjectFolder(root, dir); err != nil {
		return core.ImportPreview{}, err
	}
	a.cleanStaleImports(root)
	p, err := core.InspectImport(context.Background(), root, dir)
	if err != nil {
		return core.ImportPreview{}, err
	}
	if free, err := a.freeBytes(root); err == nil {
		p.FreeBytes = free
		if p.CanCopy && uint64(p.Bytes)+importSpaceMargin > free {
			p.CanCopy = false
			p.CopyBlocked = fmt.Sprintf("no hay espacio en el disco: hacen falta %s y quedan %s",
				core.FormatBytes(p.Bytes), core.FormatBytes(int64(free)))
		}
	}
	p.ID = newImportID()
	p.Notices = core.ImportNotices(p)
	a.rememberImport(p)
	return p, nil
}

// ImportProject starts copying a previewed folder into the repository as
// <repo>/<Slugify(name)>/ and returns at once. The copy goes to a staging
// folder and is renamed at the end, so a failed or canceled import leaves
// nothing behind.
func (a *App) ImportProject(opts ImportOptions) error {
	root := a.repository()
	p, ok := a.pendingImport(opts.ID)
	switch {
	case root == "":
		return errors.New("el repositorio central no está configurado")
	case !ok:
		return errImportUnknown
	case !p.CanCopy:
		return fmt.Errorf("no se puede copiar: %s", p.CopyBlocked)
	case opts.MainFile != "" && !slices.Contains(p.MainCandidates, opts.MainFile):
		return fmt.Errorf("%q no es un documento de esa carpeta", opts.MainFile)
	}
	if opts.MainFile == "" && len(p.MainCandidates) > 0 {
		opts.MainFile = p.MainCandidates[0]
	}
	withGit := opts.IncludeGit && p.HasGit
	dir, err := core.ImportedProjectDir(root, opts.Name)
	if err != nil {
		return err
	}
	need := p.Bytes
	if withGit {
		need += p.GitBytes
	}
	if free, err := a.freeBytes(root); err == nil && uint64(need)+importSpaceMargin > free {
		return fmt.Errorf("no hay espacio en el disco: hacen falta %s y quedan %s", core.FormatBytes(need), core.FormatBytes(int64(free)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.importMu.Lock()
	if a.importCancel != nil {
		a.importMu.Unlock()
		cancel()
		return errImportBusy
	}
	a.importCancel = cancel
	a.importMu.Unlock()

	go a.runImport(ctx, root, dir, p, opts.MainFile, withGit)
	return nil
}

// CancelImport stops the copy in progress (no-op if none).
func (a *App) CancelImport() {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	if a.importCancel != nil {
		a.importCancel()
	}
}

// OpenImportInPlace opens the previewed folder where it is, without
// copying (the recommended option for git repositories and the only one
// for collections of documents).
func (a *App) OpenImportInPlace(id string) (WorkspaceState, error) {
	p, ok := a.pendingImport(id)
	if !ok {
		return WorkspaceState{}, errImportUnknown
	}
	if err := core.CheckProjectFolder(a.repository(), p.Source); err != nil {
		return WorkspaceState{}, err
	}
	doc := core.DefaultTeXFile(p.Source)
	if len(p.MainCandidates) > 0 && !p.Collection {
		doc = filepath.Join(p.Source, filepath.FromSlash(p.MainCandidates[0]))
	}
	a.setWorkspace(p.Source, doc)
	return a.workspaceState()
}

func (a *App) runImport(ctx context.Context, root, dir string, p core.ImportPreview, mainRel string, withGit bool) {
	staging := core.ImportStagingDir(root, p.ID)
	finished := ImportFinishedEvent{ID: p.ID}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[import] panic: %v", r)
			finished.Success, finished.Error = false, fmt.Sprint(r)
		}
		if !finished.Success {
			_ = os.RemoveAll(staging)
		}
		a.importMu.Lock()
		a.importCancel = nil
		a.importMu.Unlock()
		a.emit(EventImportFinished, finished)
	}()

	total := ImportProgressEvent{ID: p.ID, TotalFiles: p.Files, TotalBytes: p.Bytes}
	if withGit {
		total.TotalFiles += p.GitFiles
		total.TotalBytes += p.GitBytes
	}
	_ = os.RemoveAll(staging) // a leftover of a crash
	last := time.Time{}
	err := core.CopyTree(ctx, p.Source, staging, core.CopyOptions{
		HiddenFiles: core.ImportHiddenFiles(),
		IncludeGit:  withGit,
		Progress: func(c core.CopyProgress) {
			if now := time.Now(); now.Sub(last) >= importProgressEvery {
				last = now
				ev := total
				ev.Files, ev.Bytes = c.Files, c.Bytes
				a.emit(EventImportProgress, ev)
			}
		},
	})
	if err == nil {
		err = core.FinishImport(staging, dir, mainRel, p, withGit)
	}
	switch {
	case ctx.Err() != nil:
		finished.Canceled = true
	case err != nil:
		finished.Error = err.Error()
	default:
		finished.Success, finished.Folder = true, dir
		a.forgetImport(p.ID)
	}
}

func (a *App) rememberImport(p core.ImportPreview) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	if a.imports == nil {
		a.imports = map[string]core.ImportPreview{}
	}
	a.imports[p.ID] = p
	a.importOrder = append(a.importOrder, p.ID)
	for len(a.importOrder) > maxPendingImports {
		delete(a.imports, a.importOrder[0])
		a.importOrder = a.importOrder[1:]
	}
}

func (a *App) pendingImport(id string) (core.ImportPreview, bool) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	p, ok := a.imports[id]
	return p, ok
}

func (a *App) forgetImport(id string) {
	a.importMu.Lock()
	defer a.importMu.Unlock()
	delete(a.imports, id)
	a.importOrder = slices.DeleteFunc(a.importOrder, func(s string) bool { return s == id })
}

// cleanStaleImports removes staging folders left by a crash (only when no
// import is running).
func (a *App) cleanStaleImports(root string) {
	a.importMu.Lock()
	running := a.importCancel != nil
	a.importMu.Unlock()
	if !running {
		_ = os.RemoveAll(filepath.Dir(core.ImportStagingDir(root, "x")))
	}
}

func newImportID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// isInsideDir reports whether path is root or below it.
func isInsideDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && (rel == "." || !startsWithParent(rel))
}

func startsWithParent(rel string) bool {
	return len(rel) >= 3 && rel[:2] == ".." && os.IsPathSeparator(rel[2])
}
