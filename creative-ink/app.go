package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
	"creative-ink/internal/system"
)

// starterDocument is written to the central repository when it has no .tex yet.
const starterDocument = "\\documentclass{article}\n\\begin{document}\n\nHola, Creative-ink.\n\n\\end{document}\n"

// Wails events emitted to the frontend (documented in docs/MAP.md §3).
const (
	// EventCompileStarted carries a CompileStartedEvent.
	EventCompileStarted = "compile:started"
	// EventCompileFinished carries a CompileFinishedEvent.
	EventCompileFinished = "compile:finished"
	// EventAppBeforeClose has no payload: the window is closing, the UI must
	// flush pending edits and answer with ConfirmClose.
	EventAppBeforeClose = "app:before-close"
	// EventWorkspaceChanged carries a WorkspaceChangedEvent: files or folders
	// were created, removed or renamed outside the app.
	EventWorkspaceChanged = "workspace:changed"
)

// shutdownTimeout bounds how long closing the app waits for latexmk to die.
const shutdownTimeout = 3 * time.Second

// closeFlushTimeout bounds how long closing the window waits for the UI to
// flush CodeMirror to disk before quitting anyway (a hung UI must not keep
// the app open forever).
const closeFlushTimeout = 2 * time.Second

const (
	closeAnywayButton = "Cerrar sin guardar"
	cancelCloseButton = "Cancelar"
)

var (
	errNoDocument = errors.New("no hay ningún documento .tex abierto")
	errNoContext  = errors.New("la aplicación todavía no terminó de iniciar")
)

// App is the single IPC facade exposed to the frontend.
// Every exported method becomes a Wails binding (window.go.main.App.*).
// Business logic lives in internal/*; App only coordinates and delegates.
type App struct {
	ctx      context.Context
	files    *core.FileManager
	compiles *compiler.Scheduler
	// emit sends a Wails event. Injected so tests never reach the Wails
	// runtime (runtime.EventsEmit calls log.Fatalf without a Wails context).
	emit func(name string, data ...any)
	// Native dialogs, injected for the same reason. "" means cancelled.
	pickFolder   func(defaultDir string) (string, error)
	pickSavePath func(defaultDir, defaultName string) (string, error)
	// repoOptions proposes repository locations for this OS (injectable).
	repoOptions func() system.RepositoryOptions
	// settingsPath is the per-user settings file ("" = cannot persist).
	settingsPath string

	// mu guards folder and docPath: bound calls each run on their own
	// goroutine, and the compile worker and preview middleware read them too.
	mu      sync.RWMutex
	folder  string // absolute path of the open folder
	docPath string // absolute path of the active .tex ("" if none)
	// mainFile is the project's root document (absolute, "" if none), read
	// from <folder>/.creative-ink-meta whenever the folder changes. When set,
	// it is compiled instead of the active document.
	mainFile string

	// Safe-close protocol (see beforeClose). quit and confirmQuit are
	// injected: the real ones need a Wails context.
	closeMu      sync.Mutex
	closing      bool // waiting for the UI to flush
	closeAllowed bool // the next OnBeforeClose lets the window close
	closeTimer   *time.Timer
	closeTimeout time.Duration
	quit         func()
	confirmQuit  func(message string) bool

	// watchFolder starts a filesystem watcher (nil in tests = no watching).
	watchFolder func(root string, onChange func()) (io.Closer, error)
	// watchMu guards watcher and treeSig.
	watchMu sync.Mutex
	watcher io.Closer
	treeSig uint64 // signature of the last tree seen, to drop no-op events

	// compilingDoc is the document of the compilation in progress. It is only
	// touched by the scheduler's worker goroutine (OnStarted → compile →
	// OnFinished run sequentially there), so it needs no lock.
	compilingDoc string
}

// SaveResult is what the frontend receives after every save. Compilation is
// reported separately through EventCompileStarted / EventCompileFinished.
type SaveResult struct {
	Saved bool `json:"saved"`
}

