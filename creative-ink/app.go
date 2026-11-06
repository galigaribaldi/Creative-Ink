package main

import (
	"context"

	"creative-ink/internal/system"
)

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

// SystemHealthCheck audits the host LaTeX toolchain (xelatex, latexmk, biber).
// Wails serialises the struct to a plain JS object for the frontend.
func (a *App) SystemHealthCheck() system.HealthStatus {
	return system.CheckHealth()
}
