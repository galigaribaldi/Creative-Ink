// Package compiler runs the LaTeX toolchain (latexmk) on documents.
//
// It must not import Wails; App translates results for the frontend.
package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"creative-ink/internal/system"
)

const (
	// compileTimeout bounds a single latexmk run (it can loop on bad input).
	compileTimeout = 60 * time.Second
	// maxLogBytes caps the log sent through IPC; the tail is what matters.
	maxLogBytes = 32 * 1024
	// errorContextLines is how many lines after a "! ..." TeX error are kept.
	errorContextLines = 6
)

// ErrLatexmkNotFound is returned when latexmk is not installed.
var ErrLatexmkNotFound = errors.New("latexmk no está instalado o no se encuentra en el PATH")

// compileMu serialises runs: two latexmk in the same dir corrupt aux files.
var compileMu sync.Mutex

// CompileLaTeX runs
//
//	latexmk -xelatex -interaction=nonstopmode -halt-on-error <filename>
//
// inside workdir. On success it returns a short message. On failure it
// returns the relevant log (TeX errors first, then the output tail) and a
// non-nil error.
func CompileLaTeX(workdir string, filename string) (string, error) {
	if err := validateFilename(filename); err != nil {
		return "", err
	}

	latexmk, ok := system.FindBinary("latexmk")
	if !ok {
		return "", ErrLatexmkNotFound
	}

	compileMu.Lock()
	defer compileMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, latexmk,
		"-xelatex", "-interaction=nonstopmode", "-halt-on-error", filename)
	cmd.Dir = workdir
	// latexmk invokes xelatex/biber by name. A GUI app launched from Finder
	// lacks /Library/TeX/texbin on PATH, so prepend the dir where latexmk lives.
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(latexmk)+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.WaitDelay = 2 * time.Second

	// Stdout and Stderr share one buffer to keep their original interleaving.
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()
	logs := out.String()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return summarize(logs), fmt.Errorf("la compilación superó el límite de %s", compileTimeout)
	}
	if runErr != nil {
		return summarize(logs), fmt.Errorf("latexmk falló: %w", runErr)
	}
	return fmt.Sprintf("Compilación exitosa: %s", pdfName(filename)), nil
}

// PDFPath returns where latexmk writes the PDF for a given .tex path.
func PDFPath(texPath string) string {
	return strings.TrimSuffix(texPath, filepath.Ext(texPath)) + ".pdf"
}

func pdfName(filename string) string {
	return filepath.Base(PDFPath(filename))
}

// validateFilename accepts only a bare file name: no directories and no
// leading "-" (it would be parsed by latexmk as an option).
func validateFilename(filename string) error {
	switch {
	case filename == "" || filename != filepath.Base(filename):
		return fmt.Errorf("nombre de archivo inválido %q: debe ser un nombre sin directorios", filename)
	case strings.HasPrefix(filename, "-"):
		return fmt.Errorf("nombre de archivo inválido %q: no puede empezar con '-'", filename)
	}
	return nil
}

// summarize puts the TeX error blocks first (what the user needs) and then
// the tail of the raw output, capped at maxLogBytes.
func summarize(logs string) string {
	tail := logs
	if len(tail) > maxLogBytes {
		tail = "…\n" + tail[len(tail)-maxLogBytes:]
	}
	errs := extractTeXErrors(logs)
	if errs == "" {
		return tail
	}
	return errs + "\n\n──── Salida de latexmk ────\n" + tail
}

// extractTeXErrors collects TeX error blocks: a line starting with "! "
// plus the following lines up to the "l.<n>" line that points to the source.
func extractTeXErrors(logs string) string {
	lines := strings.Split(logs, "\n")
	var blocks []string
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "! ") {
			continue
		}
		block := []string{lines[i]}
		for j := i + 1; j < len(lines) && j <= i+errorContextLines; j++ {
			block = append(block, lines[j])
			if strings.HasPrefix(lines[j], "l.") {
				i = j
				break
			}
		}
		blocks = append(blocks, strings.Join(block, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}
