package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"creative-ink/internal/ci"
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
	// EventConfigChanged carries a ConfigChangedEvent (config service).
	EventConfigChanged = "config:changed"
	// EventProjectHealth carries a core.ProjectHealth: the large-project
	// detector's result for the open folder (it only informs).
	EventProjectHealth = "project:health"
	// EventTerminalStarted carries a TerminalStartedEvent: a terminal run
	// began and its full output is being spooled (ReadTerminalOutput).
	EventTerminalStarted = "terminal:started"
	// EventTerminalProgress carries a TerminalProgressEvent while a
	// `ci compile`/`ci clean` build runs (throttled, ~4 per second).
	EventTerminalProgress = "terminal:progress"
)

// shutdownTimeout bounds how long closing the app waits for latexmk to die.
const shutdownTimeout = 3 * time.Second

// closeFlushTimeout bounds how long closing the window waits for the UI to
// flush CodeMirror to disk before quitting anyway (a hung UI must not keep
// the app open forever).
const closeFlushTimeout = 2 * time.Second

// previewSnapshotsKept: the PDF on screen, the one loading and one spare.
const previewSnapshotsKept = 3

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
	pickNewPath  func(defaultDir, defaultName string) (string, error)
	// openTerminal opens the OS's own terminal in a folder and returns its
	// name (system.OpenTerminal; injected so tests never open a window).
	openTerminal func(dir string) (string, error)
	// repoOptions proposes repository locations for this OS (injectable).
	repoOptions func() system.RepositoryOptions
	// settingsPath is the per-user settings file ("" = cannot persist).
	settingsPath string
	// repoRoot is the central repository (a launcher, never the open folder
	// itself); repoShared marks the "all users" scope. Guarded by mu.
	repoRoot   string
	repoShared bool
	// repoWarnings are shown once on the start screen (e.g. a migration
	// that could not run). Guarded by mu.
	repoWarnings []string

	// mu guards folder and docPath: bound calls each run on their own
	// goroutine, and the compile worker and preview middleware read them too.
	mu      sync.RWMutex
	folder  string // absolute path of the open folder
	docPath string // absolute path of the active document, any editable file ("" if none)
	// texDoc is the last .tex made active in this folder. Without a main
	// file it is the compile target, so opening refs.bib keeps compiling
	// and previewing the chapter that was open before.
	texDoc string
	// mainFile is the project's root document (absolute, "" if none), read
	// from <folder>/.creative-ink-meta whenever the folder changes. When set,
	// it is compiled instead of the active document.
	mainFile string
	// location is where the open folder lives (repository, quick document
	// or external, git, sync), computed when the folder changes, before it
	// is opened (core.ClassifyLocation; docs/MAP.md §4.7).
	location core.ProjectLocation

	// Safe-close protocol (see beforeClose). quit and confirmQuit are
	// injected: the real ones need a Wails context.
	closeMu      sync.Mutex
	closing      bool // waiting for the UI to flush
	closeAllowed bool // the next OnBeforeClose lets the window close
	closeTimer   *time.Timer
	closeTimeout time.Duration
	quit         func()
	confirmQuit  func(message string) bool

	// previews holds immutable copies of the compiled PDFs served to PDF.js
	// (nil = serve the live PDF, e.g. in tests or if the temp dir failed).
	previews *core.PDFSnapshots
	// lastPreview is the PDF behind the newest published version. Only read
	// or written by publishPreview, which always runs under the compile lock
	// (the worker's OnFinished and `ci compile`), so it needs no lock of its own.
	lastPreview previewStamp

	// Imports (import_app.go): previews by id (the source path never comes
	// from the UI), and the copy in progress. freeBytes is injectable.
	importMu     sync.Mutex
	imports      map[string]core.ImportPreview
	importOrder  []string
	importCancel context.CancelFunc
	freeBytes    func(path string) (uint64, error)

	// termMu guards termCancel: at most one terminal command runs at a time.
	termMu     sync.Mutex
	termCancel context.CancelFunc
	// spool keeps the full output of terminal runs on disk (nil = none,
	// e.g. in tests or if the temp dir failed).
	spool *core.OutputSpool

	// Large-project detector: one analysis at a time, debounced
	// (healthDelay) after opening a folder, external changes and saves.
	healthMu    sync.Mutex
	healthTimer *time.Timer
	health      core.ProjectHealth
	healthDelay time.Duration

	// trustMu guards lastTrust: the trust last reported to the UI.
	trustMu   sync.Mutex
	lastTrust core.TrustInfo

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
	// Location of the open folder: kind (repository, quickdoc, external),
	// git root, sync service and the default notices the Terminal CI and the
	// Shell show (core.LocationNotice). Zero without a project.
	Location core.ProjectLocation `json:"location"`
	// NeedsSetup: first run, the user has not chosen where the central
	// repository lives yet (see GetRepositoryOptions / SetupRepository).
	NeedsSetup bool `json:"needsSetup"`
	// NeedsStart: the repository is ready but no project is open yet; the UI
	// shows the start screen (GetStartScreen).
	NeedsStart     bool   `json:"needsStart"`
	RepositoryPath string `json:"repositoryPath"`
	Canceled       bool   `json:"canceled"`   // the user closed the dialog
	FolderPath     string `json:"folderPath"` // absolute
	FilePath       string `json:"filePath"`   // absolute active document (any editable file), "" if none
	// TexFilePath is the last .tex made active: the compile target when the
	// project has no main file (editing a .bib keeps it).
	TexFilePath string `json:"texFilePath"`
	// MainFilePath is the project's root document ("" if none). When set it
	// is what gets compiled and previewed, whatever file is being edited.
	MainFilePath string `json:"mainFilePath"`
	// AutoCompile is the effective compile.auto (config service: default
	// off < user < project). The UI only schedules automatic compilations
	// when it is on, and Go rejects automatic requests when it is off.
	AutoCompile bool          `json:"autoCompile"`
	Tree        core.FileNode `json:"tree"`
	Truncated   bool          `json:"truncated"` // the tree hit its size limits
	// TrustStatus is the trust of the folder latexmk runs in ("" without a
	// project): "untrusted" means its .latexmkrc is ignored (-norc) and the
	// UI offers to trust it. TrustFiles are the rc files found there.
	TrustStatus core.TrustState `json:"trustStatus"`
	TrustFiles  []string        `json:"trustFiles"`
}