// WorkspaceState describes the open folder for the explorer.
type WorkspaceState struct {
	// NeedsSetup: first run, the user has not chosen where the central
	// repository lives yet (see GetRepositoryOptions / SetupRepository).
	NeedsSetup bool   `json:"needsSetup"`
	Canceled   bool   `json:"canceled"`   // the user closed the dialog
	FolderPath string `json:"folderPath"` // absolute
	FilePath   string `json:"filePath"`   // absolute active .tex, "" if none
	// MainFilePath is the project's root document ("" if none). When set it
	// is what gets compiled and previewed, whatever file is being edited.
	MainFilePath string        `json:"mainFilePath"`
	Tree         core.FileNode `json:"tree"`
	Truncated    bool          `json:"truncated"` // the tree hit its size limits
}

// OpenedFile is a document loaded into the editor.
type OpenedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WorkspaceChangedEvent is the payload of EventWorkspaceChanged. Folder lets
// the UI ignore an event for a folder it already left.
type WorkspaceChangedEvent struct {
	Folder string `json:"folder"`
}

// CompileStartedEvent is the payload of EventCompileStarted.
type CompileStartedEvent struct {
	File string `json:"file"`
}

// CompileFinishedEvent is the payload of EventCompileFinished: the compiler
// result plus the document it belongs to, so the UI can ignore results for
// a file that is no longer active.
type CompileFinishedEvent struct {
	compiler.Result
	File string `json:"file"`
}

// NewApp creates the App facade. No folder is open until startup prepares
// the central repository (~/Documents/My Creative Ink Repository).
func NewApp() *App {
	a := newApp("", "", compiler.CompileLaTeXContext)
	if path, err := core.SettingsPath(); err == nil {
		a.settingsPath = path
	} else {
		log.Printf("[settings] la configuración no se guardará: %v", err)
	}
	a.emit = a.emitWails
	a.pickFolder = a.dialogPickFolder
	a.pickSavePath = a.dialogPickSavePath
	a.watchFolder = func(root string, onChange func()) (io.Closer, error) {
		return core.WatchFolder(root, core.WatchDebounce, onChange)
	}
	a.quit = a.quitWails
	a.confirmQuit = a.dialogConfirmQuit
	return a
}

// newApp wires the facade; tests call it with a fake compile function.
func newApp(folder, docPath string, compile func(ctx context.Context, workdir, filename string) (string, error)) *App {
	a := &App{
		files:   core.NewFileManager(),
		folder:  folder,
		docPath: docPath,
		emit:    func(string, ...any) {},
		quit:    func() {},
		// Overridden by tests; production detects the host OS.
		repoOptions:  system.DetectRepositoryOptions,
		closeTimeout: closeFlushTimeout,
		confirmQuit:  func(string) bool { return true },
	}
	if folder != "" {
		a.mainFile = loadMainFile(folder)
	}
	a.compiles = compiler.NewScheduler(
		func(ctx context.Context) (string, error) {
			if a.compilingDoc == "" {
				return "", errNoDocument
			}
			return compile(ctx, filepath.Dir(a.compilingDoc), filepath.Base(a.compilingDoc))
		},
		compiler.Hooks{
			OnStarted: func() {
				// Snapshot the target at the moment the run starts: coalesced
				// requests always compile the latest one. With a main file set,
				// that is the main file, whatever chapter is being edited.
				a.compilingDoc = a.compileTarget()
				a.emit(EventCompileStarted, CompileStartedEvent{File: a.compilingDoc})
			},
			OnFinished: func(r compiler.Result) {
				a.emit(EventCompileFinished, CompileFinishedEvent{Result: r, File: a.compilingDoc})
			},
		},
	)
	return a
}

// startup is called by Wails when the app starts. The context is kept to
// use the Wails runtime (events, dialogs) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := a.openConfiguredRepository(); err != nil {
		// Not fatal: GetWorkspace retries and reports the error to the UI.
		log.Printf("[workspace] no se pudo abrir el repositorio central: %v", err)
	}
}

// shutdown is called by Wails when the app is closing: stop latexmk so it
// does not outlive the window.
func (a *App) shutdown(_ context.Context) {
	a.stopWatching()
	a.compiles.Close(shutdownTimeout)
}

