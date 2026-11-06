package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
	"creative-ink/internal/system"
)

// defaultDocumentPath is the provisional single document, relative to the
// working directory, until "open folder/file" exists.
const defaultDocumentPath = "workspace/main.tex"

// Wails events emitted to the frontend (documented in docs/MAP.md §3).
const (
	// EventCompileStarted has no payload.
	EventCompileStarted = "compile:started"
	// EventCompileFinished carries a compiler.Result.
	EventCompileFinished = "compile:finished"
)

// shutdownTimeout bounds how long closing the app waits for latexmk to die.
const shutdownTimeout = 3 * time.Second

// App is the single IPC facade exposed to the frontend.
// Every exported method becomes a Wails binding (window.go.main.App.*).
// Business logic lives in internal/*; App only coordinates and delegates.
type App struct {
	ctx      context.Context
	files    *core.FileManager
	docPath  string
	compiles *compiler.Scheduler
	// emit sends a Wails event. Injected so tests never reach the Wails
	// runtime (runtime.EventsEmit calls log.Fatalf without a Wails context).
	emit func(name string, data ...any)
}

// SaveResult is what the frontend receives after every save. Compilation is
// reported separately through EventCompileStarted / EventCompileFinished.
type SaveResult struct {
	Saved bool `json:"saved"`
}

// NewApp creates the App facade.
func NewApp() *App {
	docPath := defaultDocumentPath
	if abs, err := filepath.Abs(docPath); err == nil {
		docPath = abs
	}

	a := &App{
		files:   core.NewFileManager(),
		docPath: docPath,
	}
	a.emit = a.emitWails

	dir, name := filepath.Dir(docPath), filepath.Base(docPath)
	a.compiles = compiler.NewScheduler(
		func(ctx context.Context) (string, error) {
			return compiler.CompileLaTeXContext(ctx, dir, name)
		},
		compiler.Hooks{
			OnStarted:  func() { a.emit(EventCompileStarted) },
			OnFinished: func(r compiler.Result) { a.emit(EventCompileFinished, r) },
		},
	)
	return a
}

// startup is called by Wails when the app starts. The context is kept to
// use the Wails runtime (events, dialogs) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called by Wails when the app is closing: stop latexmk so it
// does not outlive the window.
func (a *App) shutdown(_ context.Context) {
	a.compiles.Close(shutdownTimeout)
}

// emitWails is the production emitter. It is a no-op before startup.
func (a *App) emitWails(name string, data ...any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data...)
}

// SystemHealthCheck audits the host LaTeX toolchain (xelatex, latexmk, biber).
// Wails serialises the struct to a plain JS object for the frontend.
func (a *App) SystemHealthCheck() system.HealthStatus {
	return system.CheckHealth()
}

// SaveContent writes the editor content to the current document and returns
// immediately. If the write succeeded it queues a background compilation,
// whose progress arrives as EventCompileStarted / EventCompileFinished.
//
// A save failure rejects the JS promise and nothing is compiled.
func (a *App) SaveContent(content string) (SaveResult, error) {
	if err := a.files.SaveDocument(content, a.docPath); err != nil {
		return SaveResult{}, err
	}
	a.compiles.Request()
	return SaveResult{Saved: true}, nil
}

// LoadContent returns the current document, or "" if it does not exist yet.
func (a *App) LoadContent() (string, error) {
	return a.files.LoadDocument(a.docPath)
}

// DocumentPath returns the absolute path of the current document.
func (a *App) DocumentPath() string {
	return a.docPath
}

// pdfPath is the PDF produced by latexmk for the current document.
func (a *App) pdfPath() string {
	return compiler.PDFPath(a.docPath)
}
