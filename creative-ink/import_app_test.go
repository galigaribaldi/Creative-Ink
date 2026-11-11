package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creative-ink/internal/core"
)

// writeFiles creates files (relative path → content) under root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

const importDoc = "\\documentclass{article}\n\\begin{document}\nHola\n\\end{document}\n"

// waitImportFinished returns the import:finished event (other events skipped).
func waitImportFinished(t *testing.T, events chan emittedEvent) ImportFinishedEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			if e.name == EventImportFinished {
				return e.data[0].(ImportFinishedEvent)
			}
		case <-deadline:
			t.Fatal("import:finished never arrived")
		}
	}
}

func TestImportADocumentOfAGitRepositoryAndOpenTheCopy(t *testing.T) {
	a, root := newRepoApp(t)
	events := make(chan emittedEvent, 64)
	a.emit = func(name string, data ...any) { events <- emittedEvent{name: name, data: data} }

	repo := filepath.Join(t.TempDir(), "Trabajos")
	writeFiles(t, repo, map[string]string{
		".git/HEAD":                   "ref: refs/heads/main\n",
		"Semestre/Registro1/main.tex": importDoc,
		"Semestre/Registro1/refs.bib": "@book{a,}\n",
		"Semestre/Registro1/main.aux": "aux\n",
		"Semestre/Articulo/main.tex":  importDoc,
	})
	src := filepath.Join(repo, "Semestre", "Registro1")
	a.pickFolder = func(string) (string, error) { return src, nil }

	p, err := a.InspectImport()
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.True(t, p.CanCopy)
	assert.False(t, p.HasGit)
	assert.Equal(t, repo, p.GitRoot)
	assert.Equal(t, 2, p.Files) // main.tex and refs.bib, not main.aux
	require.NotEmpty(t, p.Notices)
	assert.Equal(t, "import-git-part", p.Notices[0].ID)

	require.NoError(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Registro 1", MainFile: "main.tex"}))
	done := waitImportFinished(t, events)
	require.True(t, done.Success, done.Error)
	assert.Equal(t, filepath.Join(root, "Registro-1"), done.Folder)
	assert.FileExists(t, filepath.Join(done.Folder, "main.tex"))
	assert.FileExists(t, filepath.Join(done.Folder, core.GitCopyNoteName))
	assert.NoFileExists(t, filepath.Join(done.Folder, "main.aux"))
	assert.FileExists(t, filepath.Join(src, "main.aux"), "the original is never modified")
	assert.NoDirExists(t, core.ImportStagingDir(root, p.ID))

	// The UI opens the copy: a project of the repository, reminded as a git copy.
	st, err := a.OpenProject(done.Folder)
	require.NoError(t, err)
	assert.Equal(t, core.LocationRepository, st.Location.Kind)
	require.Len(t, st.Location.Notices, 1)
	assert.Equal(t, core.NoticeGitCopy, st.Location.Notices[0].ID)
	assert.Equal(t, filepath.Join(done.Folder, "main.tex"), st.MainFilePath)

	// The preview was used: importing it again needs a new analysis.
	assert.ErrorIs(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Otra"}), errImportUnknown)
}

func TestImportACollectionOnlyOpensInPlace(t *testing.T) {
	a, _ := newRepoApp(t)
	repo := filepath.Join(t.TempDir(), "Trabajos")
	writeFiles(t, repo, map[string]string{
		".git/HEAD":      "ref: refs/heads/main\n",
		"A/Uno/main.tex": importDoc,
		"B/Dos/main.tex": importDoc,
		"README.md":      "# Trabajos\n",
	})
	a.pickFolder = func(string) (string, error) { return repo, nil }

	p, err := a.InspectImport()
	require.NoError(t, err)
	assert.True(t, p.Collection)
	assert.False(t, p.CanCopy)
	assert.Error(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Trabajos"}), "a collection is never copied as a whole")

	st, err := a.OpenImportInPlace(p.ID)
	require.NoError(t, err)
	assert.Equal(t, repo, st.FolderPath)
	assert.Equal(t, core.LocationExternal, st.Location.Kind)
	assert.Equal(t, core.NoticeExternalGit, st.Location.Notices[0].ID)
}

func TestImportRejectsBadChoices(t *testing.T) {
	a, root := newRepoApp(t)
	src := filepath.Join(t.TempDir(), "Tesis")
	writeFiles(t, src, map[string]string{"main.tex": importDoc})
	a.pickFolder = func(string) (string, error) { return src, nil }
	p, err := a.InspectImport()
	require.NoError(t, err)

	assert.Error(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Tesis", MainFile: "../fuera.tex"}), "only listed documents")
	require.NoError(t, os.Mkdir(filepath.Join(root, "Tesis"), 0o755))
	assert.ErrorIs(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Tesis"}), core.ErrProjectExists)
	assert.ErrorIs(t, a.ImportProject(ImportOptions{ID: "no-existe", Name: "X"}), errImportUnknown)

	// Not enough disk space: blocked before copying.
	a.freeBytes = func(string) (uint64, error) { return 1024, nil }
	assert.ErrorContains(t, a.ImportProject(ImportOptions{ID: p.ID, Name: "Tesis-2"}), "no hay espacio")

	// The dialog closed: nothing happens.
	a.pickFolder = func(string) (string, error) { return "", nil }
	p, err = a.InspectImport()
	require.NoError(t, err)
	assert.True(t, p.Canceled)

	// The repository itself and broad folders are refused.
	a.pickFolder = func(string) (string, error) { return root, nil }
	_, err = a.InspectImport()
	assert.ErrorIs(t, err, core.ErrBroadFolder)
}
