package main

import (
	"net/http"
	"os"
	"strconv"
)

// previewRoute is the URL the frontend iframe loads to display the PDF.
// Keep it in sync with PREVIEW_URL in frontend/src/components/Preview.svelte.
const previewRoute = "/__preview/document.pdf"

// previewMiddleware serves the compiled PDF at previewRoute and passes every
// other request to the default Wails handler.
//
// It is a Middleware (not AssetServer.Handler) on purpose: in `wails dev` the
// Handler is only reached when Vite answers 404, but Vite answers unknown
// paths with index.html (SPA fallback). Middleware runs first in both dev
// and production builds.
//
// `?v=<version>` selects an immutable snapshot (see core.PDFSnapshots): PDF.js
// loads it lazily with Range requests, possibly while latexmk is already
// rewriting the live PDF. Without `v` it serves the newest snapshot.
//
// Only Go-owned files are exposed; the URL never selects a path on disk.
func (a *App) previewMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != previewRoute {
			next.ServeHTTP(w, r)
			return
		}

		path, ok := a.previewFile(r.URL.Query().Get("v"))
		if !ok {
			http.Error(w, "PDF no disponible", http.StatusNotFound)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.Error(w, "PDF no disponible", http.StatusNotFound)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, "No se pudo leer el PDF", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		// Versioned URLs: nothing to revalidate, and nothing worth caching.
		w.Header().Set("Cache-Control", "no-store")
		// ServeContent (like ServeFile, without its /index.html redirects)
		// sets Accept-Ranges and Content-Length and answers Range requests
		// with 206 Partial Content: that is what PDF.js range loading needs.
		http.ServeContent(w, r, "document.pdf", info.ModTime(), f)
	})
}

// previewFile resolves the file to serve: a snapshot by version, the newest
// snapshot, or (without a snapshot store) the live PDF of the compile target.
func (a *App) previewFile(version string) (string, bool) {
	if a.previews == nil {
		path := a.pdfPath()
		return path, path != ""
	}
	if version == "" {
		_, path, ok := a.previews.Latest()
		return path, ok
	}
	v, err := strconv.ParseUint(version, 10, 64)
	if err != nil {
		return "", false
	}
	return a.previews.Path(v)
}
