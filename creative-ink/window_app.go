package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"creative-ink/internal/core"
)

// Several windows (docs/MAP.md §4.7). Wails v2 has one window per process,
// so ⇧⌘N "Nueva ventana" starts another instance of the app. Each window
// locks the folder it has open (<UserConfigDir>/creative-ink/locks/): the
// same project in two windows would overwrite each other's autosaves and
// run latexmk on the same .aux files, so the second window refuses it
// (the user's decision 4, 2026-11-11). The OS releases the lock if a
// window crashes.

// errNewWindowInDev: a second instance of the `wails dev` binary has no
// development server; the feature is tried with `wails build`.
var errNewWindowInDev = errors.New("«Nueva ventana» abre otra instancia de Creative-ink: pruébala con la app de wails build (en wails dev la segunda instancia no tiene el servidor de desarrollo)")

// NewWindow (⇧⌘N) opens another, independent Creative-ink window: it starts
// on the start screen and can open any other project.
func (a *App) NewWindow() error {
	if devBuild {
		return errNewWindowInDev
	}
	return a.launchInstance()
}

// launchNewInstance starts the app again. On macOS, a bundle is launched
// with `open -n` (a real new instance for LaunchServices: its own Dock
// icon and menu); elsewhere, the executable itself. No shell.
func launchNewInstance() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	var cmd *exec.Cmd
	if bundle := macAppBundle(exe); runtime.GOOS == "darwin" && bundle != "" {
		cmd = exec.Command("open", "-n", bundle)
	} else {
		cmd = exec.Command(exe)
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("abrir otra ventana: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// macAppBundle returns /…/Creative-ink.app for an executable inside it.
func macAppBundle(exe string) string {
	slash := filepath.ToSlash(exe)
	if i := strings.Index(slash, ".app/Contents/MacOS/"); i >= 0 {
		return filepath.FromSlash(slash[:i+len(".app")])
	}
	return ""
}

// locationLockPath is the lock file of an open folder (or loose file).
func (a *App) locationLockPath(folder, single string) string {
	key := single
	if key == "" {
		key = folder
	}
	if real, err := filepath.EvalSymlinks(key); err == nil {
		key = real // the same folder through a symlink is the same project
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(a.lockDir, hex.EncodeToString(sum[:16])+".lock")
}

// claimLocation locks folder (or the loose file) for this window. With
// another window holding it, it fails with a message that says so.
func (a *App) claimLocation(folder, single string) (func(), error) {
	if a.lockDir == "" {
		return func() {}, nil
	}
	release, err := core.TryLockFile(a.locationLockPath(folder, single))
	if errors.Is(err, core.ErrLocked) {
		name := filepath.Base(folder)
		if single != "" {
			name = filepath.Base(single)
		}
		return nil, fmt.Errorf("%s ya está abierto en otra ventana de Creative-ink: cámbiate a esa ventana (dos ventanas en el mismo proyecto se pisarían los cambios)", name)
	}
	if err != nil {
		return func() {}, nil // the lock directory is unusable: do not block the user
	}
	return release, nil
}

// checkLocationFree reports whether another window has folder open,
// without taking the lock (Save As checks it before writing).
func (a *App) checkLocationFree(folder string) error {
	if current, _ := a.workspace(); current == folder {
		return nil
	}
	release, err := a.claimLocation(folder, "")
	if err != nil {
		return err
	}
	release()
	return nil
}

// releaseLocation frees the open folder's lock (on shutdown).
func (a *App) releaseLocation() {
	a.mu.Lock()
	release := a.releaseLock
	a.releaseLock = nil
	a.mu.Unlock()
	if release != nil {
		release()
	}
}
