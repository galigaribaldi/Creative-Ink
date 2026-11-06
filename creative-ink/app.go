package main

import (
	"context"
	"path/filepath"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
	"creative-ink/internal/system"
)

// defaultDocumentPath is the provisional single document, relative to the
// working directory, until "open folder/file" exists.
const defaultDocumentPath = "workspace/main.tex"

// App is the single IPC facade exposed to the frontend.
// Every exported method becomes a Wails binding (window.go.main.App.*).
// Business logic lives in internal/*; App only coordinates and delegates.
type App struct {
	ctx     context.Context
	files   *core.FileManager
	docPath string
}

// SaveResult is what the frontend receives after every save.
type SaveResult struct {
	Saved          bool   `json:"saved"`
	CompileSuccess bool   `json:"compileSuccess"`
	CompileLogs    string `json:"compileLogs"`
}

// NewApp creates the App facade.
func NewApp() *App {
	docPath := defaultDocumentPath
	if abs, err := filepath.Abs(docPath); err == nil {
		docPath = abs
	}
	return &App{
		files:   core.NewFileManager(),
		docPath: docPath,
	}
}

// startup is called by Wails when the app starts. The context is kept to
// use the Wails runtime (events, dialogs) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SystemHealthCheck audits the host LaTeX toolchain (xelatex, latexmk, biber).
// Wails serialises the struct to a plain JS object for the frontend.
func (a *App) SystemHealthCheck() system.HealthStatus {
	return system.CheckHealth()
}

// SaveContent writes the editor content to the current document and, only if
// the write succeeded, compiles it with latexmk.
//
// A save failure rejects the JS promise (nothing to compile). A compile
// failure is a normal outcome: Saved=true, CompileSuccess=false, logs set.
func (a *App) SaveContent(content string) (SaveResult, error) {
	if err := a.files.SaveDocument(content, a.docPath); err != nil {
		return SaveResult{}, err
	}

	logs, err := compiler.CompileLaTeX(filepath.Dir(a.docPath), filepath.Base(a.docPath))
	if err != nil {
		if logs == "" {
			logs = err.Error()
		} else {
			logs = err.Error() + "\n\n" + logs
		}
		return SaveResult{Saved: true, CompileSuccess: false, CompileLogs: logs}, nil
	}
	return SaveResult{Saved: true, CompileSuccess: true, CompileLogs: logs}, nil
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