// StartScreen is what the start screen shows when a session begins.
type StartScreen struct {
	RepositoryPath string             `json:"repositoryPath"`
	Projects       []core.ProjectInfo `json:"projects"`
	QuickDocs      []core.ProjectInfo `json:"quickDocs"`
	// StaleQuickDocs: quick documents untouched for more than
	// StaleAfterHours. Only reported; deleting them is the user's call.
	StaleQuickDocs  []core.ProjectInfo `json:"staleQuickDocs"`
	StaleAfterHours int                `json:"staleAfterHours"`
	// Recent: folders opened before that still exist, newest first, with
	// their location (an external one is marked, with git or not).
	Recent      []RecentFolder `json:"recent"`
	PluginsPath string         `json:"pluginsPath"`
	Warnings    []string       `json:"warnings"`
	// Templates for "Nuevo proyecto" (CreateProjectFromTemplate), from
	// <repo>/.creative-ink/templates and <UserConfigDir>/creative-ink/templates.
	Templates []core.TemplateInfo `json:"templates"`
}

// RecentFolder is one entry of the start screen's recent list.
type RecentFolder struct {
	Path     string            `json:"path"`
	Location core.LocationKind `json:"location"`
	GitRoot  string            `json:"gitRoot"` // "" if not under git (external folders only)
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

// TerminalResult is the outcome of a command run from the integrated
// terminal. A non-zero exit, a timeout or a cancel are normal outcomes, not
// errors: the promise only rejects when the command could not be started.
type TerminalResult struct {
	Command    string `json:"command"`
	Folder     string `json:"folder"`   // where it ran
	Output     string `json:"output"`   // Stdout+Stderr, capped at 256 KB
	ExitCode   int    `json:"exitCode"` // -1 if it did not exit normally
	TimedOut   bool   `json:"timedOut"`
	Canceled   bool   `json:"canceled"`
	DurationMs int64  `json:"durationMs"`
	// RunID identifies the full output on disk (ReadTerminalOutput); 0 if
	// it was not spooled.
	RunID int `json:"runId"`
}

// TerminalStartedEvent is the payload of EventTerminalStarted.
type TerminalStartedEvent struct {
	RunID   int    `json:"runId"`
	Command string `json:"command"`
}

// TerminalProgressEvent is the payload of EventTerminalProgress.
type TerminalProgressEvent struct {
	RunID int `json:"runId"`
	compiler.BuildProgress
}

// errTerminalBusy rejects a second command while one is running.
var errTerminalBusy = errors.New("ya hay un comando en ejecución; espera o cancélalo")

// SyncTexForwardResult is where a source line appears in the PDF of the
// compile target: 1-based page and a box in PDF points from the page's
// top-left corner. Found is false (no error) when SyncTeX has no answer;
// Reason then says why, for a subtle hint in the UI.
type SyncTexForwardResult struct {
	Found  bool    `json:"found"`
	Page   int     `json:"page"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Reason string  `json:"reason,omitempty"`
}

// SyncTexBackwardResult is the source of a point of the PDF: a .tex inside
// the open folder (absolute) and a 1-based line.
type SyncTexBackwardResult struct {
	Found  bool   `json:"found"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Reason string `json:"reason,omitempty"`
}

const syncTexNoData = "Sin datos de SyncTeX: compila el documento (⌘S) e inténtalo de nuevo."

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
	// PDFVersion identifies the immutable preview snapshot of this result
	// (0 when there is none: failed compilation or snapshot error). The UI
	// loads /__preview/document.pdf?v=<PDFVersion>.
	PDFVersion uint64 `json:"pdfVersion"`
	// PDFModTime is the PDF's modification time (Unix ms, 0 if unknown). An
	// unchanged PDF (latexmk: "All targets up-to-date") keeps the same
	// PDFVersion and PDFModTime, so the UI does not reload the canvas.
	PDFModTime int64 `json:"pdfModTime"`
	// Cached: nothing was compiled; this is the PDF already on disk, shown
	// when a project opens with compile.auto off.
	Cached bool `json:"cached"`
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
	a.pickNewPath = a.dialogPickNewPath
	a.watchFolder = func(root string, onChange func()) (io.Closer, error) {
		return core.WatchFolder(root, core.WatchDebounce, onChange)
	}
	a.quit = a.quitWails
	a.confirmQuit = a.dialogConfirmQuit
	a.openTerminal = system.OpenTerminal
	a.freeBytes = system.FreeBytes
	if previews, err := core.NewPDFSnapshots("", previewSnapshotsKept); err == nil {
		a.previews = previews
	} else {
		log.Printf("[preview] sin instantáneas, se servirá el PDF en vivo: %v", err)
	}
	if spool, err := core.NewOutputSpool(); err == nil {
		a.spool = spool
	} else {
		log.Printf("[terminal] la salida completa no se guardará: %v", err)
	}
	return a
}

