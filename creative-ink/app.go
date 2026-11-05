package main

import "context"

// App is the single IPC facade exposed to the frontend.
// Every exported method becomes a Wails binding (window.go.main.App.*).
// Business logic lives in internal/core and internal/system; App only delegates.
type App struct {
	ctx context.Context
}

// NewApp creates the App facade.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the app starts. The context is kept to
// use the Wails runtime (events, dialogs) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SystemHealthCheck verifies that the frontend ↔ Go IPC bridge is alive.
func (a *App) SystemHealthCheck() string {
	return "Creative-ink Core: Puente IPC Activo"
}
