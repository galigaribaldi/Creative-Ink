package compiler

// Integration and stress tests against the real TeX toolchain. They are
// opt-in: they need latexmk + xelatex installed and take seconds to minutes.
//
//	CREATIVE_INK_LATEX_TEST=1 go test ./internal/compiler -run Integration -v -timeout 10m
//
// Without the variable they are skipped, so `go test ./...` stays fast.

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creative-ink/internal/core"
	"creative-ink/internal/system"
)

// requireLaTeX skips unless CREATIVE_INK_LATEX_TEST is set; once asked
// for, a missing toolchain is a failure, not a skip.
func requireLaTeX(t *testing.T) {
	t.Helper()
	if os.Getenv("CREATIVE_INK_LATEX_TEST") == "" {
		t.Skip("set CREATIVE_INK_LATEX_TEST=1 to run against the real latexmk")
	}
	for _, tool := range []string{"latexmk", "xelatex"} {
		if _, ok := system.FindBinary(tool); !ok {
			t.Fatalf("%s not found: install TeX (MacTeX/BasicTeX, texlive) to run the integration tests", tool)
		}
	}
}

// save writes a project file the way the editor does (atomic FileManager).
func save(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, core.NewFileManager().SaveDocument(content, path))
	return path
}

func document(body string) string {
	return "\\documentclass{article}\n\\begin{document}\n" + body + "\n\\end{document}\n"
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// ── Layout B: multi-file project ───────────────────────────────────────────

// TestIntegrationLayoutBMultiFile: main.tex pulls cap1.tex with \input.
// Saving the chapter and running the pipeline (Scheduler → latexmk) builds
// main.tex, which picks up the new chapter text. (The App-level rule
// "saving a chapter compiles the main file" is TestSetMainFile… in the
// root package.)
func TestIntegrationLayoutBMultiFile(t *testing.T) {
	requireLaTeX(t)
	dir := t.TempDir()
	save(t, dir, "main.tex", document(`\input{cap1.tex}`))
	chapter := save(t, dir, "cap1.tex", "Primera versión del capítulo.\n")

	results := make(chan Result, 4)
	s := NewScheduler(func(ctx context.Context) (string, error) {
		return CompileLaTeXContext(ctx, dir, "main.tex") // the compile target, never the chapter
	}, Hooks{OnFinished: func(r Result) { results <- r }})
	defer s.Close(5 * time.Second)
	build := func() Result {
		t.Helper()
		require.True(t, s.Request(TriggerManual))
		select {
		case r := <-results:
			return r
		case <-time.After(compileTimeout + 10*time.Second):
			require.FailNow(t, "the build did not finish")
			return Result{}
		}
	}

	first := build()
	require.True(t, first.Success, first.Logs)
	pdf := filepath.Join(dir, "main.pdf")
	before, err := os.Stat(pdf)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "cap1.pdf"), "only the main document is built")
	// latexmk records main's inputs (-recorder): the chapter belongs to main.
	assert.Contains(t, readFile(t, filepath.Join(dir, "main.fls")), "cap1.tex")

	// Save the chapter like the editor (atomic) and run the pipeline again.
	require.NoError(t, core.NewFileManager().SaveDocument(
		strings.Repeat("Segunda versión, bastante más larga. ", 300)+"\n", chapter))
	second := build()
	require.True(t, second.Success, second.Logs)
	after, err := os.Stat(pdf)
	require.NoError(t, err)
	assert.True(t, after.ModTime().After(before.ModTime()), "main.pdf must be rebuilt after the chapter changed")
	assert.NotEqual(t, before.Size(), after.Size())
	assert.Contains(t, readFile(t, filepath.Join(dir, "main.log")), "cap1.tex")
}

// ── Layout C: error parsing in a sub-file ──────────────────────────────────

// TestIntegrationLayoutCErrorInSubfile: an undefined command in a chapter.
// CompileLaTeXContext must return the TeX error block ("! …" up to
// "l.<line>") and the output must name the chapter that caused it.
func TestIntegrationLayoutCErrorInSubfile(t *testing.T) {
	requireLaTeX(t)
	dir := t.TempDir()
	save(t, dir, "main.tex", document(`\input{capitulos/cap1}`))
	save(t, dir, "capitulos/cap1.tex", "Línea uno.\n\nLínea tres: \\noexiste\n")

	logs, err := CompileLaTeXContext(context.Background(), dir, "main.tex")
	require.Error(t, err, "a broken chapter must fail the build")

	// The error block comes first, with the source line.
	assert.True(t, strings.HasPrefix(logs, "! Undefined control sequence."), "logs must start with the TeX error:\n%s", logs)
	assert.Regexp(t, `l\.3 .*\\noexiste`, logs)

	// TeX's own .log places the error inside capitulos/cap1.tex …
	texLog := readFile(t, filepath.Join(dir, "main.log"))
	errAt := strings.Index(texLog, "! Undefined control sequence.")
	require.Positive(t, errAt)
	assert.True(t, fileOpenAt(texLog[:errAt], "./capitulos/cap1.tex"),
		"the error must happen while capitulos/cap1.tex is the file being read")
	// … and what the user sees names the sub-file. Today this comes from
	// the latexmk output tail appended after the error blocks: the block
	// itself has no path (that would need -file-line-error, see MAP §4.3).
	assert.Contains(t, logs, "capitulos/cap1.tex")
}

