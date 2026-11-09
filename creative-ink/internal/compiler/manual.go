package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

// maxTailBytes is all the output of a pass kept in memory (for the failure
// summary). The full output streams to BuildOutput.Log instead: a 200-page
// thesis writes several MB of warnings per XeLaTeX pass.
const maxTailBytes = 256 * 1024

// progressInterval throttles BuildOutput.Progress (pass boundaries always
// report): at most ~4 events per second cross the IPC bridge.
const progressInterval = 250 * time.Millisecond

// BuildOutput receives a manual build's output while it runs (docs/MAP.md
// §4.3.6). The zero value discards it.
type BuildOutput struct {
	// Log gets every byte the passes write (stdout and stderr), with a
	// "▶ tool args" header per pass. The App spools it to disk so the
	// terminal can read it on demand. nil = discarded.
	Log io.Writer
	// Progress is called at each pass boundary and, while a pass runs, at
	// most every progressInterval. Never concurrently.
	Progress func(BuildProgress)
	// ExpectedPages is the page count of the previous build (0 = unknown),
	// to show "página 143 de 212" before this one finishes.
	ExpectedPages int
}

// BuildProgress is a snapshot of a running manual build.
type BuildProgress struct {
	Pass      int    `json:"pass"`   // 1-based
	Passes    int    `json:"passes"` // total
	Tool      string `json:"tool"`   // xelatex, bibtex…
	Page      int    `json:"page"`   // last page shipped by the current pass (TeX "[n]"), 0 if none
	Pages     int    `json:"pages"`  // expected or final page count, 0 if unknown
	ElapsedMs int64  `json:"elapsedMs"`
	Done      bool   `json:"done"`   // the build ended (successfully or not)
	Failed    bool   `json:"failed"` // the current pass failed
}

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
// for failures) because it is shown in the terminal; the full output goes
// to out.Log while it runs, and out.Progress follows the passes and pages.
// Memory stays bounded (maxTailBytes per pass). Cancelling ctx kills the
// running tool and its children.
func RunPasses(ctx context.Context, workdir string, passes []Pass, after func() string, out BuildOutput) (string, error) {
	compileMu.Lock()
	defer compileMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, ManualBuildTimeout)
	defer cancel()

	progress := &progressTracker{out: out, start: time.Now()}
	var report strings.Builder
	finish := func(failed bool) { progress.finish(failed) }
	for i, p := range passes {
		bin, err := resolveTool(p.Tool)
		if err != nil {
			finish(true)
			return report.String(), err
		}
		header := fmt.Sprintf("▶ %s %s\n", p.Tool, strings.Join(p.Args, " "))
		report.WriteString(header)
		if out.Log != nil {
			_, _ = io.WriteString(out.Log, header)
		}
		progress.begin(i, len(passes), p.Tool)

		w := &passWriter{log: out.Log, onLine: progress.line}
		start := time.Now()
		runErr := runToolTo(ctx, bin, workdir, p.Args, w)
		w.flush()
		elapsed := time.Since(start).Seconds()

		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			finish(true)
			return report.String(), fmt.Errorf("se superó el límite de %s", ManualBuildTimeout)
		case ctx.Err() != nil:
			finish(true)
			return report.String(), ctx.Err()
		case runErr != nil:
			fmt.Fprintf(&report, "  ✗ falló en %.1f s\n%s\n", elapsed, indent(failureSummary(w.tail.String())))
			if !p.ContinueOnError {
				finish(true)
				return report.String(), fmt.Errorf("%s falló: %w", p.Tool, runErr)
			}
		default:
			fmt.Fprintf(&report, "  ✓ %.1f s\n", elapsed)
		}
	}
	if after != nil {
		report.WriteString(after())
	}
	finish(false)
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
	var out bytes.Buffer
	err := runToolTo(ctx, bin, workdir, args, &out)
	return out.String(), err
}

// runToolTo is RunTool streaming Stdout+Stderr to w. Cancelling ctx kills
// the tool and every child it started.
func runToolTo(ctx context.Context, bin, workdir string, args []string, w io.Writer) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "PATH="+system.ToolPATH())
	cmd.Stdin = nil
	system.KillTreeOnCancel(cmd)
	cmd.WaitDelay = 2 * time.Second
	// The same writer for both: os/exec then serialises the Write calls.
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// ── Streaming output ───────────────────────────────────────────────────────

