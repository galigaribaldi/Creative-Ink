package ci

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creative-ink/internal/core"
)

func TestRegistryHasTheNativeCommands(t *testing.T) {
	var names []string
	for _, c := range Commands() {
		names = append(names, c.Name)
		assert.NotNil(t, c.Run, c.Name)
		assert.NotEmpty(t, c.Usage, c.Name)
		assert.NotEmpty(t, c.Subcommands, c.Name)
	}
	assert.Equal(t, []string{"help", "compile", "clean", "config", "trust", "guide"}, names)
}

func TestHelpIsGeneratedFromTheRegistry(t *testing.T) {
	help := helpText()
	for _, c := range Commands() {
		for _, sc := range c.Subcommands {
			assert.Contains(t, help, "ci "+sc.Syntax)
		}
	}
}

func TestInfoCompletionsStopAtPlaceholders(t *testing.T) {
	completions := map[string]string{}
	for _, c := range Info() {
		for _, sc := range c.Subcommands {
			completions[sc.Syntax] = sc.Completion
		}
	}
	assert.Equal(t, "compile completo", completions["compile completo"])
	assert.Equal(t, "compile solo ", completions["compile solo <capítulo>"])
	assert.Equal(t, "config get ", completions["config get <clave>"])
	assert.Equal(t, "trust status", completions["trust status"])
}

func TestGuideIsGeneratedCompiledAndShown(t *testing.T) {
	r := &recorder{}
	env, _ := newEnv(t, r)
	env.GuideDir = filepath.Join(t.TempDir(), "creative-ink", "guide")
	var compiled, shown string
	env.compileGuide = func(_ context.Context, workdir, filename string) (string, error) {
		compiled = filepath.Join(workdir, filename)
		return "ok", nil
	}
	env.ShowGuide = func(pdf string) error { shown = pdf; return nil }

	out, err := Run(context.Background(), "ci guide", env)
	require.NoError(t, err)
	tex := filepath.Join(env.GuideDir, GuideFile)
	assert.Equal(t, tex, compiled)
	assert.Equal(t, filepath.Join(env.GuideDir, "ci-guia.pdf"), shown)
	assert.Contains(t, out, "✓ Guía generada")

	data, err := os.ReadFile(tex)
	require.NoError(t, err)
	src := string(data)
	for _, c := range Commands() {
		assert.Contains(t, src, `\texttt{ci `+c.Name+`}`)
	}
	// Registry text is escaped for LaTeX: no raw backslash commands leak in.
	assert.Contains(t, src, `\textbackslash{}borrador`)
	assert.Contains(t, src, `$\to$`)
	assert.True(t, strings.HasSuffix(src, "\\end{document}\n"))

	env.GuideDir = ""
	_, err = Run(context.Background(), "ci guide", env)
	assert.Error(t, err)
	_, err = Run(context.Background(), "ci guide extra", env)
	assert.ErrorIs(t, err, ErrUsage)
}

func TestGuideTeXDateIsSpanish(t *testing.T) {
	src := GuideTeX(nil, time.Date(2026, time.November, 10, 0, 0, 0, 0, time.UTC))
	assert.Contains(t, src, `\date{10 de noviembre de 2026}`)
}

func TestRequiresTrustIsEnforced(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	ran := false
	registry = append(Commands(), CommandDef{
		Name: "peligroso", Usage: "ci peligroso", RequiresTrust: true,
		Subcommands: []Subcommand{{"peligroso", "x"}},
		Run:         func(context.Context, []string, Env) (string, error) { ran = true; return "", nil },
	})
	r := &recorder{}
	env, _ := newEnv(t, r)
	trust := &fakeTrust{info: core.TrustInfo{Status: core.TrustUntrusted, RCFiles: []string{".latexmkrc"}}}
	env.Trust = trust

	_, err := Run(context.Background(), "ci peligroso", env)
	assert.ErrorIs(t, err, ErrNeedsTrust)
	assert.False(t, ran)
	trust.info.Status = core.TrustTrusted
	_, err = Run(context.Background(), "ci peligroso", env)
	require.NoError(t, err)
	assert.True(t, ran)
}