// fileOpenAt reports whether file is still open at the end of a TeX log
// prefix: TeX prints "(<file>" when it opens a file and ")" when it closes
// it, so after the last "(<file>" opens must outnumber closes.
func fileOpenAt(logPrefix, file string) bool {
	i := strings.LastIndex(logPrefix, "("+file)
	if i < 0 {
		return false
	}
	rest := logPrefix[i:]
	return strings.Count(rest, "(") > strings.Count(rest, ")")
}

// ── Layout D: stress test with a massive thesis ────────────────────────────

const thesisPages = 200

// massiveThesis writes a main.tex with thesisPages pages of random text
// (deterministic seed, so every run compiles the same document).
func massiveThesis(t *testing.T, dir string) {
	t.Helper()
	rng := rand.New(rand.NewPCG(2026, 11))
	var b strings.Builder
	b.WriteString("\\documentclass[12pt]{article}\n\\begin{document}\n")
	for p := 1; p <= thesisPages; p++ {
		fmt.Fprintf(&b, "\\section*{Pagina %d}\n", p)
		for par := 0; par < 3; par++ {
			b.WriteString(randomParagraph(rng, 110))
			b.WriteString("\n\n")
		}
		b.WriteString("\\newpage\n")
	}
	b.WriteString("\\end{document}\n")
	save(t, dir, "main.tex", b.String())
}

// randomParagraph returns words of random lowercase letters (ASCII only:
// no font or encoding warnings), as a capitalised sentence.
func randomParagraph(rng *rand.Rand, words int) string {
	var b strings.Builder
	for w := 0; w < words; w++ {
		if w > 0 {
			b.WriteByte(' ')
		}
		n := 2 + rng.IntN(9)
		for i := 0; i < n; i++ {
			c := byte('a' + rng.IntN(26))
			if w == 0 && i == 0 {
				c -= 'a' - 'A'
			}
			b.WriteByte(c)
		}
	}
	b.WriteByte('.')
	return b.String()
}

// TestIntegrationLayoutDMassiveThesis: 200 pages must compile within the
// timeout, produce the PDF, and keep the app's own memory bounded (latexmk
// runs in its own process; Go only holds its output).
func TestIntegrationLayoutDMassiveThesis(t *testing.T) {
	requireLaTeX(t)
	dir := t.TempDir()
	massiveThesis(t, dir)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	start := time.Now()
	logs, err := CompileLaTeXContext(context.Background(), dir, "main.tex")
	elapsed := time.Since(start)
	require.NoError(t, err, logs)

	runtime.GC()
	runtime.ReadMemStats(&after)
	retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	allocated := after.TotalAlloc - before.TotalAlloc
	pages := PagesFromLog(filepath.Join(dir, "main.log"))
	info, statErr := os.Stat(filepath.Join(dir, "main.pdf"))
	require.NoError(t, statErr, "the PDF must be generated")
	t.Logf("%d páginas, PDF de %.1f MB en %s · heap Go retenido %+d KB, asignado %d KB durante la compilación",
		pages, float64(info.Size())/(1<<20), elapsed.Round(time.Millisecond), retained/1024, allocated/1024)

	assert.GreaterOrEqual(t, pages, thesisPages)
	assert.Less(t, elapsed, compileTimeout, "a 200-page document must fit in the automatic-compile timeout")
	assert.Less(t, retained, int64(16<<20), "the app must not retain the build in memory")
	assert.Less(t, allocated, uint64(256<<20), "the output buffers must stay bounded")
}

// TestIntegrationCancelMassiveThenRecompile: the Scheduler's bounded
// cancellation against real latexmk. An automatic build of the massive
// thesis is cancelled while xelatex runs (KillTreeOnCancel kills the whole
// tree) and the follow-up must still produce a valid 200-page PDF: the
// interrupted .aux must not break the next build.
func TestIntegrationCancelMassiveThenRecompile(t *testing.T) {
	requireLaTeX(t)
	dir := t.TempDir()
	massiveThesis(t, dir)

	started := make(chan struct{}, 4)
	results := make(chan Result, 4)
	s := NewScheduler(func(ctx context.Context) (string, error) {
		return CompileLaTeXContext(ctx, dir, "main.tex")
	}, Hooks{
		OnStarted:  func() { started <- struct{}{} },
		OnFinished: func(r Result) { results <- r },
	})
	defer s.Close(5 * time.Second)
	wait := func() Result {
		t.Helper()
		select {
		case r := <-results:
			return r
		case <-time.After(compileTimeout + 10*time.Second):
			require.FailNow(t, "the build did not finish")
			return Result{}
		}
	}

	require.True(t, s.Request(TriggerAuto))
	<-started
	time.Sleep(700 * time.Millisecond) // xelatex is now typesetting
	require.True(t, s.Request(TriggerAuto))

	first := wait()
	if !first.Canceled {
		t.Skipf("the first build finished in under 0.7 s (%d ms): nothing to cancel on this machine", first.DurationMs)
	}
	second := wait()
	require.True(t, second.Success, "the build after a cancellation must succeed:\n%s", second.Logs)
	assert.GreaterOrEqual(t, PagesFromLog(filepath.Join(dir, "main.log")), thesisPages)
	t.Logf("cancelada a los %d ms; la siguiente tardó %d ms", first.DurationMs, second.DurationMs)
}
