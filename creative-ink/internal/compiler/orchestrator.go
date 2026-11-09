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
	"regexp"
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
	// errorContextLines is how many lines after a "! ..." TeX error are kept
	// when no "l.<n>" source reference is found.
	errorContextLines = 6
)

// ErrLatexmkNotFound is returned when latexmk is not installed.
var ErrLatexmkNotFound = errors.New("latexmk no está instalado o no se encuentra en el PATH")

// compileMu serialises runs: two latexmk in the same dir corrupt aux files.
var compileMu sync.Mutex

// CompileLaTeX compiles without external cancellation (see CompileLaTeXContext).
func CompileLaTeX(workdir string, filename string) (string, error) {
	return CompileLaTeXContext(context.Background(), workdir, filename)
}

// CompileLaTeXContext runs
//
//	latexmk -xelatex -interaction=nonstopmode -halt-on-error -synctex=1 <filename>
//
// inside workdir. -synctex=1 writes <doc>.synctex.gz, needed for SyncTeX
// (PDF ↔ source navigation, see synctex.go). Cancelling ctx (e.g. on app shutdown) kills latexmk.
// On success it returns a short message. On failure it returns the relevant
// log (TeX errors first, then the output tail) and a non-nil error.
func CompileLaTeXContext(parent context.Context, workdir string, filename string) (string, error) {
	if err := validateFilename(filename); err != nil {
		return "", err
	}

	latexmk, ok := system.FindBinary("latexmk")
	if !ok {
		return "", ErrLatexmkNotFound
	}

	compileMu.Lock()
	defer compileMu.Unlock()

	ctx, cancel := context.WithTimeout(parent, compileTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, latexmk,
		"-xelatex", "-interaction=nonstopmode", "-halt-on-error", "-synctex=1", filename)
	cmd.Dir = workdir
	// latexmk invokes xelatex/biber by name. A GUI app launched from Finder
	// lacks /Library/TeX/texbin on PATH, so prepend the dir where latexmk lives.
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(latexmk)+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Cancelling (the Scheduler's bounded cancellation, app shutdown) or the
	// timeout kills latexmk AND its xelatex/biber children: a surviving
	// xelatex would keep writing the .aux that the next run reads.
	system.KillTreeOnCancel(cmd)
	// Kill-and-wait bound for the pipes.
	cmd.WaitDelay = 2 * time.Second

	// Stdout and Stderr share one buffer to keep their original interleaving.
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()
	output := out.String()

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return summarize(output, readTeXLog(workdir, filename)),
			fmt.Errorf("la compilación superó el límite de %s", compileTimeout)
	case errors.Is(parent.Err(), context.Canceled):
		return "", fmt.Errorf("compilación cancelada: %w", context.Canceled)
	case runErr != nil:
		return summarize(output, readTeXLog(workdir, filename)),
			fmt.Errorf("latexmk falló: %w", runErr)
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

// logPath returns the TeX .log written next to the document.
func logPath(workdir, filename string) string {
	return filepath.Join(workdir, strings.TrimSuffix(filename, filepath.Ext(filename))+".log")
}

// readTeXLog returns the engine's .log, or "" if it cannot be read.
func readTeXLog(workdir, filename string) string {
	data, err := os.ReadFile(logPath(workdir, filename))
	if err != nil {
		return ""
	}
	return string(data)
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
// the tail of latexmk's output, capped at maxLogBytes.
//
// Errors come from the engine's .log when it has any: it is complete, while
// stdout may lack them (e.g. latexmk skipping a rerun after a previous
// failure). Otherwise they are extracted from latexmk's own output.
func summarize(output, texLog string) string {
	tail := output
	if len(tail) > maxLogBytes {
		tail = "…\n" + tail[len(tail)-maxLogBytes:]
	}

	errs := extractTeXErrors(texLog)
	if errs == "" {
		errs = extractTeXErrors(output)
	}
	if errs == "" {
		return tail
	}
	return errs + "\n\n──── Salida de latexmk ────\n" + tail
}

// sourceRefLine matches TeX's source reference, e.g. "l.42 \foo".
var sourceRefLine = regexp.MustCompile(`^l\.\d+`)

// extractTeXErrors joins the TeX error blocks of logs (see teXErrorBlocks).
func extractTeXErrors(logs string) string {
	return strings.Join(teXErrorBlocks(logs), "\n\n")
}

// teXErrorBlocks collects TeX error blocks. A block starts at a line
// beginning with "! " and ends at the "l.<n>" source reference (plus its
// continuation line, which holds the rest of the offending source line).
// Without a reference, it keeps up to errorContextLines lines and never
// swallows the next "! " error.
func teXErrorBlocks(logs string) []string {
	lines := strings.Split(strings.ReplaceAll(logs, "\r\n", "\n"), "\n")
	var blocks []string
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "! ") {
			continue
		}
		block := []string{lines[i]}
		for j := i + 1; j < len(lines) && j <= i+errorContextLines; j++ {
			line := lines[j]
			if strings.HasPrefix(line, "! ") {
				i = j - 1 // let the outer loop start the next block here
				break
			}
			block = append(block, line)
			if sourceRefLine.MatchString(line) {
				// TeX splits the source line at the error point: the rest is on
				// the next, indented line.
				if next := j + 1; next < len(lines) && strings.HasPrefix(lines[next], " ") &&
					strings.TrimSpace(lines[next]) != "" {
					block = append(block, lines[next])
					j = next
				}
				i = j
				break
			}
		}
		blocks = append(blocks, strings.TrimRight(strings.Join(trimBlankEdges(block), "\n"), " "))
	}
	return blocks
}

// trimBlankEdges drops trailing blank lines from a block.
func trimBlankEdges(block []string) []string {
	end := len(block)
	for end > 1 && strings.TrimSpace(block[end-1]) == "" {
		end--
	}
	return block[:end]
}
