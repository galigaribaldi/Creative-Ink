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

func TestSearchProjectSearchesTheOpenFolder(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	require.NoError(t, os.WriteFile(filepath.Join(folder, "notas.md"), []byte("Una nota\notra Nota"), 0o644))
	res, err := a.SearchProject(core.SearchQuery{Text: "nota"})
	require.NoError(t, err)
	require.Len(t, res.Files, 1)
	assert.Equal(t, "notas.md", res.Files[0].Rel)
	assert.Equal(t, 2, res.Total)

	empty := newApp("", "", okCompile)
	_, err = empty.SearchProject(core.SearchQuery{Text: "x"})
	assert.ErrorIs(t, err, core.ErrNoFolder)
}

func TestSetActiveDocumentFollowsTheFocusedPane(t *testing.T) {
	a, events := newTestApp(t, okCompile)
	folder, mainTex := a.workspace()
	poster := filepath.Join(folder, "poster.tex")
	notes := filepath.Join(folder, "notas.md")
	require.NoError(t, os.WriteFile(poster, []byte("\\documentclass{beamer}"), 0o644))
	require.NoError(t, os.WriteFile(notes, []byte("# notas"), 0o644))

	got, err := a.SetActiveDocument(poster)
	require.NoError(t, err)
	assert.Equal(t, poster, got.CompileTarget, "the focused .tex compiles")
	assert.True(t, got.AffectsBuild)
	expectCompileOf(t, events, poster)

	// A non-TeX pane keeps the last .tex as the target.
	got, err = a.SetActiveDocument(notes)
	require.NoError(t, err)
	assert.Equal(t, poster, got.CompileTarget)
	assert.False(t, got.AffectsBuild)
	expectNoEvent(t, events)

	// A main file (★) wins whatever pane is focused.
	_, err = a.SetMainFile(mainTex)
	require.NoError(t, err)
	expectCompileOf(t, events, mainTex)
	got, err = a.SetActiveDocument(poster)
	require.NoError(t, err)
	assert.Equal(t, mainTex, got.CompileTarget)

	_, err = a.SetActiveDocument(filepath.Join(filepath.Dir(folder), "fuera.tex"))
	assert.Error(t, err, "only files of the open folder")
}

func TestMoveEntryKeepsTheMainFileAndTheActiveDocument(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, mainTex := a.workspace()
	caps := filepath.Join(folder, "capitulos")
	require.NoError(t, os.MkdirAll(caps, 0o755))
	notes := filepath.Join(folder, "notas.md")
	require.NoError(t, os.WriteFile(notes, []byte("x"), 0o644))
	_, err := a.SetMainFile(mainTex)
	require.NoError(t, err)

	// A file into a folder.
	res, err := a.MoveEntry(notes, caps)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(caps, "notas.md"), res.To)
	assert.FileExists(t, res.To)
	assert.NoFileExists(t, notes)

	// The main file moves too: .creative-ink-meta follows it.
	res, err = a.MoveEntry(mainTex, caps)
	require.NoError(t, err)
	newMain := filepath.Join(caps, "main.tex")
	assert.Equal(t, newMain, res.State.MainFilePath)
	meta, err := core.LoadProjectMeta(folder)
	require.NoError(t, err)
	assert.Equal(t, "capitulos/main.tex", meta.MainFile)
	_, doc := a.workspace()
	assert.Equal(t, newMain, doc, "Go's active document follows the move")

	// A folder (and everything in it) into another one.
	partes := filepath.Join(folder, "partes")
	require.NoError(t, os.MkdirAll(partes, 0o755))
	res, err = a.MoveEntry(caps, partes)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(partes, "capitulos"), res.To)
	assert.Equal(t, filepath.Join(partes, "capitulos", "main.tex"), res.State.MainFilePath)

	// Never into itself, never replacing, never outside.
	_, err = a.MoveEntry(partes, filepath.Join(partes, "capitulos"))
	assert.ErrorContains(t, err, "dentro de sí misma")
	require.NoError(t, os.WriteFile(filepath.Join(folder, "main.tex"), []byte("otro"), 0o644))
	_, err = a.MoveEntry(filepath.Join(folder, "main.tex"), filepath.Join(partes, "capitulos"))
	assert.ErrorContains(t, err, "no se reemplaza")
	_, err = a.MoveEntry(filepath.Join(folder, "main.tex"), filepath.Dir(folder))
	assert.Error(t, err)
	_, err = a.MoveEntry(folder, partes)
	assert.Error(t, err)
}
