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

// Real xelatex .log fragment for a missing package in nonstopmode: the first
// error has no "l.<n>", the second has one plus a continuation line.
const sampleMissingPackage = `(./main.tex
LaTeX2e <2025-11-01>

! LaTeX Error: File ` + "`foo.sty'" + ` not found.

Type X to quit or <RETURN> to proceed,
or enter new name. (Default extension: sty)

Enter file name: 
! Emergency stop.
<read *> 
         
l.3 \usepackage
               {foo}^^M
No pages of output.
`

func TestExtractTeXErrorsMissingPackage(t *testing.T) {
	blocks := teXErrorBlocks(sampleMissingPackage)
	got := strings.Join(blocks, "\n")

	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d:\n%s", len(blocks), got)
	}
	if !strings.HasPrefix(blocks[0], "! LaTeX Error: File `foo.sty' not found.") {
		t.Errorf("block 1 = %q", blocks[0])
	}
	if strings.Contains(blocks[0], "Emergency stop") {
		t.Errorf("block 1 must not swallow the next error:\n%s", blocks[0])
	}
	if !strings.HasPrefix(blocks[1], "! Emergency stop.") ||
		!strings.Contains(blocks[1], "l.3 \\usepackage") ||
		!strings.Contains(blocks[1], "{foo}") {
		t.Errorf("block 2 must end at l.3 with its continuation line, got:\n%s", blocks[1])
	}
	if strings.Contains(got, "No pages of output") {
		t.Errorf("text after the source reference must be excluded:\n%s", got)
	}
}

func TestExtractTeXErrorsIgnoresLookalikes(t *testing.T) {
	// "l." must be followed by digits to count as a source reference.
	logs := "! Missing $ inserted.\nl.notaline\nl.12 $x^2\n"
	got := extractTeXErrors(logs)
	if !strings.HasSuffix(got, "l.12 $x^2") {
		t.Fatalf("block must end at l.12, got %q", got)
	}
}

func TestSummarizePrefersTeXLog(t *testing.T) {
	// latexmk may skip the rerun after a failure, leaving no "! " in stdout.
	stdout := "Latexmk: Nothing to do for 'main.tex'.\nLatexmk: Errors, so I did not complete making targets\n"
	got := summarize(stdout, sampleFailure)

	if !strings.HasPrefix(got, "! Undefined control sequence.\nl.4 \\foo") {
		t.Fatalf("errors from the .log must come first, got:\n%s", got)
	}
	if !strings.Contains(got, "Latexmk: Errors") {
		t.Errorf("the latexmk output tail must follow for diagnosis")
	}
}

func TestReadTeXLog(t *testing.T) {
	dir := t.TempDir()
	if got := readTeXLog(dir, "main.tex"); got != "" {
		t.Fatalf("missing .log must read as empty, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.log"), []byte(sampleFailure), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readTeXLog(dir, "main.tex"); got != sampleFailure {
		t.Fatalf("readTeXLog = %q", got)
	}
}

func TestSummarizePutsErrorsFirstAndCapsSize(t *testing.T) {
	huge := strings.Repeat("x", maxLogBytes*2) + "\n" + sampleFailure
	got := summarize(huge, "")

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
