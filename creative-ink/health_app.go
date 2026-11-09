package main

import (
	"reflect"
	"time"

	"creative-ink/internal/core"
)

// healthCheckDelay debounces the large-project analysis: opening a folder,
// a burst of saves or of external changes runs it once.
const healthCheckDelay = 2 * time.Second

// GetProjectHealth returns the last large-project analysis of the open
// folder (empty warnings = nothing to report). The UI also receives every
// new result through EventProjectHealth.
func (a *App) GetProjectHealth() core.ProjectHealth {
	folder, _ := a.workspace()
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.health.Folder != folder {
		return core.ProjectHealth{Folder: folder, Warnings: []core.ProjectWarning{}}
	}
	return a.health
}

// scheduleHealthCheck (re)arms the debounced analysis of the open folder.
func (a *App) scheduleHealthCheck() {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.healthTimer != nil {
		a.healthTimer.Stop()
	}
	a.healthTimer = time.AfterFunc(a.healthDelay, a.checkProjectHealth)
}

// checkProjectHealth analyses the open folder (in the timer's goroutine:
// it reads files) and emits EventProjectHealth when the result changed.
// It only informs: nothing about compilation or the preview changes.
func (a *App) checkProjectHealth() {
	folder, _ := a.workspace()
	if folder == "" {
		return
	}
	h := core.AnalyzeProject(folder)
	if current, _ := a.workspace(); current != folder {
		return // the folder changed meanwhile: its own check is scheduled
	}
	a.healthMu.Lock()
	changed := a.health.Folder != h.Folder || !reflect.DeepEqual(a.health.Warnings, h.Warnings)
	a.health = h
	a.healthMu.Unlock()
	if changed {
		a.emit(EventProjectHealth, h)
	}
}

func (a *App) stopHealthChecks() {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.healthTimer != nil {
		a.healthTimer.Stop()
		a.healthTimer = nil
	}
}
