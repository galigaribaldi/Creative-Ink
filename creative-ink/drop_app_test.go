package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creative-ink/internal/core"
)

const dropDoc = "\\documentclass{article}\n\\begin{document}\n\\input{secciones/intro}\n\\end{document}\n"

// collectionRepo: a git repository with two documents (Trabajos-Maestria-Urbanismo in small).
func collectionRepo(t *testing.T) string {
	repo := filepath.Join(t.TempDir(), "Trabajos")
	writeFiles(t, repo, map[string]string{
		".git/HEAD":                              "ref: refs/heads/main\n",
		"Semestre/Registro1/main.tex":            dropDoc,
		"Semestre/Registro1/secciones/intro.tex": "Introducción\n",
		"Semestre/Ensayo/main.tex":               dropDoc,
		"README.md":                              "# Trabajos\n",
	})
	return repo
}

func TestCollectionCompilesTheDocumentOfTheOpenFile(t *testing.T) {
	a, _ := newRepoApp(t)
	events := make(chan emittedEvent, 64)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }
	repo := collectionRepo(t)
	a.pickFolder = func(string) (string, error) { return repo, nil }
	st, err := a.OpenFolderDialog()
	require.NoError(t, err)
	assert.True(t, st.Location.Collection)
	assert.Equal(t, 2, st.Location.Documents)
	assert.Contains(t, noticeIDsOf(st.Location.Notices), core.NoticeCollection)
	assert.Empty(t, st.CompileTarget, "no document at the root")

	section := filepath.Join(repo, "Semestre", "Registro1", "secciones", "intro.tex")
	opened, err := a.OpenFile(section)
	require.NoError(t, err)
	main := filepath.Join(repo, "Semestre", "Registro1", "main.tex")
	assert.Equal(t, main, opened.CompileTarget, "a section compiles its document's main.tex, never itself")
	require.True(t, a.RequestCompile("manual"))
	expectCompileOf(t, events, main)

	// In a collection there is no single main file (it would write into the repository).
	_, err = a.SetMainFile(main)
	assert.ErrorIs(t, err, errCollectionMain)
	assert.NoFileExists(t, filepath.Join(repo, core.MetaFileName))
}

func TestClassifyDrop(t *testing.T) {
	a, root := newRepoApp(t)
	repo := collectionRepo(t)
	a.pickFolder = func(string) (string, error) { return repo, nil }
	_, err := a.OpenFolderDialog()
	require.NoError(t, err)

	// The app's own drags arrive on macOS with an empty path: ignored.
	_, ok := a.classifyDrop(0, 0, []string{""})
	assert.False(t, ok)

	inside := filepath.Join(repo, "README.md")
	ev, ok := a.classifyDrop(10, 20, []string{inside, filepath.Join(repo, "x")})
	require.True(t, ok)
	assert.Equal(t, DropOpen, ev.Kind)
	assert.Equal(t, inside, ev.Path)
	assert.Equal(t, 1, ev.Extra, "one at a time")
	assert.Equal(t, 10, ev.X)

	outside := filepath.Join(t.TempDir(), "Tesis")
	writeFiles(t, outside, map[string]string{"main.tex": dropDoc, "foto.png": "png"})
	ev, _ = a.classifyDrop(0, 0, []string{outside})
	assert.Equal(t, DropChoose, ev.Kind)
	assert.True(t, ev.IsDir)
	assert.Empty(t, ev.Path, "the path stays in Go")
	p, err := a.InspectDrop(ev.ID)
	require.NoError(t, err)
	assert.Equal(t, outside, p.Source)
	assert.True(t, p.CanCopy)

	ev, _ = a.classifyDrop(0, 0, []string{filepath.Join(outside, "foto.png")})
	assert.Equal(t, DropRejected, ev.Kind, "not a text file")
	ev, _ = a.classifyDrop(0, 0, []string{repo})
	assert.Equal(t, DropRejected, ev.Kind, "already open")
	ev, _ = a.classifyDrop(0, 0, []string{root})
	assert.Equal(t, DropRejected, ev.Kind, "the launcher is not a project")

	// A project of the repository is opened directly (no question).
	project := filepath.Join(root, "Tesis")
	writeFiles(t, project, map[string]string{"main.tex": dropDoc, "capitulos/c1.tex": "x"})
	ev, _ = a.classifyDrop(0, 0, []string{filepath.Join(project, "capitulos")})
	assert.Equal(t, DropInPlace, ev.Kind)
	st, err := a.OpenImportInPlace(ev.ID)
	require.NoError(t, err)
	assert.Equal(t, project, st.FolderPath, "a subfolder opens its project")
}

