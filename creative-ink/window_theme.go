package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"

	"creative-ink/internal/system"
)

// Native window colours: the --surface-base token of each theme in
// frontend/src/style.css (TestWindowBackgroundMatchesTheTokens keeps them in
// sync). The window paints this before the WebView's first frame.
var (
	surfaceBaseDark  = options.RGBA{R: 0x18, G: 0x18, B: 0x18, A: 255} // #181818
	surfaceBaseLight = options.RGBA{R: 0xf9, G: 0xf9, B: 0xf9, A: 255} // #f9f9f9
)

// windowBackground picks the native background for the appearance the OS
// uses at launch, so the app never flashes the wrong theme while the
// frontend loads (lib/theme.svelte.js resolves 'system' the same way).
// Wails turns a nil BackgroundColour into white, so one is always set.
//
// If the user forced the other theme in the app (localStorage), a short
// frame of the system colour can still show: Go cannot read that choice
// before the WebView starts.
func windowBackground(dark bool) *options.RGBA {
	if dark {
		c := surfaceBaseDark
		return &c
	}
	c := surfaceBaseLight
	return &c
}

// systemWindowBackground is windowBackground for the current OS appearance.
func systemWindowBackground() *options.RGBA {
	return windowBackground(system.SystemPrefersDark())
}