// openConfiguredRepository opens the central repository chosen on first
// run, recreating it (and its starter main.tex) if it was deleted. configured
// is false on first run: nothing is created until the user chooses a scope.
// A folder the user opened in the meantime is never replaced.
func (a *App) openConfiguredRepository() (configured bool, err error) {
	if a.settingsPath == "" {
		return false, nil
	}
	settings, ok, err := core.LoadSettings(a.settingsPath)
	if err != nil {
		// A corrupt settings file must not lock the user out: ask again.
		log.Printf("[settings] %v; se pedirá elegir el repositorio de nuevo", err)
		return false, nil
	}
	if !ok {
		return false, nil
	}
	shared := settings.RepositoryScope == system.ScopeShared
	doc, err := core.PrepareRepository(settings.RepositoryPath, starterDocument, shared)
	if err != nil {
		return true, err
	}

	main := loadMainFile(settings.RepositoryPath)
	a.mu.Lock()
	opened := a.folder == ""
	if opened {
		a.folder, a.docPath, a.mainFile = settings.RepositoryPath, doc, main
	}
	a.mu.Unlock()
	if opened {
		a.restartWatcher()
	}
	return true, nil
}

// currentRepository returns the saved scope and path ("" if not configured).
func (a *App) currentRepository() (scope, path string) {
	if a.settingsPath == "" {
		return "", ""
	}
	if s, ok, err := core.LoadSettings(a.settingsPath); err == nil && ok {
		return s.RepositoryScope, s.RepositoryPath
	}
	return "", ""
}

// ── Bindings ───────────────────────────────────────────────────────────────

// SystemHealthCheck audits the host LaTeX toolchain (xelatex, latexmk, biber).
// The report includes the OS and the proposed repository locations.
func (a *App) SystemHealthCheck() system.HealthStatus {
	status := system.CheckHealth()
	status.Repository.CurrentScope, status.Repository.CurrentPath = a.currentRepository()
	return status
}

// GetRepositoryOptions proposes, for this OS, where the central repository
// can live: the user's Documents and, on macOS and Windows, the Documents
// folder shared by all users. Used by the first-run screen.
func (a *App) GetRepositoryOptions() system.RepositoryOptions {
	opts := a.repoOptions()
	opts.CurrentScope, opts.CurrentPath = a.currentRepository()
	return opts
}

// SetupRepository creates the central repository for scope ("user" or
// "shared"), remembers the choice and opens it. The path is always the one
// Go computed for this OS: the frontend only picks the scope.
func (a *App) SetupRepository(scope string) (WorkspaceState, error) {
	opts := a.repoOptions()
	var option system.RepositoryOption
	switch scope {
	case system.ScopeUser:
		option = opts.User
	case system.ScopeShared:
		option = opts.Shared
	default:
		return WorkspaceState{}, fmt.Errorf("ámbito de repositorio inválido %q", scope)
	}
	if !option.Available {
		return WorkspaceState{}, fmt.Errorf("opción no disponible en %s: %s", opts.OSName, option.Reason)
	}

	doc, err := core.PrepareRepository(option.Path, starterDocument, scope == system.ScopeShared)
	if err != nil {
		return WorkspaceState{}, err
	}
	if a.settingsPath != "" {
		settings := core.Settings{RepositoryScope: scope, RepositoryPath: option.Path}
		if err := core.SaveSettings(a.settingsPath, settings); err != nil {
			return WorkspaceState{}, fmt.Errorf("guardar la configuración: %w", err)
		}
	}
	a.setWorkspace(option.Path, doc)
	a.compiles.Request()
	return a.workspaceState()
}

// GetWorkspace returns the open folder, its tree and the active document.
// With no folder open yet it opens the configured central repository, or
// reports NeedsSetup on first run (nothing is created on disk until then).
func (a *App) GetWorkspace() (WorkspaceState, error) {
	if folder, _ := a.workspace(); folder == "" {
		configured, err := a.openConfiguredRepository()
		if err != nil {
			return WorkspaceState{}, fmt.Errorf("abrir el repositorio central: %w", err)
		}
		if !configured {
			return WorkspaceState{NeedsSetup: true}, nil
		}
	}
	return a.workspaceState()
}

// RefreshWorkspace rescans the open folder (e.g. after creating files outside the app).
func (a *App) RefreshWorkspace() (WorkspaceState, error) {
	return a.workspaceState()
}

// OpenFolderDialog lets the user pick a folder with the native dialog, opens
// it and selects its default document (main.tex or the first .tex).
func (a *App) OpenFolderDialog() (WorkspaceState, error) {
	current, _ := a.workspace()
	dir, err := a.pickFolder(current)
	if err != nil {
		return WorkspaceState{}, fmt.Errorf("abrir carpeta: %w", err)
	}
	if dir == "" {
		return WorkspaceState{Canceled: true}, nil
	}
	dir = filepath.Clean(dir)
	a.setWorkspace(dir, core.DefaultTeXFile(dir))
	return a.workspaceState()
}

