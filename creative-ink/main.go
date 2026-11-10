package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
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
		// The theme's --surface-base for the appearance the OS uses now
		// (window_theme.go): no flash of the wrong theme at startup.
		BackgroundColour: systemWindowBackground(),
		Mac: &mac.Options{
			// Follow the system (light/dark), like the frontend's 'system' theme.
			Appearance: mac.DefaultAppearance,
			// WKWebView paints white before the page's first frame; transparent,
			// the native window colour above shows instead until the CSS paints.
			WebviewIsTransparent: true,
		},
		Windows: &windows.Options{
			// Title bar and frame follow the system theme, and its changes.
			Theme: windows.SystemDefault,
		},
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
