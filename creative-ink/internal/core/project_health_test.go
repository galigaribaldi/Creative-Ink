package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSmallProjectHasNoWarnings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.tex"), "\\documentclass{article}\n\\includegraphics{a}\n")
	writeFile(t, filepath.Join(dir, "main.pdf"), "%PDF") // the compiled document, not an image
	if h := AnalyzeProject(dir); len(h.Warnings) != 0 || h.ImageCalls != 1 || h.ImageBytes != 0 {
		t.Fatalf("health = %+v", h)
	}
}

func TestProjectHealthSchemes(t *testing.T) {
	defer func(f, tf, b, l, i int) {
		maxProjectFiles, maxTeXFiles, maxSourceBytes, maxSourceLines, maxImageCalls = f, tf, int64(b), l, i
	}(maxProjectFiles, maxTeXFiles, int(maxSourceBytes), maxSourceLines, maxImageCalls)
	maxProjectFiles, maxTeXFiles, maxSourceBytes, maxSourceLines, maxImageCalls = 5, 3, 1<<20, 50, 3

	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d"} {
		writeFile(t, filepath.Join(dir, "capitulos", n+".tex"), "texto\n")
	}
	// Commented-out and escaped calls do not count; four real ones do.
	writeFile(t, filepath.Join(dir, "main.tex"), strings.Repeat("linea\n", 60)+
		"\\includegraphics{a} \\includegraphics[width=1cm]{b}\n% \\includegraphics{c}\n50\\% \\includepdf{d} \\includesvg{e}\n")
	writeFile(t, filepath.Join(dir, "figuras", "foto.png"), strings.Repeat("x", 2048))
	writeFile(t, filepath.Join(dir, "main.aux"), "") // build artifact: ignored

	h := AnalyzeProject(dir)
	kinds := map[string]string{}
	for _, w := range h.Warnings {
		kinds[w.Kind] = w.Message
	}
	if _, ok := kinds[HealthTooManyFiles]; !ok || h.TeXFiles != 5 || h.Files != 6 {
		t.Errorf("files: %+v", h)
	}
	if msg, ok := kinds[HealthLargeFiles]; !ok || !strings.Contains(msg, "main.tex") {
		t.Errorf("size: %q", msg)
	}
	if _, ok := kinds[HealthManyImages]; !ok || h.ImageCalls != 4 || h.ImageBytes != 2048 {
		t.Errorf("images: calls=%d bytes=%d %v", h.ImageCalls, h.ImageBytes, kinds)
	}
}

func TestStripTeXComment(t *testing.T) {
	for in, want := range map[string]string{
		`a % b`: `a `, `50\% de % x`: `50\% de `, `\\% c`: `\\`, `sin`: `sin`,
	} {
		if got := stripTeXComment(in); got != want {
			t.Errorf("stripTeXComment(%q) = %q, want %q", in, got, want)
		}
	}
}
