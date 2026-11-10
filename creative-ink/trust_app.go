package main

import (
	"errors"
	"log"
	"path/filepath"

	"creative-ink/internal/ci"
	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
)

// EventTrustChanged carries a TrustChangedEvent: the trust of the open
// project changed (TrustProject/UntrustProject, `ci trust`, or the
// .latexmkrc changed on disk and a compilation noticed it).
const EventTrustChanged = "trust:changed"

// TrustChangedEvent is the payload of EventTrustChanged.
type TrustChangedEvent struct {
	core.TrustInfo
	Project string `json:"project"` // the open folder it applies to
}

var errNoTrustSettings = errors.New("no se puede guardar la confianza en este sistema")

// trustDir is the folder whose rc files latexmk reads: the compile
// target's folder, or the open folder when nothing is compiled yet.
func (a *App) trustDir() string {
	if target := a.compileTarget(); target != "" {
		return filepath.Dir(target)
	}
	folder, _ := a.workspace()
	return folder
}

// trustInfoFor is core.GetTrustStatus for dir that never fails open: if the
// settings cannot be read, a folder with rc files counts as untrusted.
func (a *App) trustInfoFor(dir string) core.TrustInfo {
	if dir == "" {
		return core.TrustInfo{Status: core.TrustTrusted, RCFiles: []string{}}
	}
	info, err := core.GetTrustStatus(a.settingsPath, dir)
	if err == nil {
		return info
	}
	log.Printf("[trust] %v; se trata la carpeta como sin confianza", err)
	hash, files, _ := core.RCHash(dir)
	info = core.TrustInfo{Folder: dir, Status: core.TrustTrusted, RCFiles: files, CurrentHash: hash}
	if len(files) > 0 {
		info.Status = core.TrustUntrusted
	}
	if info.RCFiles == nil {
		info.RCFiles = []string{}
	}
	return info
}

// trustInfo is the trust of the folder latexmk runs in now.
func (a *App) trustInfo() core.TrustInfo { return a.trustInfoFor(a.trustDir()) }

// compileOptionsFor tells latexmk whether to skip dir's rc files (-norc).
func (a *App) compileOptionsFor(dir string) compiler.CompileOptions {
	info := a.trustInfoFor(dir)
	a.noteTrust(info, false)
	return compiler.CompileOptions{NoRC: info.NeedsNoRC()}
}

// noteTrust remembers the trust seen for a folder. It emits
// EventTrustChanged when the user decided (decided = true) or when a
// compilation finds that the same folder's trust changed (e.g. a git pull
// rewrote .latexmkrc). The first look at a folder is not a change: the UI
// already got it in WorkspaceState.
func (a *App) noteTrust(info core.TrustInfo, decided bool) {
	a.trustMu.Lock()
	changed := a.lastTrust.Folder == info.Folder &&
		(a.lastTrust.Status != info.Status || a.lastTrust.CurrentHash != info.CurrentHash)
	a.lastTrust = info
	a.trustMu.Unlock()
	if changed || decided {
		folder, _ := a.workspace()
		a.emit(EventTrustChanged, TrustChangedEvent{TrustInfo: info, Project: folder})
	}
}

// TrustProject trusts the .latexmkrc/latexmkrc of the folder latexmk runs
// in (its current content, by hash) and recompiles with it. Called by the
// banner's "Confiar y recompilar"; `ci trust add` does the same.
func (a *App) TrustProject() (WorkspaceState, error) {
	if _, err := a.setTrust(true); err != nil {
		return WorkspaceState{}, err
	}
	if a.compileTarget() != "" {
		a.compiles.Request(compiler.TriggerManual)
	}
	return a.workspaceState()
}

// UntrustProject restricts the folder latexmk runs in: it always compiles
// with -norc and the UI stops asking. `ci trust remove` does the same.
func (a *App) UntrustProject() (WorkspaceState, error) {
	if _, err := a.setTrust(false); err != nil {
		return WorkspaceState{}, err
	}
	return a.workspaceState()
}

func (a *App) setTrust(trust bool) (core.TrustInfo, error) {
	if a.settingsPath == "" {
		return core.TrustInfo{}, errNoTrustSettings
	}
	dir := a.trustDir()
	if dir == "" {
		return core.TrustInfo{}, core.ErrNoTrustFolder
	}
	update := core.UntrustProject
	if trust {
		update = core.TrustProject
	}
	info, err := update(a.settingsPath, dir)
	if err != nil {
		return core.TrustInfo{}, err
	}
	a.noteTrust(info, true)
	return info, nil
}

// ciTrust adapts the trust to ci.TrustService (`ci trust …`).
type ciTrust struct{ a *App }

var _ ci.TrustService = ciTrust{}

func (c ciTrust) Status() (core.TrustInfo, error) {
	if c.a.trustDir() == "" {
		return core.TrustInfo{}, core.ErrNoTrustFolder
	}
	return c.a.trustInfo(), nil
}

func (c ciTrust) Trust() (core.TrustInfo, error)   { return c.a.setTrust(true) }
func (c ciTrust) Untrust() (core.TrustInfo, error) { return c.a.setTrust(false) }
