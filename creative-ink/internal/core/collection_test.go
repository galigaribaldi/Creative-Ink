package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootDocumentForFindsTheMainOfItsDocument(t *testing.T) {
	root := masterRepo(t) // Semestre/<Doc>/main.tex + secciones, a standalone figure
	reg := filepath.Join(root, "Semestre", "Registro1")
	main := filepath.Join(reg, "main.tex")

	assert.Equal(t, main, RootDocumentFor(root, filepath.Join(reg, "secciones", "intro.tex"), ""), "a section compiles its document")
	assert.Equal(t, main, RootDocumentFor(root, main, ""), "a document compiles itself")
	fig := filepath.Join(root, "Semestre", "Articulo", "img", "tikz", "fig1.tex")
	assert.Equal(t, fig, RootDocumentFor(root, fig, ""), "a standalone figure is a document of its own")

	// Without any document above, the file itself (as before collections).
	loose := filepath.Join(root, "suelto.tex")
	require.NoError(t, os.WriteFile(loose, []byte("texto"), 0o644))
	assert.Equal(t, loose, RootDocumentFor(root, loose, ""))
}

func TestRootDocumentForBreaksTiesWithTheLastOneOpened(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"tesis.tex": doc, "poster.tex": doc, "caps/cap1.tex": "Capítulo\n"})
	cap1 := filepath.Join(dir, "caps", "cap1.tex")
	assert.Equal(t, cap1, RootDocumentFor(dir, cap1, ""), "ambiguous: the user picks one by opening it or with ★")
	poster := filepath.Join(dir, "poster.tex")
	assert.Equal(t, poster, RootDocumentFor(dir, cap1, poster))
}

func TestDetectCollection(t *testing.T) {
	root := masterRepo(t)
	c := DetectCollection(root)
	assert.True(t, c.Collection)
	assert.Equal(t, 4, c.Documents)
	assert.False(t, DetectCollection(filepath.Join(root, "Semestre", "Registro1")).Collection, "a document folder")
	single := t.TempDir()
	writeTree(t, single, map[string]string{"tex/main.tex": doc})
	assert.False(t, DetectCollection(single).Collection, "one document in a subfolder is not a collection")

	loc := ClassifyLocation("", root)
	assert.True(t, loc.Collection)
	assert.Contains(t, noticeIDs(loc.Notices), NoticeCollection)
}

func TestReusableQuickDocOnlyReturnsUntouchedOnes(t *testing.T) {
	repo := t.TempDir()
	const starter = "\\documentclass{article}\n"
	dir1, _, err := CreateQuickDoc(repo, starter)
	require.NoError(t, err)
	got, _ := ReusableQuickDoc(repo, starter)
	assert.Equal(t, dir1, got)

	// Edited, or compiled (more files): never reused.
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "main.tex"), []byte(starter+"Hola"), 0o644))
	got, _ = ReusableQuickDoc(repo, starter)
	assert.Empty(t, got)
	dir2, _, err := CreateQuickDoc(repo, starter)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "main.pdf"), []byte("%PDF"), 0o644))
	got, _ = ReusableQuickDoc(repo, starter)
	assert.Empty(t, got)
}

func TestInspectAndCopyASingleFile(t *testing.T) {
	root := masterRepo(t)
	file := filepath.Join(root, "Semestre", "Presentacion", "main.tex")
	p, err := InspectFileImport("", file)
	require.NoError(t, err)
	assert.Equal(t, file, p.File)
	assert.Equal(t, root, p.GitRoot)
	assert.Equal(t, 1, p.Files)
	assert.Equal(t, []string{"main.tex"}, p.MainCandidates)
	assert.Equal(t, 2, p.ExternalRefCount, "secciones/intro and ../Ensayo/img/foto.png are not copied with the file")
	assert.Contains(t, noticeIDs(ImportNotices(p)), "import-git-part")

	repo := t.TempDir()
	dir, copied, err := CopyFileToQuickDoc(repo, p)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "main.tex"), copied)
	assert.FileExists(t, filepath.Join(dir, GitCopyNoteName))
	meta, err := LoadProjectMeta(dir)
	require.NoError(t, err)
	assert.Equal(t, "main.tex", meta.MainFile)
	require.NotNil(t, meta.Imported)
	assert.Equal(t, file, meta.Imported.From)

	_, err = InspectFileImport("", filepath.Join(root, "Semestre", "Registro1", "Registro1.pdf"))
	assert.ErrorIs(t, err, ErrNotEditable)
}

func TestInspectFileImportInABroadFolderOpensOnlyTheFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	file := filepath.Join(home, "Desktop", "nota.tex")
	writeTree(t, filepath.Dir(file), map[string]string{"nota.tex": doc})
	p, err := InspectFileImport("", file)
	require.NoError(t, err)
	assert.True(t, p.InPlaceSingle)
	assert.Contains(t, noticeIDs(ImportNotices(p)), "import-single")
}

func TestLockFileSerialisesAndTryLockReportsTheHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := LockFile(path)
	require.NoError(t, err)
	_, err = TryLockFile(path)
	assert.ErrorIs(t, err, ErrLocked, "held: another window is using it")

	acquired := make(chan struct{})
	go func() {
		u, err := LockFile(path)
		if err == nil {
			u()
		}
		close(acquired)
	}()
	select {
	case <-acquired:
		t.Fatal("LockFile must wait for the holder")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	<-acquired
	u, err := TryLockFile(path)
	require.NoError(t, err)
	u()
}

func TestUpdateSettingsLosesNoConcurrentChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := UpdateSettings(path, func(s *Settings) {
				s.RestrictedProjects = append(s.RestrictedProjects, filepath.Join("/p", string(rune('a'+i))))
			})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	s, err := ReadSettings(path)
	require.NoError(t, err)
	assert.Len(t, s.RestrictedProjects, 20, "every read-modify-write kept the others' changes")
}