func TestDroppedLooseFileOfDesktopOpensAloneOrCopies(t *testing.T) {
	a, root := newRepoApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	desktop := filepath.Join(home, "Desktop")
	writeFiles(t, desktop, map[string]string{"nota.tex": dropDoc, "otro.tex": "x", "foto.png": "png"})
	note := filepath.Join(desktop, "nota.tex")

	ev, _ := a.classifyDrop(0, 0, []string{note})
	require.Equal(t, DropChoose, ev.Kind)
	p, err := a.InspectDrop(ev.ID)
	require.NoError(t, err)
	assert.True(t, p.InPlaceSingle)

	// Open in place: only that file (single-file mode).
	st, err := a.OpenImportInPlace(ev.ID)
	require.NoError(t, err)
	assert.Equal(t, note, st.SingleFile)
	assert.Equal(t, desktop, st.FolderPath)
	require.Len(t, st.Tree.Children, 1, "the rest of Desktop is not shown")
	assert.Equal(t, note, st.Tree.Children[0].Path)
	assert.Equal(t, core.NoticeSingleFile, st.Location.Notices[0].ID)
	assert.Equal(t, note, st.CompileTarget)
	_, err = a.OpenFile(filepath.Join(desktop, "otro.tex"))
	assert.ErrorIs(t, err, errSingleFile)
	_, err = a.SaveContent(note, dropDoc+"%")
	assert.NoError(t, err, "the file itself is saved normally")
	_, err = a.NewFileDialog()
	assert.ErrorIs(t, err, errSingleFile)
	_, err = a.SetMainFile(note)
	assert.ErrorIs(t, err, errSingleFile)
	_, err = a.SetConfig("compile.auto", "on", "project")
	assert.ErrorIs(t, err, errSingleFile)
	assert.NoFileExists(t, filepath.Join(desktop, core.MetaFileName))

	// Copy: a new quick document with the file (the original stays).
	events := make(chan emittedEvent, 16)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }
	ev, _ = a.classifyDrop(0, 0, []string{note})
	_, err = a.InspectDrop(ev.ID)
	require.NoError(t, err)
	require.NoError(t, a.ImportProject(ImportOptions{ID: ev.ID}))
	done := waitImportFinished(t, events)
	require.True(t, done.Success, done.Error)
	assert.True(t, isInsideDir(filepath.Join(root, core.QuickDocsDirName), done.Folder))
	assert.FileExists(t, filepath.Join(done.Folder, "nota.tex"))
	assert.FileExists(t, note)
	st, err = a.OpenProject(done.Folder)
	require.NoError(t, err)
	assert.Empty(t, st.SingleFile, "a normal quick document now")
	assert.Equal(t, filepath.Join(done.Folder, "nota.tex"), st.MainFilePath)
}

func TestOpenBlankDocumentReusesAnUntouchedQuickDocument(t *testing.T) {
	a, root := newRepoApp(t)
	first, err := a.OpenBlankDocument()
	require.NoError(t, err)
	second, err := a.OpenBlankDocument()
	require.NoError(t, err)
	assert.Equal(t, first.FolderPath, second.FolderPath, "closing the start screen twice does not create two")

	require.NoError(t, os.WriteFile(second.FilePath, []byte("editado"), 0o644))
	third, err := a.OpenBlankDocument()
	require.NoError(t, err)
	assert.NotEqual(t, second.FolderPath, third.FolderPath, "an edited one is never reused")
	assert.True(t, isInsideDir(filepath.Join(root, core.QuickDocsDirName), third.FolderPath))
}

func TestCleanStaleImportsKeepsCopiesInProgress(t *testing.T) {
	a, root := newRepoApp(t)
	busy := core.ImportStagingDir(root, "otra-ventana")
	stale := core.ImportStagingDir(root, "abandonada")
	require.NoError(t, os.MkdirAll(busy, 0o755))
	require.NoError(t, os.MkdirAll(stale, 0o755))
	unlock, err := core.LockFile(busy + ".lock") // another window copying
	require.NoError(t, err)
	defer unlock()

	a.cleanStaleImports(root)
	assert.DirExists(t, busy)
	assert.NoDirExists(t, stale)
}

func noticeIDsOf(notices []core.LocationNotice) []string {
	ids := []string{}
	for _, n := range notices {
		ids = append(ids, n.ID)
	}
	return ids
}
