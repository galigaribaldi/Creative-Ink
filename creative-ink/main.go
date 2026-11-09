package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Creative-ink",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: app.previewMiddleware, // serves the compiled PDF (see preview.go)
		},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 255},
		// macOS only (nil elsewhere): Edit menu for the clipboard, plus the
		// app's commands. See menu.go.
		Menu:       app.applicationMenu(),
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		// Flush CodeMirror to disk before the window closes (see App.beforeClose).
		OnBeforeClose: app.beforeClose,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