// newApp wires the facade; tests call it with a fake compile function.
func newApp(folder, docPath string, compile func(ctx context.Context, workdir, filename string) (string, error)) *App {
	a := &App{
		files:   core.NewFileManager(),
		folder:  folder,
		docPath: docPath,
		texDoc:  texOnly(docPath),
		emit:    func(string, ...any) {},
		quit:    func() {},
		// Overridden by tests; production detects the host OS.
		repoOptions:  system.DetectRepositoryOptions,
		closeTimeout: closeFlushTimeout,
		confirmQuit:  func(string) bool { return true },
		healthDelay:  healthCheckDelay,
		openTerminal: func(string) (string, error) { return "", errors.New("terminal del sistema no disponible") },
		freeBytes:    func(string) (uint64, error) { return 1 << 40, nil },
	}
	if folder != "" {
		a.mainFile = loadMainFile(folder)
	}
	a.compiles = compiler.NewScheduler(
		func(ctx context.Context) (string, error) {
			if a.compilingDoc == "" {
				return "", errNoDocument
			}
			// Untrusted .latexmkrc (Perl code) → latexmk -norc (trust_app.go).
			dir := filepath.Dir(a.compilingDoc)
			ctx = compiler.WithCompileOptions(ctx, a.compileOptionsFor(dir))
			return compile(ctx, dir, filepath.Base(a.compilingDoc))
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
				event := CompileFinishedEvent{Result: r, File: a.compilingDoc}
				// Canceled: a newer request made it obsolete and the next run
				// starts at once; nothing to publish.
				if r.Success && !r.Canceled {
					// Under the compile lock: neither the next latexmk nor a
					// `ci compile` can be rewriting the PDF during the copy.
					compiler.WithCompileLock(func() {
						event.PDFVersion, event.PDFModTime = a.publishPreview(compiler.PDFPath(a.compilingDoc))
					})
				}
				a.emit(EventCompileFinished, event)
			},
		},
	)
	return a
}

// startup is called by Wails when the app starts. The context is kept to
// use the Wails runtime (events, dialogs) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := a.loadRepository(); err != nil {
		// Not fatal: GetWorkspace retries and reports the error to the UI.
		log.Printf("[workspace] no se pudo preparar el repositorio central: %v", err)
	}
	a.ensureUserTemplatesDir()
}

// shutdown is called by Wails when the app is closing: stop latexmk so it
// does not outlive the window.
func (a *App) shutdown(_ context.Context) {
	a.CancelTerminalCommand()
	a.CancelImport() // its goroutine removes the half-made copy
	a.stopWatching()
	a.stopHealthChecks()
	a.compiles.Close(shutdownTimeout)
	if a.previews != nil {
		_ = a.previews.Close()
	}
	if a.spool != nil {
		_ = a.spool.Close()
	}
}

// previewStamp identifies a PDF on disk at publish time.
type previewStamp struct {
	path    string
	modTime time.Time
	size    int64
	version uint64
}

// publishPreview must run under compiler.WithCompileLock (no TeX run can be
// rewriting the PDF meanwhile). It reads the
// PDF's ModTime and, only if the PDF changed, copies a new snapshot. It
// returns the version to show and the ModTime (Unix ms); 0, 0 on error.
func (a *App) publishPreview(pdf string) (version uint64, modTime int64) {
	info, err := os.Stat(pdf)
	if err != nil {
		log.Printf("[preview] no se encontró el PDF %q: %v", pdf, err)
		return 0, 0
	}
	modTime = info.ModTime().UnixMilli()

	// Same file, same ModTime and size: latexmk did not rewrite it. Reuse the
	// published version (no disk copy, and the UI keeps its canvas).
	last := a.lastPreview
	if last.version != 0 && last.path == pdf && last.modTime.Equal(info.ModTime()) && last.size == info.Size() {
		if a.previews == nil {
			return last.version, modTime
		}
		if _, ok := a.previews.Path(last.version); ok {
			return last.version, modTime
		}
	}

	if a.previews == nil {
		// No snapshot store: the middleware serves the live PDF, and the
		// ModTime works as a version that changes whenever the PDF does.
		version = uint64(modTime)
	} else if version, err = a.previews.Publish(pdf); err != nil {
		log.Printf("[preview] no se pudo publicar %q: %v", pdf, err)
		return 0, modTime
	}
	a.lastPreview = previewStamp{path: pdf, modTime: info.ModTime(), size: info.Size(), version: version}
	return version, modTime
}

// loadRepository reads the repository chosen on first run, migrates a
// legacy name ("My Creative Ink Repository" → "My-Creative-Ink-Repository")
// and makes sure its layout exists (.creative-ink/, tmp-Workspaces/). It does
// NOT open a folder: the repository root is a launcher and the start screen
// picks a project. configured is false on first run (nothing is created).
func (a *App) loadRepository() (configured bool, err error) {
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

	root, warning := a.migrateLegacyRepository(settings.RepositoryPath)
	shared := settings.RepositoryScope == system.ScopeShared
	if err := core.EnsureRepositoryLayout(root, shared); err != nil {
		return true, err
	}

	a.mu.Lock()
	a.repoRoot, a.repoShared = root, shared
	if warning != "" {
		a.repoWarnings = append(a.repoWarnings, warning)
	}
	a.mu.Unlock()
	if shared {
		a.ensureUserPluginsDir()
	}
	return true, nil
}

