package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"creative-ink/internal/ci"
	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
)

// EventPreviewShow carries a PreviewShowEvent: show a PDF that is not the
// compile target (the `ci guide` guide) until the next compilation.
const EventPreviewShow = "preview:show"

// PreviewShowEvent is the payload of EventPreviewShow.
type PreviewShowEvent struct {
	File       string `json:"file"`
	Title      string `json:"title"`
	PDFVersion uint64 `json:"pdfVersion"`
	PDFModTime int64  `json:"pdfModTime"`
}

// ListTerminalCommands returns the `ci` command registry without handlers:
// names, usage, descriptions and the forms the Terminal CI completes with
// Tab. It is the same registry `ci help` and `ci guide` are made from.
func (a *App) ListTerminalCommands() []ci.CommandInfo {
	return ci.Info()
}

// userDataDir is <UserConfigDir>/creative-ink ("" if unknown).
func (a *App) userDataDir() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Dir(a.settingsPath)
}

// guideDir is where `ci guide` writes and compiles the guide.
func (a *App) guideDir() string {
	if dir := a.userDataDir(); dir != "" {
		return filepath.Join(dir, "guide")
	}
	return ""
}

// showGuide publishes the compiled guide as a preview snapshot and tells
// the UI to show it (it stays until the next compilation of the document).
func (a *App) showGuide(pdf string) error {
	var version uint64
	var modTime int64
	compiler.WithCompileLock(func() { version, modTime = a.publishPreview(pdf) })
	if version == 0 {
		return errors.New("no se pudo publicar el PDF de la guía")
	}
	a.emit(EventPreviewShow, PreviewShowEvent{File: pdf, Title: "Guía de ci", PDFVersion: version, PDFModTime: modTime})
	return nil
}

// ── Project templates ──────────────────────────────────────────────────────

// templateSources: the repository's templates first, then the user's own.
func (a *App) templateSources() []core.TemplateSource {
	var sources []core.TemplateSource
	if root := a.repository(); root != "" {
		sources = append(sources, core.TemplateSource{
			Dir: filepath.Join(core.MetaDir(root), "templates"), Source: core.TemplateSourceRepository,
		})
	}
	if dir := a.userDataDir(); dir != "" {
		sources = append(sources, core.TemplateSource{Dir: filepath.Join(dir, "templates"), Source: core.TemplateSourceUser})
	}
	return sources
}

// ensureUserTemplatesDir creates <UserConfigDir>/creative-ink/templates so
// the user has a place to drop their academic templates (best effort).
func (a *App) ensureUserTemplatesDir() {
	if dir := a.userDataDir(); dir != "" {
		_ = os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	}
}

// CreateProjectFromTemplate creates <repo>/<name normalized>/ as a copy of
// one of the templates listed on the start screen (template = its path),
// opens it and compiles it like CreateProject. A .latexmkrc that comes from
// the user's own template is trusted: projects created by the editor are
// (docs/MAP.md §4.6, decision 7).
func (a *App) CreateProjectFromTemplate(name, template string) (WorkspaceState, error) {
	root := a.repository()
	if root == "" {
		return WorkspaceState{}, errors.New("el repositorio central no está configurado")
	}
	var chosen *core.TemplateInfo
	for _, t := range core.ListTemplates(a.templateSources()...) {
		if t.Path == template {
			chosen = &t
			break
		}
	}
	if chosen == nil { // only listed folders can be copied
		return WorkspaceState{}, fmt.Errorf("%w: %s", core.ErrUnknownTemplate, filepath.Base(template))
	}
	dir, mainTex, err := core.CreateProjectFromTemplate(root, name, chosen.Path)
	if err != nil {
		return WorkspaceState{}, err
	}
	if a.settingsPath != "" {
		trustDir := dir
		if mainTex != "" {
			trustDir = filepath.Dir(mainTex)
		}
		if _, files, _ := core.RCHash(trustDir); len(files) > 0 {
			if _, err := core.TrustProject(a.settingsPath, trustDir); err != nil {
				return WorkspaceState{}, fmt.Errorf("confiar en la plantilla: %w", err)
			}
		}
	}
	return a.openProjectFolder(dir, mainTex)
}