// passWriter copies a pass's output to the log, keeps only its tail in
// memory and hands complete lines to onLine.
type passWriter struct {
	log    io.Writer
	tail   tailBuffer
	line   []byte // incomplete last line (capped)
	onLine func(string)
}

// maxLineBytes caps a single line kept for parsing (TeX wraps at 79).
const maxLineBytes = 4096

func (w *passWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.log != nil {
		_, _ = w.log.Write(p) // a full disk must not fail the build
	}
	w.tail.Write(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if room := maxLineBytes - len(w.line); room > 0 {
				w.line = append(w.line, p[:min(room, len(p))]...)
			}
			break
		}
		w.line = append(w.line, p[:min(i, max(0, maxLineBytes-len(w.line)))]...)
		w.emitLine()
		p = p[i+1:]
	}
	return n, nil
}

// flush hands over the last line when the output did not end in "\n".
func (w *passWriter) flush() {
	if len(w.line) > 0 {
		w.emitLine()
	}
}

func (w *passWriter) emitLine() {
	if w.onLine != nil {
		w.onLine(strings.TrimRight(string(w.line), "\r"))
	}
	w.line = w.line[:0]
}

// tailBuffer keeps the last maxTailBytes written to it.
type tailBuffer struct{ buf []byte }

func (t *tailBuffer) Write(p []byte) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*maxTailBytes {
		t.buf = append(t.buf[:0:0], t.buf[len(t.buf)-maxTailBytes:]...)
	}
}

func (t *tailBuffer) String() string {
	if len(t.buf) > maxTailBytes {
		return string(t.buf[len(t.buf)-maxTailBytes:])
	}
	return string(t.buf)
}

var (
	// pageMarker matches the "[12]" TeX prints when it ships out page 12
	// (also "[12{…pdftex.map}]" and "[12" wrapped at the end of a line).
	pageMarker = regexp.MustCompile(`\[(\d+)(?:[\]\s{<]|$)`)
	// outputWritten is TeX's last line: "Output written on tesis.pdf (212 pages, …)".
	outputWritten = regexp.MustCompile(`Output written on .*\((\d+) pages?`)
)

// progressTracker turns output lines into throttled BuildProgress calls.
type progressTracker struct {
	out   BuildOutput
	start time.Time
	last  time.Time
	cur   BuildProgress
}

func (t *progressTracker) begin(i, passes int, tool string) {
	pages := t.cur.Pages
	if pages == 0 {
		pages = t.out.ExpectedPages
	}
	t.cur = BuildProgress{Pass: i + 1, Passes: passes, Tool: tool, Pages: pages}
	t.emit(true)
}

func (t *progressTracker) line(text string) {
	if m := outputWritten.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			t.cur.Pages = n
			t.cur.Page = n
		}
		t.emit(true)
		return
	}
	for _, m := range pageMarker.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > t.cur.Page {
			t.cur.Page = n
		}
	}
	t.emit(false)
}

func (t *progressTracker) finish(failed bool) {
	t.cur.Done, t.cur.Failed = true, failed
	t.emit(true)
}

func (t *progressTracker) emit(force bool) {
	if t.out.Progress == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(t.last) < progressInterval {
		return
	}
	t.last = now
	t.cur.ElapsedMs = now.Sub(t.start).Milliseconds()
	t.out.Progress(t.cur)
}

// PagesFromLog reads the page count of the last build from a TeX .log
// ("Output written on … (212 pages"), or 0. Only its tail is read.
func PagesFromLog(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	const tail = 16 * 1024
	if info, err := f.Stat(); err == nil && info.Size() > tail {
		_, _ = f.Seek(-tail, io.SeekEnd)
	}
	data, _ := io.ReadAll(io.LimitReader(f, tail))
	if m := outputWritten.FindSubmatch(data); m != nil {
		n, _ := strconv.Atoi(string(m[1]))
		return n
	}
	return 0
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
