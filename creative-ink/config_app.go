package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"creative-ink/internal/ci"
	"creative-ink/internal/compiler"
	"creative-ink/internal/config"
	"creative-ink/internal/core"
)

// RequestCompile triggers (docs/MAP.md §3).
const (
	compileTriggerAuto   = "auto"
	compileTriggerManual = "manual"
)

// cachedPreviewNote is the log of a preview shown without compiling.
const cachedPreviewNote = "Vista previa del último PDF compilado. La compilación automática está desactivada: " +
	"compila con ⌘S / Ctrl+S o actívala con «ci config autocompile on»."

// ConfigChangedEvent is the payload of EventConfigChanged: the key's new
// effective entry, and the layer that was written.
type ConfigChangedEvent struct {
	config.Entry
	Written config.Scope `json:"written"`
	Folder  string       `json:"folder"` // project the project layer belongs to ("" if none)
}

// GetConfig resolves every configuration key (value and the layer it comes
// from: default < user < project).
func (a *App) GetConfig() []config.Entry {
	return a.configLayers().All()
}

// SetConfig writes value to scope ("user" or "project"; "" = user) and
// emits EventConfigChanged. An empty value removes that layer's value, so
// the next layer down applies. It returns the key's new effective entry.
func (a *App) SetConfig(key, value, scope string) (config.Entry, error) {
	sc := config.Scope(scope)
	if sc == "" {
		sc = config.ScopeUser
	}
	k, raw, err := config.Prepare(key, value, sc)
	if err != nil {
		return config.Entry{}, err
	}
	folder, _ := a.workspace()
	switch sc {
	case config.ScopeUser:
		if a.settingsPath == "" {
			return config.Entry{}, errors.New("no se puede guardar la configuración del usuario en este sistema")
		}
		if _, err := core.UpdateSettings(a.settingsPath, func(s *core.Settings) { s.SetUserConfig(k.ID, raw) }); err != nil {
			return config.Entry{}, fmt.Errorf("guardar la configuración: %w", err)
		}
	case config.ScopeProject:
		if folder == "" {
			return config.Entry{}, fmt.Errorf("%w: abre un proyecto para usar --project", core.ErrNoFolder)
		}
		meta, err := core.LoadProjectMeta(folder)
		if err != nil {
			meta = core.ProjectMeta{} // corrupt: rewrite it valid (like SetMainFile)
		}
		if raw == nil {
			delete(meta.Config, k.ID)
		} else {
			if meta.Config == nil {
				meta.Config = map[string]json.RawMessage{}
			}
			meta.Config[k.ID] = raw
		}
		if err := core.SaveProjectMeta(folder, meta); err != nil {
			return config.Entry{}, fmt.Errorf("guardar la configuración del proyecto: %w", err)
		}
	}
	entry := a.configLayers().Resolve(k)
	a.emit(EventConfigChanged, ConfigChangedEvent{Entry: entry, Written: sc, Folder: folder})
	return entry, nil
}

// configLayers loads the user layer (settings.json) and the open project's
// (.creative-ink-meta). Unreadable layers are skipped: defaults apply.
func (a *App) configLayers() config.Layers {
	var l config.Layers
	if a.settingsPath != "" {
		if s, err := core.ReadSettings(a.settingsPath); err == nil {
			l.User = s.UserConfig()
		}
	}
	if folder, _ := a.workspace(); folder != "" {
		if meta, err := core.LoadProjectMeta(folder); err == nil {
			l.Project = meta.Config
		}
	}
	return l
}

// autoCompileEnabled is the effective compile.auto (default off).
func (a *App) autoCompileEnabled() bool {
	return a.configLayers().Bool(config.CompileAuto)
}

// compileOnOpen runs after a file or project is opened (or the main file
// changes). With compile.auto on it queues an automatic compilation; with
// it off nothing is compiled and the PDF already on disk, if any, is shown.
func (a *App) compileOnOpen() {
	if a.autoCompileEnabled() {
		a.compiles.Request(compiler.TriggerAuto)
		return
	}
	target := a.compileTarget()
	if target == "" {
		return
	}
	go a.showCachedPreview(target)
}

// showCachedPreview publishes target's existing PDF (no TeX run) and emits a
// successful compile:finished marked Cached, so the preview is not empty
// after opening a project with compile.auto off.
func (a *App) showCachedPreview(target string) {
	pdf := compiler.PDFPath(target)
	if info, err := os.Stat(pdf); err != nil || !info.Mode().IsRegular() {
		return // never compiled: the preview keeps its hint
	}
	event := CompileFinishedEvent{
		Result: compiler.Result{Success: true, Logs: cachedPreviewNote},
		File:   target,
		Cached: true,
	}
	compiler.WithCompileLock(func() {
		event.PDFVersion, event.PDFModTime = a.publishPreview(pdf)
	})
	if event.PDFVersion == 0 || a.compileTarget() != target {
		return
	}
	a.emit(EventCompileFinished, event)
}

// ciConfig adapts the config bindings to ci.ConfigService.
type ciConfig struct{ a *App }

var _ ci.ConfigService = ciConfig{}

func (c ciConfig) List() []config.Entry { return c.a.GetConfig() }

func (c ciConfig) Set(key, value string, scope config.Scope) (config.Entry, error) {
	return c.a.SetConfig(key, value, string(scope))
}
