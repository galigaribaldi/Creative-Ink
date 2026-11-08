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
	"time"

	"creative-ink/internal/system"
)

// ManualBuildTimeout bounds a whole manual build (`ci compile`, `ci clean`):
// a thesis with three XeLaTeX passes and BibTeX can take minutes, far more
// than the terminal's 30 s.
const ManualBuildTimeout = 10 * time.Minute

// tailLines is how much of a failed pass's output is shown when it has no
// TeX error block ("! …").
const tailLines = 15

// Pass is one step of a manual build, run directly (no shell).
type Pass struct {
	Tool string   // binary name (resolved with system.FindBinary) or absolute path
	Args []string // passed verbatim: TeX code such as `\def\x{}\input{doc}` needs no quoting
	// ContinueOnError mirrors `|| true` in a Makefile: a failure is reported
	// but the sequence goes on (e.g. the first XeLaTeX pass before BibTeX).
	ContinueOnError bool
}

// RunPasses runs passes in workdir, in order, while holding the compile
// lock: they never overlap latexmk from the Scheduler (two TeX runs in the
// same folder corrupt the .aux files). after, if not nil, also runs under
// the lock once no blocking pass failed; its text is appended to the report.
//
// The report is compact (one line per pass; TeX errors or an output tail
// for failures) because it is shown in the terminal. Cancelling ctx kills
// the running tool.
func RunPasses(ctx context.Context, workdir string, passes []Pass, after func() string) (string, error) {
	compileMu.Lock()
	defer compileMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, ManualBuildTimeout)
	defer cancel()

	var report strings.Builder
	for _, p := range passes {
		bin, err := resolveTool(p.Tool)
		if err != nil {
			return report.String(), err
		}
		fmt.Fprintf(&report, "▶ %s %s\n", p.Tool, strings.Join(p.Args, " "))

		start := time.Now()
		out, runErr := RunTool(ctx, bin, workdir, p.Args)
		elapsed := time.Since(start).Seconds()

		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return report.String(), fmt.Errorf("se superó el límite de %s", ManualBuildTimeout)
		case ctx.Err() != nil:
			return report.String(), ctx.Err()
		case runErr != nil:
			fmt.Fprintf(&report, "  ✗ falló en %.1f s\n%s\n", elapsed, indent(failureSummary(out)))
			if !p.ContinueOnError {
				return report.String(), fmt.Errorf("%s falló: %w", p.Tool, runErr)
			}
		default:
			fmt.Fprintf(&report, "  ✓ %.1f s\n", elapsed)
		}
	}
	if after != nil {
		report.WriteString(after())
	}
	return report.String(), nil
}

// WithCompileLock runs fn while no compilation can run (e.g. to delete
// auxiliary files without racing latexmk).
func WithCompileLock(fn func()) {
	compileMu.Lock()
	defer compileMu.Unlock()
	fn()
}

// RunTool executes bin directly (no shell) in workdir with the TeX/Homebrew
// PATH and no stdin, returning Stdout+Stderr combined.
func RunTool(ctx context.Context, bin, workdir string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "PATH="+system.ToolPATH())
	cmd.Stdin = nil
	cmd.WaitDelay = 2 * time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func resolveTool(tool string) (string, error) {
	if filepath.IsAbs(tool) {
		return tool, nil
	}
	if bin, ok := system.FindBinary(tool); ok {
		return bin, nil
	}
	return "", fmt.Errorf("%s no está instalado o no está en el PATH", tool)
}

// failureSummary prefers TeX error blocks; otherwise the last lines.
func failureSummary(out string) string {
	if errs := extractTeXErrors(out); errs != "" {
		return errs
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > tailLines {
		lines = append([]string{"…"}, lines[len(lines)-tailLines:]...)
	}
	return strings.Join(lines, "\n")
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}