// OpenFile loads a .tex inside the open folder into the editor, makes it the
// active document and queues its compilation.
func (a *App) OpenFile(path string) (OpenedFile, error) {
	folder, _ := a.workspace()
	clean, err := a.validateTeXPath(folder, path)
	if err != nil {
		return OpenedFile{}, err
	}
	if info, err := os.Stat(clean); err != nil || !info.Mode().IsRegular() {
		return OpenedFile{}, fmt.Errorf("el archivo %q no existe", filepath.Base(clean))
	}
	content, err := a.files.LoadDocument(clean)
	if err != nil {
		return OpenedFile{}, err
	}

	a.setWorkspace(folder, clean)
	a.compiles.Request()
	return OpenedFile{Path: clean, Content: content}, nil
}

// SaveContent writes content to path (a .tex inside the open folder) and
// returns immediately. The path travels with every save so a late autosave
// can never land in another file. It does NOT compile: the UI calls
// RequestCompile after 3 s without typing or on Cmd/Ctrl+S, so typing does
// not keep the compile worker busy. A failure rejects the promise.
func (a *App) SaveContent(path string, content string) (SaveResult, error) {
	folder, _ := a.workspace()
	clean, err := a.validateTeXPath(folder, path)
	if err != nil {
		return SaveResult{}, err
	}
	if err := a.files.SaveDocument(content, clean); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Saved: true}, nil
}

// SaveAsDialog asks for a destination with the native dialog, writes content
// there and makes it the active document. If the destination is outside the
// open folder, its directory becomes the open folder.
func (a *App) SaveAsDialog(content string) (WorkspaceState, error) {
	folder, active := a.workspace()
	defaultName := "documento.tex"
	if active != "" {
		defaultName = filepath.Base(active)
	}

	target, err := a.pickSavePath(folder, defaultName)
	if err != nil {
		return WorkspaceState{}, fmt.Errorf("guardar como: %w", err)
	}
	if target == "" {
		return WorkspaceState{Canceled: true}, nil
	}
	target = filepath.Clean(target)
	if filepath.Ext(target) == "" {
		target += ".tex"
	}
	if !filepath.IsAbs(target) || !core.IsTeXFile(target) {
		return WorkspaceState{}, core.ErrNotTeX
	}

	if err := a.files.SaveDocument(content, target); err != nil {
		return WorkspaceState{}, err
	}
	if _, err := core.ResolveInFolder(folder, target); err != nil {
		folder = filepath.Dir(target) // saved elsewhere: open that folder
	}
	a.setWorkspace(folder, target)
	a.compiles.Request()
	return a.workspaceState()
}

// SetMainFile makes path (a .tex inside the open folder) the project's root
// document, or clears it when path is "". The choice is saved in
// <folder>/.creative-ink-meta, so it survives restarts and travels with the
// folder. It queues a compilation of the new target.
func (a *App) SetMainFile(path string) (WorkspaceState, error) {
	folder, _ := a.workspace()
	if folder == "" {
		return WorkspaceState{}, core.ErrNoFolder
	}

	main := ""
	if path != "" {
		clean, err := a.validateTeXPath(folder, path)
		if err != nil {
			return WorkspaceState{}, err
		}
		if info, err := os.Stat(clean); err != nil || !info.Mode().IsRegular() {
			return WorkspaceState{}, fmt.Errorf("el archivo %q no existe", filepath.Base(clean))
		}
		main = clean
	}

	meta, err := core.LoadProjectMeta(folder)
	if err != nil {
		meta = core.ProjectMeta{} // corrupt: overwrite it with a valid one
	}
	meta.MainFile = ""
	if main != "" {
		if meta.MainFile, err = core.RelativeMainFile(folder, main); err != nil {
			return WorkspaceState{}, err
		}
	}
	if err := core.SaveProjectMeta(folder, meta); err != nil {
		return WorkspaceState{}, fmt.Errorf("guardar el archivo principal: %w", err)
	}

	a.mu.Lock()
	if a.folder == folder { // the folder did not change meanwhile
		a.mainFile = main
	}
	a.mu.Unlock()

	a.compiles.Request()
	return a.workspaceState()
}

