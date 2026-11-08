package ci

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"creative-ink/internal/compiler"
)

// recorder captures the passes a command would run, without TeX.
type recorder struct {
	passes   []compiler.Pass
	workdir  string
	started  int
	finished int
}

func newEnv(t *testing.T, r *recorder) (Env, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "tesis.tex")
	auto := true
	return Env{
		Target:         target,
		AutoCompile:    func() (bool, error) { return auto, nil },
		SetAutoCompile: func(on bool) error { auto = on; return nil },
		BuildStarted:   func() { r.started++ },
		BuildFinished:  func(time.Duration, error, string) { r.finished++ },
		runPasses: func(ctx context.Context, workdir string, passes []compiler.Pass, after func() string) (string, error) {
			r.passes, r.workdir = passes, workdir
			return "ok\n" + after(), nil
		},
	}, dir
}

func tools(passes []compiler.Pass) []string {
	out := make([]string, len(passes))
	for i, p := range passes {
		out[i] = p.Tool + " " + strings.Join(p.Args, " ")
	}
	return out
}

func TestIsCommand(t *testing.T) {
	for line, want := range map[string]bool{
		"ci compile rapido": true, "   ci help": true, "ci": true,
		"cid": false, "echo ci": false, "": false, "git status": false,
	} {
		if got := IsCommand(line); got != want {
			t.Errorf("IsCommand(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestCompileVariantsTranslateTheMakefile(t *testing.T) {
	cases := map[string][]string{
		"ci compile rapido": {"xelatex -interaction=nonstopmode -synctex=1 tesis.tex"},
		"ci compile borrador": {
			`xelatex -interaction=nonstopmode -synctex=1 \def\borrador{}\input{tesis}`,
			`xelatex -interaction=nonstopmode -synctex=1 \def\borrador{}\input{tesis}`,
		},
		"ci compile completo": {
			"xelatex -interaction=nonstopmode -synctex=1 tesis.tex",
			"bibtex tesis",
			"xelatex -interaction=nonstopmode -synctex=1 tesis.tex",
			"xelatex -interaction=nonstopmode -synctex=1 tesis.tex",
		},
		"ci compile solo 6-Analisis-SocioEspacial/1-Analisis": {
			`xelatex -interaction=nonstopmode -synctex=1 -jobname=tesis_6-Analisis-SocioEspacial \def\SoloCapitulo{6-Analisis-SocioEspacial/1-Analisis}\input{tesis}`,
			`xelatex -interaction=nonstopmode -synctex=1 -jobname=tesis_6-Analisis-SocioEspacial \def\SoloCapitulo{6-Analisis-SocioEspacial/1-Analisis}\input{tesis}`,
		},
	}
	for line, want := range cases {
		r := &recorder{}
		env, dir := newEnv(t, r)
		if _, err := Run(context.Background(), line, env); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		got := tools(r.passes)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", line, got, want)
		}
		if r.workdir != dir {
			t.Errorf("%s: workdir = %q, want the main document's folder", line, r.workdir)
		}
	}
}

func TestCompileContinueOnErrorMatchesMakefile(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	if _, err := Run(context.Background(), "ci compile completo", env); err != nil {
		t.Fatal(err)
	}
	// xelatex || true ; bibtex ; xelatex || true ; xelatex
	want := []bool{true, false, true, false}
	for i, p := range r.passes {
		if p.ContinueOnError != want[i] {
			t.Errorf("pass %d (%s): ContinueOnError = %v, want %v", i, p.Tool, p.ContinueOnError, want[i])
		}
	}
}

func TestCompileUpdatesThePreviewExceptSolo(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	_, _ = Run(context.Background(), "ci compile rapido", env)
	if r.started != 1 || r.finished != 1 {
		t.Fatalf("rapido: started=%d finished=%d", r.started, r.finished)
	}
	out, _ := Run(context.Background(), "ci compile solo cap1", env)
	if r.started != 1 || r.finished != 1 {
		t.Fatal("solo writes a separate PDF: it must not refresh the preview")
	}
	if !strings.Contains(out, "tesis_cap1.pdf") {
		t.Errorf("solo must report its PDF, got %q", out)
	}
}

func TestCompileSoloRejectsTeXInjection(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	for _, bad := range []string{`}\immediate\write18{rm`, "../fuera", "a{b}", "-x", `\input{x}`} {
		_, err := Run(context.Background(), "ci compile solo "+bad, env)
		if !errors.Is(err, ErrUsage) || r.passes != nil {
			t.Errorf("chapter %q: err = %v, passes = %v; must be rejected before running", bad, err, r.passes)
		}
	}
}

func TestConfigAutocompile(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	if out, err := Run(context.Background(), "ci config autocompile off", env); err != nil || !strings.Contains(out, "off") {
		t.Fatalf("off: %q, %v", out, err)
	}
	if out, _ := Run(context.Background(), "ci config autocompile", env); !strings.Contains(out, "off") {
		t.Fatalf("show: %q", out)
	}
	if _, err := Run(context.Background(), "ci config autocompile maybe", env); !errors.Is(err, ErrUsage) {
		t.Fatalf("bad value: err = %v", err)
	}
}

func TestCleanTikzRemovesOnlyTheCache(t *testing.T) {
	r := &recorder{}
	env, dir := newEnv(t, r)
	cache := filepath.Join(dir, "Figures", "TiKz_Libraries", "externalized")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"fig1.pdf", "fig1.md5", "fig1.log", "fig1.dpth", "fig1.tex"} {
		if err := os.WriteFile(filepath.Join(cache, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := Run(context.Background(), "ci clean tikz", env)
	if err != nil || !strings.Contains(out, "4 archivos") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "fig1.tex")); err != nil {
		t.Fatal("sources in the cache folder must be kept")
	}
}

func TestCleanAllRemovesLeftovers(t *testing.T) {
	r := &recorder{}
	env, dir := newEnv(t, r)
	for _, n := range []string{"tesis.bbl", "tesis.xdv", "tesis.tex"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := Run(context.Background(), "ci clean all", env)
	if err != nil || !strings.Contains(out, "2 archivos") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if got := tools(r.passes); len(got) != 1 || got[0] != "latexmk -C tesis.tex" {
		t.Fatalf("passes = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "tesis.tex")); err != nil {
		t.Fatal("the source must never be deleted")
	}
}

func TestHelpAndErrors(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	if out, err := Run(context.Background(), "ci", env); err != nil || !strings.Contains(out, "ci compile completo") {
		t.Fatalf("help: %q, %v", out, err)
	}
	// Project-specific tools (e.g. a bibliography check) will come from plugins.
	if _, err := Run(context.Background(), "ci check bib", env); !errors.Is(err, ErrUsage) {
		t.Errorf("ci check was removed: err = %v", err)
	}
	if _, err := Run(context.Background(), "ci volar", env); !errors.Is(err, ErrUsage) {
		t.Errorf("unknown: err = %v", err)
	}
	env.Target = ""
	if _, err := Run(context.Background(), "ci compile rapido", env); err == nil {
		t.Error("without a document compile must fail")
	}
}