// migrateLegacyRepository renames a repository that still uses the legacy
// name with spaces, and updates settings.json (path and recents). It never
// overwrites: if both folders exist it keeps the old one and warns.
func (a *App) migrateLegacyRepository(path string) (root, warning string) {
	if filepath.Base(path) != system.LegacyRepositoryName {
		return path, ""
	}
	target := filepath.Join(filepath.Dir(path), system.RepositoryName)
	oldExists, newExists := dirExists(path), dirExists(target)
	switch {
	case oldExists && newExists:
		return path, fmt.Sprintf("Existen \"%s\" y \"%s\": se sigue usando la primera. Mueve tus proyectos y borra la que sobre.",
			system.LegacyRepositoryName, system.RepositoryName)
	case oldExists:
		if err := os.Rename(path, target); err != nil {
			return path, fmt.Sprintf("No se pudo renombrar el repositorio a %s: %v", system.RepositoryName, err)
		}
		log.Printf("[repository] renombrado %q → %q", path, target)
	}
	if a.settingsPath != "" {
		_, err := core.UpdateSettings(a.settingsPath, func(s *core.Settings) {
			s.RepositoryPath = target
			for i, f := range s.RecentFolders {
				if f == path || strings.HasPrefix(f, path+string(filepath.Separator)) {
					s.RecentFolders[i] = target + strings.TrimPrefix(f, path)
				}
			}
		})
		if err != nil {
			log.Printf("[settings] no se pudo guardar la nueva ruta del repositorio: %v", err)
		}
	}
	return target, ""
}

// ensureUserPluginsDir creates the per-user plugins folder used when the
// repository is shared (plugins are never stored where every account can
// write: see core.EnsureRepositoryLayout).
func (a *App) ensureUserPluginsDir() {
	if dir := a.pluginsDir(); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[plugins] %v", err)
		}
	}
}

// pluginsDir is where this user's plugins live: <repo>/.creative-ink/plugins
// for a personal repository, <UserConfigDir>/creative-ink/plugins for a
// shared one. "" before the repository is configured.
func (a *App) pluginsDir() string {
	a.mu.RLock()
	root, shared := a.repoRoot, a.repoShared
	a.mu.RUnlock()
	switch {
	case root == "":
		return ""
	case shared:
		if a.settingsPath == "" {
			return ""
		}
		return filepath.Join(filepath.Dir(a.settingsPath), "plugins")
	}
	return filepath.Join(core.MetaDir(root), "plugins")
}

func (a *App) repository() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.repoRoot
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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

	shared := scope == system.ScopeShared
	if err := core.EnsureRepositoryLayout(option.Path, shared); err != nil {
		return WorkspaceState{}, err
	}
	if a.settingsPath != "" {
		_, err := core.UpdateSettings(a.settingsPath, func(s *core.Settings) {
			s.RepositoryScope, s.RepositoryPath = scope, option.Path
		})
		if err != nil {
			return WorkspaceState{}, fmt.Errorf("guardar la configuración: %w", err)
		}
	}
	a.mu.Lock()
	a.repoRoot, a.repoShared = option.Path, shared
	a.mu.Unlock()
	if shared {
		a.ensureUserPluginsDir()
	}
	// Next: the start screen (new project / quick document).
	return a.workspaceState()
}

// GetWorkspace returns the open folder, its tree and the active document.
// With no folder open yet it opens the configured central repository, or
// reports NeedsSetup on first run (nothing is created on disk until then).
func (a *App) GetWorkspace() (WorkspaceState, error) {
	if a.repository() == "" {
		configured, err := a.loadRepository()
		if err != nil {
			return WorkspaceState{}, fmt.Errorf("preparar el repositorio central: %w", err)
		}
		if !configured {
			if folder, _ := a.workspace(); folder == "" {
				return WorkspaceState{NeedsSetup: true}, nil
			}
		}
	}
	return a.workspaceState()
}

// RefreshWorkspace rescans the open folder (e.g. after creating files outside the app).
func (a *App) RefreshWorkspace() (WorkspaceState, error) {
	return a.workspaceState()
}

// GetStartScreen lists the repository's projects and quick documents, the
// recent folders, and the quick documents older than 8 h (a reminder: the
// editor never deletes them).
func (a *App) GetStartScreen() (StartScreen, error) {
	root := a.repository()
	if root == "" {
		return StartScreen{}, errors.New("el repositorio central no está configurado")
	}
	projects, err := core.ListProjects(root)
	if err != nil {
		return StartScreen{}, err
	}
	quick, err := core.ListQuickDocs(root)
	if err != nil {
		return StartScreen{}, err
	}
	stale, err := core.StaleQuickDocs(root, core.QuickDocMaxAge, time.Now())
	if err != nil {
		return StartScreen{}, err
	}

	screen := StartScreen{
		RepositoryPath:  root,
		Projects:        projects,
		QuickDocs:       quick,
		StaleQuickDocs:  stale,
		StaleAfterHours: int(core.QuickDocMaxAge / time.Hour),
		Recent:          []RecentFolder{},
		PluginsPath:     a.pluginsDir(),
		Warnings:        []string{},
		Templates:       core.ListTemplates(a.templateSources()...),
	}
	if a.settingsPath != "" {
		if s, err := core.ReadSettings(a.settingsPath); err == nil {
			for _, f := range s.RecentFolders {
				// Broad folders opened before the rule are no longer offered.
				if dirExists(f) && core.CheckProjectFolder(root, f) == nil {
					loc := core.ClassifyLocation(root, f)
					screen.Recent = append(screen.Recent, RecentFolder{Path: f, Location: loc.Kind, GitRoot: loc.GitRoot})
				}
			}
		}
	}
	a.mu.RLock()
	screen.Warnings = append(screen.Warnings, a.repoWarnings...)
	a.mu.RUnlock()
	if core.DefaultTeXFile(root) != "" {
		screen.Warnings = append(screen.Warnings,
			"Hay documentos .tex sueltos en la raíz del repositorio. La raíz es solo el lanzador: muévelos a un proyecto o a tmp-Workspaces.")
	}
	return screen, nil
}