// ConfirmClose is the UI's answer to EventAppBeforeClose: it has flushed
// CodeMirror, and saved reports whether everything reached the disk. If not,
// a native dialog asks before discarding the edits.
func (a *App) ConfirmClose(saved bool) {
	a.closeMu.Lock()
	if !a.closing || a.closeAllowed {
		a.closeMu.Unlock()
		return // no close in progress, or the timeout already quit
	}
	if a.closeTimer != nil && !a.closeTimer.Stop() {
		a.closeMu.Unlock()
		return // the timeout fired meanwhile and is quitting
	}
	a.closeMu.Unlock()

	if !saved && !a.confirmQuit("Hay cambios que no se pudieron guardar. ¿Cerrar de todos modos y perderlos?") {
		a.closeMu.Lock()
		a.closing = false // stay open; the next close request starts over
		a.closeMu.Unlock()
		return
	}
	a.allowCloseAndQuit()
}

// RequestCompile queues a background compilation of the active document as
// it is on disk, without saving. It returns at once and reports false only
// while the app is shutting down.
func (a *App) RequestCompile() bool {
	return a.compiles.Request()
}

// ── Safe close ─────────────────────────────────────────────────────────────

// beforeClose is Wails' OnBeforeClose. The unsaved text lives in CodeMirror,
// so Go cannot flush it alone. Protocol, without blocking (on Windows this
// runs on the UI thread):
//  1. First call: prevent the close, emit EventAppBeforeClose and arm a
//     timeout (closeTimeout).
//  2. The UI flushes and calls ConfirmClose → closeAllowed → runtime.Quit,
//     which calls beforeClose again: now it returns false and the app quits.
//  3. If the UI never answers, the timeout quits anyway.
func (a *App) beforeClose(_ context.Context) (prevent bool) {
	a.closeMu.Lock()
	switch {
	case a.closeAllowed:
		a.closeMu.Unlock()
		return false
	case a.closing:
		a.closeMu.Unlock()
		return true // already waiting for the UI (e.g. a second click on ✕)
	}
	a.closing = true
	a.closeTimer = time.AfterFunc(a.closeTimeout, func() {
		log.Printf("[close] la interfaz no confirmó el guardado en %s; se cierra igualmente", a.closeTimeout)
		a.allowCloseAndQuit()
	})
	a.closeMu.Unlock()

	a.emit(EventAppBeforeClose)
	return true
}

// allowCloseAndQuit lets the next OnBeforeClose pass and quits (once).
func (a *App) allowCloseAndQuit() {
	a.closeMu.Lock()
	if a.closeAllowed {
		a.closeMu.Unlock()
		return
	}
	a.closeAllowed = true
	a.closeMu.Unlock()
	a.quit()
}

func (a *App) quitWails() {
	if a.ctx != nil {
		runtime.Quit(a.ctx) // re-enters beforeClose, which now allows it
	}
}

func (a *App) dialogConfirmQuit(message string) bool {
	if a.ctx == nil {
		return true
	}
	answer, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Cambios sin guardar",
		Message:       message,
		Buttons:       []string{closeAnywayButton, cancelCloseButton},
		DefaultButton: cancelCloseButton,
		CancelButton:  cancelCloseButton,
	})
	if err != nil {
		return true // never trap the user in an app that cannot close
	}
	// Windows ignores custom buttons and answers "Yes"/"No".
	return answer == closeAnywayButton || answer == "Yes"
}

// ── Helpers ────────────────────────────────────────────────────────────────

// workspace returns the open folder and active document.
func (a *App) workspace() (folder, docPath string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.folder, a.docPath
}

// setWorkspace changes the open folder and active document. Entering a
// different folder reloads its main file from .creative-ink-meta (read
// before locking: it is disk I/O).
func (a *App) setWorkspace(folder, docPath string) {
	current, _ := a.workspace()
	changed := folder != current
	var main string
	if changed {
		main = loadMainFile(folder)
	}

	a.mu.Lock()
	a.folder, a.docPath = folder, docPath
	if changed {
		a.mainFile = main
	}
	a.mu.Unlock()

	if changed {
		a.restartWatcher()
	}
}

