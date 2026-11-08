package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"creative-ink/internal/core"
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

func TestPreviewMiddlewareServesRanges(t *testing.T) {
	dir := t.TempDir()
	app := &App{docPath: filepath.Join(dir, "main.tex")}
	content := []byte("%PDF-1.7 0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dir, "main.pdf"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	h := app.previewMiddleware(http.NotFoundHandler())

	// PDF.js first checks that ranges are supported…
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, previewRoute, nil))
	if rec.Header().Get("Accept-Ranges") != "bytes" || rec.Header().Get("Content-Length") != "25" {
		t.Fatalf("headers = %v; PDF.js needs Accept-Ranges and Content-Length", rec.Header())
	}

	// …then asks only for the chunks it needs.
	req := httptest.NewRequest(http.MethodGet, previewRoute, nil)
	req.Header.Set("Range", "bytes=9-12")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("Range: code = %d, want 206", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 9-12/25" {
		t.Errorf("Content-Range = %q", got)
	}
	if rec.Body.String() != "0123" {
		t.Errorf("body = %q, want only the requested bytes", rec.Body.String())
	}
}

func TestPreviewMiddlewareServesImmutableSnapshots(t *testing.T) {
	a, events := newTestApp(t, func(ctx context.Context, workdir, filename string) (string, error) {
		return "ok", os.WriteFile(filepath.Join(workdir, "main.pdf"), []byte("%PDF v1"), 0o644)
	})
	previews, err := core.NewPDFSnapshots(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = previews.Close() })
	a.previews = previews
	folder, _ := a.workspace()

	a.RequestCompile()
	nextEvent(t, events) // started
	fin := nextEvent(t, events).data[0].(CompileFinishedEvent)
	if fin.PDFVersion == 0 {
		t.Fatal("a successful compilation must publish a snapshot")
	}

	// The next latexmk run rewrites main.pdf in place…
	if err := os.WriteFile(filepath.Join(folder, "main.pdf"), []byte("%PDF v2, half written"), 0o644); err != nil {
		t.Fatal(err)
	}
	// …but the version PDF.js opened keeps serving the same bytes.
	h := a.previewMiddleware(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, previewRoute+"?v="+strconv.FormatUint(fin.PDFVersion, 10), nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "%PDF v1" {
		t.Fatalf("snapshot: code=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, v := range []string{"999", "abc"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, previewRoute+"?v="+v, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("?v=%s: code = %d, want 404", v, rec.Code)
		}
	}
}

func TestFailedCompileDoesNotPublish(t *testing.T) {
	a, events := newTestApp(t, func(ctx context.Context, workdir, filename string) (string, error) {
		return "! Undefined control sequence.", errors.New("latexmk falló")
	})
	previews, err := core.NewPDFSnapshots(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = previews.Close() })
	a.previews = previews

	a.RequestCompile()
	nextEvent(t, events)
	if fin := nextEvent(t, events).data[0].(CompileFinishedEvent); fin.Success || fin.PDFVersion != 0 {
		t.Fatalf("finished = %+v, want failure without a snapshot", fin)
	}
}
