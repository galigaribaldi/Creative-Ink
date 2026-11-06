package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewMiddleware(t *testing.T) {
	dir := t.TempDir()
	app := &App{docPath: filepath.Join(dir, "main.tex")}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := app.previewMiddleware(next)

	serve := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	// Other routes go to the default Wails handler.
	if rec := serve("/index.html"); rec.Code != http.StatusTeapot || !nextCalled {
		t.Fatalf("non-preview route: code=%d nextCalled=%v", rec.Code, nextCalled)
	}

	// No PDF yet → 404, not the SPA index.
	if rec := serve(previewRoute + "?t=1"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing PDF: code=%d, want 404", rec.Code)
	}

	pdf := []byte("%PDF-1.7 fake")
	if err := os.WriteFile(filepath.Join(dir, "main.pdf"), pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := serve(previewRoute + "?t=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("PDF route: code=%d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if rec.Body.String() != string(pdf) {
		t.Errorf("body = %q", rec.Body.String())
	}
}