// restartWatcher watches the folder that is open now (not a parameter: if two
// folder changes race, the last call always watches the current folder).
func (a *App) restartWatcher() {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	if a.watcher != nil {
		_ = a.watcher.Close()
		a.watcher = nil
	}
	folder, _ := a.workspace()
	if a.watchFolder == nil || folder == "" {
		return
	}
	if tree, truncated, err := core.ScanTree(folder); err == nil {
		a.treeSig = core.TreeSignature(tree, truncated)
	}
	w, err := a.watchFolder(folder, func() { a.onFolderChanged(folder) })
	if err != nil {
		// Not fatal: the explorer still has its manual refresh button.
		log.Printf("[watcher] no se vigilará %q: %v", folder, err)
		return
	}
	a.watcher = w
}

func (a *App) stopWatching() {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	if a.watcher != nil {
		_ = a.watcher.Close()
		a.watcher = nil
	}
}

// onFolderChanged runs on the watcher goroutine after a debounced burst of
// external changes. It emits EventWorkspaceChanged only if the tree the
// explorer shows really changed (e.g. kqueue reports Remove+Create when
// main.tex is replaced by an atomic save: same tree, no event).
func (a *App) onFolderChanged(folder string) {
	if current, _ := a.workspace(); current != folder {
		return // a stale watcher for a folder that is no longer open
	}
	tree, truncated, err := core.ScanTree(folder)
	if err != nil {
		return
	}
	sig := core.TreeSignature(tree, truncated)

	a.watchMu.Lock()
	unchanged := sig == a.treeSig
	a.treeSig = sig
	a.watchMu.Unlock()
	if !unchanged {
		a.emit(EventWorkspaceChanged, WorkspaceChangedEvent{Folder: folder})
	}
}

// loadMainFile returns the valid main file stored in folder's meta, or "".
func loadMainFile(folder string) string {
	meta, err := core.LoadProjectMeta(folder)
	if err != nil {
		log.Printf("[project] %v; se ignora el archivo principal", err)
		return ""
	}
	return meta.MainFilePath(folder)
}

// effectiveMainFile is the main file if it still exists on disk, else "".
func (a *App) effectiveMainFile() string {
	a.mu.RLock()
	main := a.mainFile
	a.mu.RUnlock()
	if main == "" {
		return ""
	}
	if info, err := os.Stat(main); err != nil || !info.Mode().IsRegular() {
		return "" // deleted or renamed outside the app: fall back to the active doc
	}
	return main
}

// compileTarget is what gets compiled and previewed: the main file when the
// project has one, otherwise the active document.
func (a *App) compileTarget() string {
	if main := a.effectiveMainFile(); main != "" {
		return main
	}
	_, doc := a.workspace()
	return doc
}

func (a *App) workspaceState() (WorkspaceState, error) {
	folder, doc := a.workspace()
	if folder == "" {
		return WorkspaceState{}, nil
	}
	tree, truncated, err := core.ScanTree(folder)
	if err != nil {
		return WorkspaceState{}, err
	}
	return WorkspaceState{
		FolderPath:   folder,
		FilePath:     doc,
		MainFilePath: a.effectiveMainFile(),
		Tree:         tree,
		Truncated:    truncated,
	}, nil
}

// validateTeXPath accepts only .tex files inside the open folder.
func (a *App) validateTeXPath(folder, path string) (string, error) {
	clean, err := core.ResolveInFolder(folder, path)
	if err != nil {
		return "", err
	}
	if !core.IsTeXFile(clean) {
		return "", core.ErrNotTeX
	}
	return clean, nil
}

// pdfPath is the PDF of the compile target ("" if none): the main file's
// when the project has one, so the preview does not change per chapter.
func (a *App) pdfPath() string {
	target := a.compileTarget()
	if target == "" {
		return ""
	}
	return compiler.PDFPath(target)
}

// emitWails is the production emitter. It is a no-op before startup.
func (a *App) emitWails(name string, data ...any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data...)
}

func (a *App) dialogPickFolder(defaultDir string) (string, error) {
	if a.ctx == nil {
		return "", errNoContext
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Abrir carpeta",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
}

func (a *App) dialogPickSavePath(defaultDir, defaultName string) (string, error) {
	if a.ctx == nil {
		return "", errNoContext
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Guardar como…",
		DefaultDirectory:     defaultDir,
		DefaultFilename:      defaultName,
		CanCreateDirectories: true,
		Filters:              []runtime.FileFilter{{DisplayName: "Documentos LaTeX (*.tex)", Pattern: "*.tex"}},
	})
}