// CreateProject creates <repo>/<name normalized with "-">/ from the template
// (main.tex as main file, capitulos/, figuras/, referencias.bib) and opens it.
func (a *App) CreateProject(name string) (WorkspaceState, error) {
	root := a.repository()
	if root == "" {
		return WorkspaceState{}, errors.New("el repositorio central no está configurado")
	}
	dir, mainTex, err := core.CreateProject(root, name)
	if err != nil {
		return WorkspaceState{}, err
	}
	return a.openProjectFolder(dir, mainTex)
}

// CreateQuickDocument creates tmp-Workspaces/unnamed-tmp<N>/main.tex (a quick
// single-file document) and opens it.
func (a *App) CreateQuickDocument() (WorkspaceState, error) {
	root := a.repository()
	if root == "" {
		return WorkspaceState{}, errors.New("el repositorio central no está configurado")
	}
	dir, tex, err := core.CreateQuickDoc(root, starterDocument)
	if err != nil {
		return WorkspaceState{}, err
	}
	return a.openProjectFolder(dir, tex)
}

// OpenProject opens a project or quick document folder from the start
// screen: a folder inside the repository (not the root, not .creative-ink)
// or one of the recent folders. Paths always come from GetStartScreen.
func (a *App) OpenProject(path string) (WorkspaceState, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || !dirExists(clean) {
		return WorkspaceState{}, fmt.Errorf("la carpeta %q no existe", path)
	}
	if !a.isOpenableProject(clean) {
		return WorkspaceState{}, fmt.Errorf("%q no es un proyecto del repositorio ni una carpeta reciente", path)
	}
	if err := core.CheckProjectFolder(a.repository(), clean); err != nil {
		return WorkspaceState{}, err // a recent from before the rule (e.g. Desktop)
	}
	doc := core.DefaultTeXFile(clean)
	if meta, err := core.LoadProjectMeta(clean); err == nil {
		if main := meta.MainFilePath(clean); main != "" {
			doc = main
		}
	}
	return a.openProjectFolder(clean, doc)
}

// RevealQuickDocuments opens tmp-Workspaces in the OS file manager, so the
// user can review and delete old quick documents themselves.
func (a *App) RevealQuickDocuments() error {
	root := a.repository()
	if root == "" {
		return errors.New("el repositorio central no está configurado")
	}
	return system.RevealInFileManager(core.QuickDocsDir(root))
}

// OpenSystemTerminal opens the OS's own terminal (Terminal, the Linux
// desktop's emulator, Windows Terminal or cmd.exe) in the open folder and
// returns its name. Go chooses the folder: the UI sends nothing, so no
// text from the WebView ever reaches the launcher. For what the integrated
// terminal cannot do (sudo, interactive prompts, the user's own shell).
func (a *App) OpenSystemTerminal() (string, error) {
	folder, _ := a.workspace()
	if folder == "" {
		return "", core.ErrNoFolder
	}
	return a.openTerminal(folder)
}

func (a *App) isOpenableProject(dir string) bool {
	if root := a.repository(); root != "" {
		rel, err := filepath.Rel(root, dir)
		inside := err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		if inside && !strings.HasPrefix(rel, core.MetaDirName) && rel != core.QuickDocsDirName {
			return true
		}
	}
	if a.settingsPath != "" {
		if s, err := core.ReadSettings(a.settingsPath); err == nil && slices.Contains(s.RecentFolders, dir) {
			return true
		}
	}
	return false
}

// openProjectFolder makes dir the open folder with doc active ("" = none)
// and compiles it.
func (a *App) openProjectFolder(dir, doc string) (WorkspaceState, error) {
	a.setWorkspace(dir, doc)
	if doc != "" {
		a.compileOnOpen()
	}
	return a.workspaceState()
}

// OpenFolderDialog lets the user pick a folder with the native dialog, opens
// it and selects its default document (main.tex or the first .tex).
func (a *App) OpenFolderDialog() (WorkspaceState, error) {
	current, _ := a.workspace()
	if current == "" {
		current = a.repository()
	}
	dir, err := a.pickFolder(current)
	if err != nil {
		return WorkspaceState{}, fmt.Errorf("abrir carpeta: %w", err)
	}
	if dir == "" {
		return WorkspaceState{Canceled: true}, nil
	}
	dir = filepath.Clean(dir)
	// Checked before opening: the launcher, the home folder, Desktop…
	// are not projects (core.ErrBroadFolder).
	if err := core.CheckProjectFolder(a.repository(), dir); err != nil {
		return WorkspaceState{}, err
	}
	a.setWorkspace(dir, core.DefaultTeXFile(dir))
	return a.workspaceState()
}

// OpenFile loads a text file inside the open folder (.tex, .bib, .sty…)
// into the main editor pane, makes it the active document and queues a
// compilation of the target (a .bib keeps the last .tex as target).
func (a *App) OpenFile(path string) (OpenedFile, error) {
	folder, _ := a.workspace()
	clean, content, err := a.readEditable(folder, path)
	if err != nil {
		return OpenedFile{}, err
	}
	a.setWorkspace(folder, clean)
	if a.compileTarget() != "" {
		a.compileOnOpen()
	}
	return OpenedFile{Path: clean, Content: content}, nil
}

// LoadFile reads a text file inside the open folder for the second editor
// pane. Unlike OpenFile it does not change the active document nor compile:
// the main pane keeps deciding what is compiled.
func (a *App) LoadFile(path string) (OpenedFile, error) {
	folder, _ := a.workspace()
	clean, content, err := a.readEditable(folder, path)
	if err != nil {
		return OpenedFile{}, err
	}
	return OpenedFile{Path: clean, Content: content}, nil
}

