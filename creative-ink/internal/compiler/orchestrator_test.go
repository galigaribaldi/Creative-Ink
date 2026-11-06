package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFilename(t *testing.T) {
	cases := map[string]bool{
		"main.tex":         true,
		"thesis-v2.tex":    true,
		"":                 false,
		"../main.tex":      false,
		"sub/main.tex":     false,
		"-shell-escape":    false,
		"-outdir=/tmp/x.t": false,
	}
	for name, ok := range cases {
		if err := validateFilename(name); (err == nil) != ok {
			t.Errorf("validateFilename(%q) err = %v, want ok=%v", name, err, ok)
		}
	}
}

func TestPDFPath(t *testing.T) {
	if got := PDFPath("/w/main.tex"); got != "/w/main.pdf" {
		t.Fatalf("PDFPath = %q", got)
	}
}

const sampleFailure = `This is XeTeX, Version 3.141592653
(./main.tex
LaTeX2e <2025-11-01>
! Undefined control sequence.
l.4 \foo

No pages of output.
Latexmk: Errors, so I did not complete making targets
`

func TestExtractTeXErrors(t *testing.T) {
	got := extractTeXErrors(sampleFailure)
	want := "! Undefined control sequence.\nl.4 \\foo"
	if got != want {
		t.Fatalf("extractTeXErrors = %q, want %q", got, want)
	}
}

func TestSummarizePutsErrorsFirstAndCapsSize(t *testing.T) {
	huge := strings.Repeat("x", maxLogBytes*2) + "\n" + sampleFailure
	got := summarize(huge)

	if !strings.HasPrefix(got, "! Undefined control sequence.") {
		t.Errorf("summary must start with the TeX error, got prefix %q", got[:40])
	}
	if len(got) > maxLogBytes+512 {
		t.Errorf("summary length = %d, want <= ~%d", len(got), maxLogBytes)
	}
}

func TestCompileLaTeXRejectsBadFilename(t *testing.T) {
	if _, err := CompileLaTeX(t.TempDir(), "../evil.tex"); err == nil {
		t.Fatal("expected error for path traversal filename")
	}
}

// TestCompileLaTeXIntegration runs the real toolchain. It is opt-in because
// it needs a TeX installation and takes seconds:
//
//	CREATIVE_INK_LATEX_TEST=1 go test ./internal/compiler -run Integration -v
func TestCompileLaTeXIntegration(t *testing.T) {
	if os.Getenv("CREATIVE_INK_LATEX_TEST") == "" {
		t.Skip("set CREATIVE_INK_LATEX_TEST=1 to run against the real latexmk")
	}
	dir := t.TempDir()

	write := func(content string) {
		if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("\\documentclass{article}\n\\begin{document}\nHola\n\\end{document}\n")
	if msg, err := CompileLaTeX(dir, "main.tex"); err != nil {
		t.Fatalf("valid document failed: %v\n%s", err, msg)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.pdf")); err != nil {
		t.Fatalf("PDF not produced: %v", err)
	}

	write("\\documentclass{article}\n\\begin{document}\n\\foo\n\\end{document}\n")
	logs, err := CompileLaTeX(dir, "main.tex")
	if err == nil {
		t.Fatal("broken document must fail")
	}
	if !strings.Contains(logs, "Undefined control sequence") {
		t.Fatalf("logs must contain the TeX error, got:\n%s", logs)
	}
}
