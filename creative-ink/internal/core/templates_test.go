package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTemplatesRepositoryWins(t *testing.T) {
	repo, user := t.TempDir(), t.TempDir()
	for _, d := range []string{filepath.Join(repo, "Tesis-UNAM"), filepath.Join(user, "tesis-unam"),
		filepath.Join(user, "Articulo"), filepath.Join(user, ".oculta")} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(user, "suelto.tex"), nil, 0o644))

	list := ListTemplates(
		TemplateSource{Dir: repo, Source: TemplateSourceRepository},
		TemplateSource{Dir: user, Source: TemplateSourceUser},
		TemplateSource{Dir: filepath.Join(user, "no-existe"), Source: TemplateSourceUser},
	)
	require.Len(t, list, 2)
	assert.Equal(t, "Articulo", list[0].Name)
	assert.Equal(t, TemplateSourceUser, list[0].Source)
	assert.Equal(t, "Tesis-UNAM", list[1].Name)
	assert.Equal(t, TemplateSourceRepository, list[1].Source, "the repository's template wins the clash")
}

func TestCreateProjectFromTemplate(t *testing.T) {
	tpl := t.TempDir()
	mkfiles(t, tpl, "capitulos/intro.tex", "figuras/logo.png", "plantilla.cls", "main.aux", ".git/HEAD", ".DS_Store")
	require.NoError(t, os.WriteFile(filepath.Join(tpl, "tesis.tex"), []byte(`\title{{{TITULO}}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tpl, ".latexmkrc"), []byte(`$pdf_mode = 5;`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tpl, "figuras", "logo.png"), []byte{0x89, 'P', 'N', 'G', 0}, 0o644))
	if err := os.Symlink("/etc/passwd", filepath.Join(tpl, "enlace.tex")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root := t.TempDir()

	dir, mainTex, err := CreateProjectFromTemplate(root, "Tesis de Maestría & Co", tpl)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "Tesis-de-Maestria-Co"), dir)
	assert.Equal(t, filepath.Join(dir, "tesis.tex"), mainTex, "the only top-level .tex is the main file")

	data, _ := os.ReadFile(mainTex)
	assert.Equal(t, `\title{Tesis de Maestría \& Co}`, string(data), "{{TITULO}} is the escaped project name")
	png, _ := os.ReadFile(filepath.Join(dir, "figuras", "logo.png"))
	assert.Equal(t, []byte{0x89, 'P', 'N', 'G', 0}, png, "binary files are copied as they are")
	for _, rel := range []string{"capitulos/intro.tex", "plantilla.cls", ".latexmkrc"} {
		assert.FileExists(t, filepath.Join(dir, rel))
	}
	for _, rel := range []string{"main.aux", ".git", ".DS_Store", "enlace.tex"} {
		assert.NoFileExists(t, filepath.Join(dir, rel), rel)
	}
	meta, _ := LoadProjectMeta(dir)
	assert.Equal(t, "tesis.tex", meta.MainFile)

	// Same name again: never reuses (or overwrites) the existing project.
	_, _, err = CreateProjectFromTemplate(root, "Tesis de Maestria & Co", tpl)
	assert.ErrorIs(t, err, ErrProjectExists)
	_, _, err = CreateProjectFromTemplate(root, "Otra", filepath.Join(tpl, "no-existe"))
	assert.ErrorIs(t, err, ErrUnknownTemplate)
	assert.NoDirExists(t, filepath.Join(root, "Otra"))
}

func TestTemplateMetaSetsTheMainFile(t *testing.T) {
	tpl := t.TempDir()
	mkfiles(t, tpl, "main.tex", "src/tesis.tex")
	require.NoError(t, SaveProjectMeta(tpl, ProjectMeta{MainFile: "src/tesis.tex"}))
	_, mainTex, err := CreateProjectFromTemplate(t.TempDir(), "X", tpl)
	require.NoError(t, err)
	assert.Equal(t, "tesis.tex", filepath.Base(mainTex))
	assert.Equal(t, "src", filepath.Base(filepath.Dir(mainTex)))
}