// readEditable validates path (editable, inside folder) and reads it.
func (a *App) readEditable(folder, path string) (clean, content string, err error) {
	if clean, err = a.validateEditablePath(folder, path); err != nil {
		return "", "", err
	}
	if content, err = a.files.LoadText(clean); err != nil {
		return "", "", err
	}
	return clean, content, nil
}

// NewFileDialog ("Nuevo archivo…", ⌘N) asks for a name with the native
// dialog, inside the open folder, and creates an empty text file there
// (.tex if no extension is given). The name is normalized with "-" like
// every name the editor creates, and an existing file is never replaced.
// The new file becomes the active document; nothing is compiled yet.
func (a *App) NewFileDialog() (WorkspaceState, error) {
	folder, _ := a.workspace()
	if folder == "" {
		return WorkspaceState{}, core.ErrNoFolder
	}
	target, err := a.pickNewPath(folder, "nuevo-archivo.tex")
	if err != nil {
		return WorkspaceState{}, fmt.Errorf("nuevo archivo: %w", err)
	}
	if target == "" {
		return WorkspaceState{Canceled: true}, nil
	}
	clean, err := a.newFilePath(folder, target)
	if err != nil {
		return WorkspaceState{}, err
	}
	if err := core.CreateEmptyFile(clean); err != nil {
		return WorkspaceState{}, err
	}
	a.setWorkspace(folder, clean)
	return a.workspaceState()
}

// newFilePath normalizes the name picked for a new file and checks that it
// is an editable type inside the open folder.
func (a *App) newFilePath(folder, target string) (string, error) {
	target = filepath.Clean(target)
	ext := filepath.Ext(target)
	if ext == "" {
		ext = ".tex"
	}
	stem := core.Slugify(strings.TrimSuffix(filepath.Base(target), filepath.Ext(target)))
	if stem == "" {
		return "", errors.New("el nombre del archivo no tiene caracteres válidos")
	}
	candidate := filepath.Join(filepath.Dir(target), stem+strings.ToLower(ext))
	if !filepath.IsAbs(candidate) {
		return "", fmt.Errorf("ruta inválida %q: debe ser absoluta", candidate)
	}
	if _, err := core.ResolveInFolder(folder, candidate); err != nil {
		return "", fmt.Errorf("el archivo nuevo debe ir dentro de la carpeta abierta: %w", err)
	}
	if !core.IsEditableFile(candidate) {
		return "", core.ErrNotEditable
	}
	return candidate, nil
}

// SaveContent writes content to path (a text file inside the open folder) and
// returns immediately. The path travels with every save so a late autosave
// can never land in another file. It does NOT compile: the UI calls
// RequestCompile after 3 s without typing or on Cmd/Ctrl+S, so typing does
// not keep the compile worker busy. A failure rejects the promise.
func (a *App) SaveContent(path string, content string) (SaveResult, error) {
	folder, _ := a.workspace()
	clean, err := a.validateEditablePath(folder, path)
	if err != nil {
		return SaveResult{}, err
	}
	if err := a.files.SaveDocument(content, clean); err != nil {
		return SaveResult{}, err
	}
	a.scheduleHealthCheck() // debounced: a file may have grown past a limit
	return SaveResult{Saved: true}, nil
}

