package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creative-ink/internal/core"
)

func TestCreateFolderAndFileFromTheExplorer(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, _ := a.workspace()

	st, err := a.CreateFolder(folder, "Coloquio 2027-1")
	require.NoError(t, err)
	dir := filepath.Join(folder, "Coloquio-2027-1")
	assert.DirExists(t, dir)
	assert.Equal(t, folder, st.FolderPath)

	st, err = a.CreateFile(dir, "Capítulo Dos")
	require.NoError(t, err)
	file := filepath.Join(dir, "Capitulo-Dos.tex")
	assert.FileExists(t, file)
	assert.Equal(t, file, st.FilePath, "the new file becomes the active document")

	_, err = a.CreateFile(dir, "notas.MD")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "notas.md"))

	// Never replaces, never escapes, only text files.
	_, err = a.CreateFolder(folder, "Coloquio 2027-1")
	assert.ErrorContains(t, err, "ya existe")
	_, err = a.CreateFile(dir, "Capitulo-Dos")
	assert.ErrorIs(t, err, core.ErrFileExists)
	_, err = a.CreateFile(filepath.Dir(folder), "fuera")
	assert.Error(t, err)
	_, err = a.CreateFolder(folder, "../fuera")
	assert.ErrorIs(t, err, errBadName)
	_, err = a.CreateFile(folder, "foto.png")
	assert.ErrorIs(t, err, core.ErrNotEditable)
	_, err = a.CreateFolder(folder, "¿?")
	assert.ErrorIs(t, err, errBadName)
}

func TestSavingANonLaTeXFileDoesNotAskToCompile(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	readme := filepath.Join(folder, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# x"), 0o644))
	res, err := a.SaveContent(readme, "# y")
	require.NoError(t, err)
	assert.False(t, res.AffectsBuild)
	res, err = a.SaveContent(filepath.Join(folder, "main.tex"), "\\documentclass{article}")
	require.NoError(t, err)
	assert.True(t, res.AffectsBuild)
}
