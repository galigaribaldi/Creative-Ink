package main

import (
	"net/http"
	"os"
)

// previewRoute is the URL the frontend iframe loads to display the PDF.
// Keep it in sync with PREVIEW_URL in frontend/src/components/Preview.svelte.
const previewRoute = "/__preview/document.pdf"

// previewMiddleware serves the compiled PDF of the current document at
// previewRoute and passes every other request to the default Wails handler.
//
// It is a Middleware (not AssetServer.Handler) on purpose: in `wails dev` the
// Handler is only reached when Vite answers 404, but Vite answers iframe
// navigations with index.html (SPA fallback). Middleware runs first in both
// dev and production builds.
//
// Only one fixed file is exposed; the URL never selects a path on disk.
func (a *App) previewMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != previewRoute {
			next.ServeHTTP(w, r)
			return
		}

		f, err := os.Open(a.pdfPath())
		if err != nil {
			http.Error(w, "PDF no disponible todavía", http.StatusNotFound)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, "No se pudo leer el PDF", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		// Every compilation overwrites the file; never let the webview cache it.
		w.Header().Set("Cache-Control", "no-store")
		// ServeContent handles Range requests, which PDF viewers use.
		http.ServeContent(w, r, "document.pdf", info.ModTime(), f)
	})
}
