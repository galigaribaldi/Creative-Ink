package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAffectsBuild(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "main.tex")
	assert.True(t, AffectsBuild(target, filepath.Join(dir, "refs.bib")))
	assert.True(t, AffectsBuild(target, filepath.Join(dir, "caps", "uno.TEX")))
	assert.False(t, AffectsBuild(target, filepath.Join(dir, "README.md")), "not read by any build")
	assert.False(t, AffectsBuild("", filepath.Join(dir, "datos.csv")))

	// The last build read datos.csv (pgfplots) and an absolute listing.
	listing := filepath.Join(t.TempDir(), "codigo.py")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.fls"), []byte(
		"PWD "+dir+"\nINPUT /usr/local/texlive/2026/texmf-dist/tex/latex/base/article.cls\nINPUT ./datos/datos.csv\nINPUT "+listing+"\nOUTPUT main.pdf\n"), 0o644))
	assert.True(t, AffectsBuild(target, filepath.Join(dir, "datos", "datos.csv")))
	assert.True(t, AffectsBuild(target, listing))
	assert.False(t, AffectsBuild(target, filepath.Join(dir, "notas.md")))
}