// SaveAsDialog asks for a destination with the native dialog, writes content
// there and makes it the active document. If the destination is outside the
// open folder, its directory becomes the open folder.
func (a *App) SaveAsDialog(content string) (WorkspaceState, error) {
	folder, active := a.workspace()
	defaultName, defaultExt := "documento.tex", ".tex"
	if active != "" {
		defaultName, defaultExt = filepath.Base(active), filepath.Ext(active)
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
		target += defaultExt // same type as the document being saved
	}
	if !filepath.IsAbs(target) || !core.IsEditableFile(target) {
		return WorkspaceState{}, core.ErrNotEditable
	}
	// Saved elsewhere: that folder is opened, so it must be a project
	// folder. Checked before writing anything.
	outside := false
	if _, err := core.ResolveInFolder(folder, target); err != nil {
		outside = true
		if err := core.CheckProjectFolder(a.repository(), filepath.Dir(target)); err != nil {
			return WorkspaceState{}, fmt.Errorf("guardar como: %w", err)
		}
	}

	if err := a.files.SaveDocument(content, target); err != nil {
		return WorkspaceState{}, err
	}
	if outside {
		folder = filepath.Dir(target) // saved elsewhere: open that folder
	}
	a.setWorkspace(folder, target)
	if a.compileTarget() != "" {
		a.compileOnOpen()
	}
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

	a.compileOnOpen()
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

// RunTerminalCommand runs command through the system shell in the open
// folder (30 s timeout) and returns its combined output. One command at a
// time. TeX and Homebrew directories are added to PATH, which apps launched
// from the macOS Finder lack.
//
// SECURITY: this is arbitrary command execution by design. The UI must only
// send what the user typed in the terminal panel.
func (a *App) RunTerminalCommand(command string) (TerminalResult, error) {
	folder, _ := a.workspace()
	internal := ci.IsCommand(command)
	if folder == "" && !internal {
		return TerminalResult{}, core.ErrNoFolder
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.termMu.Lock()
	if a.termCancel != nil {
		a.termMu.Unlock()
		cancel()
		return TerminalResult{}, errTerminalBusy
	}
	a.termCancel = cancel
	a.termMu.Unlock()
	defer func() {
		a.termMu.Lock()
		a.termCancel = nil
		a.termMu.Unlock()
		cancel()
	}()

	start := time.Now()
	// The full output goes to disk while it runs (the UI reads it on demand
	// with ReadTerminalOutput); only a capped summary returns here.
	runID, spoolWriter := a.startSpool(command)
	var tee io.Writer
	if spoolWriter != nil {
		tee = spoolWriter
		defer spoolWriter.Close()
	}
	// Hybrid terminal: `ci …` is handled natively in Go, never by a shell.
	if internal {
		result := a.runInternalCommand(ctx, command, folder, start, runID, tee)
		result.RunID = runID
		return result, nil
	}
	output, err := core.ExecuteCommandStream(ctx, command, folder, []string{"PATH=" + system.ToolPATH()}, tee)
	result := TerminalResult{
		Command:    command,
		Folder:     folder,
		Output:     output,
		DurationMs: time.Since(start).Milliseconds(),
		RunID:      runID,
	}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.ExitCode = 0
	case errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	case errors.Is(err, core.ErrCommandTimeout):
		result.ExitCode, result.TimedOut = -1, true
	case errors.Is(err, core.ErrCommandCanceled):
		result.ExitCode, result.Canceled = -1, true
	default:
		return TerminalResult{}, err // could not start (empty command, missing folder, no shell)
	}
	return result, nil
}

// runInternalCommand runs a reserved `ci` command (see internal/ci). A
// failure is a normal terminal result: exit code 1 and the reason appended.
func (a *App) runInternalCommand(ctx context.Context, command, folder string, start time.Time, runID int, output io.Writer) TerminalResult {
	text, err := ci.Run(ctx, command, a.ciEnv(runID, output))
	result := TerminalResult{Command: command, Folder: folder, Output: text}
	switch {
	case err == nil:
	case ctx.Err() != nil:
		result.ExitCode, result.Canceled = -1, true
	default:
		result.ExitCode = 1
		if result.Output != "" && !strings.HasSuffix(result.Output, "\n") {
			result.Output += "\n"
		}
		result.Output += "✗ " + err.Error() + "\n"
	}
	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// ciEnv gives the `ci` handler the compile target, the settings and hooks
// that keep the preview in sync with manual builds.
func (a *App) ciEnv(runID int, output io.Writer) ci.Env {
	target := a.compileTarget()
	var rcArgs []string
	if target != "" {
		rcArgs = compiler.LatexmkRCArgs(a.compileOptionsFor(filepath.Dir(target)))
	}
	return ci.Env{
		Target:        target,
		Config:        ciConfig{a},
		Trust:         ciTrust{a},
		LatexmkRCArgs: rcArgs,
		GuideDir:      a.guideDir(),
		ShowGuide:     a.showGuide,
		OpenTerminal:  a.OpenSystemTerminal,
		Location:      a.projectLocation(),
		Output: compiler.BuildOutput{
			Log: output,
			Progress: func(p compiler.BuildProgress) {
				a.emit(EventTerminalProgress, TerminalProgressEvent{RunID: runID, BuildProgress: p})
			},
		},
		BuildStarted: func() {
			a.emit(EventCompileStarted, CompileStartedEvent{File: target})
		},
		BuildFinished: func(d time.Duration, err error, report string) {
			event := CompileFinishedEvent{
				Result: compiler.Result{Success: err == nil, DurationMs: d.Milliseconds()},
				File:   target,
			}
			if err != nil {
				event.Logs = err.Error() + "\n\n" + report
			} else {
				// Already under the compile lock (see ci.Env.BuildFinished).
				event.Logs = "Compilado con ci compile."
				event.PDFVersion, event.PDFModTime = a.publishPreview(compiler.PDFPath(target))
			}
			a.emit(EventCompileFinished, event)
		},
	}
}

// startSpool opens the on-disk output of a terminal run and tells the UI
// its id (EventTerminalStarted). Without a spool it returns 0, nil.
func (a *App) startSpool(command string) (int, *core.SpoolWriter) {
	if a.spool == nil {
		return 0, nil
	}
	id, w, err := a.spool.Start()
	if err != nil {
		log.Printf("[terminal] sin salida completa para %q: %v", command, err)
		return 0, nil
	}
	a.emit(EventTerminalStarted, TerminalStartedEvent{RunID: id, Command: command})
	return id, w
}

// ReadTerminalOutput returns up to 64 KB of a run's full output from offset
// (cut at a UTF-8 boundary). The UI calls it only when the user opens
// "Salida completa", every 500 ms while the run is going, so nothing is
// pushed through the bridge unless someone is looking.
func (a *App) ReadTerminalOutput(runID int, offset int64) (core.OutputChunk, error) {
	if a.spool == nil {
		return core.OutputChunk{}, core.ErrUnknownRun
	}
	return a.spool.Read(runID, offset, core.MaxOutputChunk)
}

// CancelTerminalCommand stops the running terminal command, if any.
func (a *App) CancelTerminalCommand() {
	a.termMu.Lock()
	defer a.termMu.Unlock()
	if a.termCancel != nil {
		a.termCancel()
	}
}

// SyncTexForward (source → PDF): where line of file (a .tex inside the open
// folder, possibly a chapter) appears in the compile target's PDF. It reads
// the .synctex.gz of the last compilation, so recent edits may be slightly
// off until the next compile.
func (a *App) SyncTexForward(line int, file string) (SyncTexForwardResult, error) {
	folder, _ := a.workspace()
	clean, err := a.validateTeXPath(folder, file)
	if err != nil {
		return SyncTexForwardResult{}, err
	}
	target := a.compileTarget()
	if target == "" {
		return SyncTexForwardResult{Reason: syncTexNoData}, nil
	}
	box, found := compiler.SyncTeXForward(context.Background(), compiler.PDFPath(target), clean, line)
	if !found {
		return SyncTexForwardResult{Reason: syncTexNoData}, nil
	}
	return SyncTexForwardResult{
		Found: true, Page: box.Page, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height,
	}, nil
}

// SyncTexBackward (PDF → source): which .tex line produced the point (x, y)
// of page, in PDF points from the page's top-left corner. Only files inside
// the open folder are returned (not e.g. article.cls from the TeX tree).
func (a *App) SyncTexBackward(page int, x float64, y float64) (SyncTexBackwardResult, error) {
	target := a.compileTarget()
	if target == "" {
		return SyncTexBackwardResult{Reason: syncTexNoData}, nil
	}
	src, found := compiler.SyncTeXBackward(context.Background(), compiler.PDFPath(target), page, x, y)
	if !found {
		return SyncTexBackwardResult{Reason: syncTexNoData}, nil
	}
	folder, _ := a.workspace()
	clean, err := a.validateTeXPath(folder, src.File)
	if err != nil {
		return SyncTexBackwardResult{Reason: "Ese punto viene de un archivo fuera del proyecto."}, nil
	}
	if info, err := os.Stat(clean); err != nil || !info.Mode().IsRegular() {
		return SyncTexBackwardResult{Reason: "El archivo de origen ya no existe."}, nil
	}
	return SyncTexBackwardResult{Found: true, File: clean, Line: src.Line}, nil
}

// RequestCompile queues a background compilation of the compile target as
// it is on disk, without saving. trigger is "auto" (the editor's idle
// timer: rejected while compile.auto is off, and cancellable by a newer
// request) or "manual" (⌘S, the Compilar button: always queued, never
// cancelled). It returns at once and reports whether it was queued (false
// also while the app is shutting down or for an unknown trigger).
func (a *App) RequestCompile(trigger string) bool {
	switch trigger {
	case compileTriggerManual:
		return a.compiles.Request(compiler.TriggerManual)
	case compileTriggerAuto:
		if !a.autoCompileEnabled() {
			return false // second lock: a stale UI or a plugin cannot bypass it
		}
		if a.trustInfo().Status == core.TrustUntrusted {
			return false // never compile on its own a folder whose .latexmkrc we doubt
		}
		return a.compiles.Request(compiler.TriggerAuto)
	}
	return false
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
	var location core.ProjectLocation
	if changed {
		main = loadMainFile(folder)
		if folder != "" {
			location = core.ClassifyLocation(a.repository(), folder)
		}
	}

	a.mu.Lock()
	a.folder, a.docPath = folder, docPath
	if changed {
		a.mainFile = main
		a.texDoc = ""
		a.location = location
	}
	if core.IsTeXFile(docPath) {
		a.texDoc = docPath
	}
	a.mu.Unlock()

	if changed {
		a.restartWatcher()
		a.rememberRecent(folder)
		if folder != "" {
			a.scheduleHealthCheck()
		}
	}
}

// rememberRecent adds folder to the start screen's recent list (best effort).
func (a *App) rememberRecent(folder string) {
	if folder == "" || a.settingsPath == "" {
		return
	}
	if _, err := core.UpdateSettings(a.settingsPath, func(s *core.Settings) { s.AddRecentFolder(folder) }); err != nil {
		log.Printf("[settings] no se pudo guardar el reciente: %v", err)
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
		a.scheduleHealthCheck()
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
// project has one, otherwise the last .tex made active (never a .bib).
func (a *App) compileTarget() string {
	if main := a.effectiveMainFile(); main != "" {
		return main
	}
	return a.lastTeX()
}

// texOnly returns path if it is a .tex file, else "".
func texOnly(path string) string {
	if core.IsTeXFile(path) {
		return path
	}
	return ""
}

func (a *App) workspaceState() (WorkspaceState, error) {
	folder, doc := a.workspace()
	if folder == "" {
		root := a.repository()
		return WorkspaceState{NeedsStart: root != "", RepositoryPath: root, AutoCompile: a.autoCompileEnabled()}, nil
	}
	tree, truncated, err := core.ScanTree(folder)
	if err != nil {
		return WorkspaceState{}, err
	}
	trust := a.trustInfo()
	return WorkspaceState{
		Location:       a.projectLocation(),
		FolderPath:     folder,
		RepositoryPath: a.repository(),
		FilePath:       doc,
		TexFilePath:    a.lastTeX(),
		MainFilePath:   a.effectiveMainFile(),
		AutoCompile:    a.autoCompileEnabled(),
		Tree:           tree,
		Truncated:      truncated,
		TrustStatus:    trust.Status,
		TrustFiles:     trust.RCFiles,
	}, nil
}

// projectLocation is the open folder's location (zero without a project).
func (a *App) projectLocation() core.ProjectLocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.location
}

// lastTeX is the last .tex made active in the open folder (see texDoc).
func (a *App) lastTeX() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.texDoc
}

// validateEditablePath accepts only text files inside the open folder.
func (a *App) validateEditablePath(folder, path string) (string, error) {
	clean, err := core.ResolveInFolder(folder, path)
	if err != nil {
		return "", err
	}
	if !core.IsEditableFile(clean) {
		return "", core.ErrNotEditable
	}
	return clean, nil
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
	})
}

func (a *App) dialogPickNewPath(defaultDir, defaultName string) (string, error) {
	if a.ctx == nil {
		return "", errNoContext
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Nuevo archivo (.tex, .bib, .sty, .txt…)",
		DefaultDirectory:     defaultDir,
		DefaultFilename:      defaultName,
		CanCreateDirectories: true,
	})
}
