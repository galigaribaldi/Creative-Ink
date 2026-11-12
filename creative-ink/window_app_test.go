package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two windows are two App values sharing the lock directory (in tests, the
// same process: flock still conflicts between separate open files).
func TestTheSameProjectCannotBeOpenInTwoWindows(t *testing.T) {
	locks := t.TempDir()
	a, _ := newRepoApp(t)
	b, _ := newRepoApp(t)
	a.lockDir, b.lockDir = locks, locks

	project := filepath.Join(t.TempDir(), "Tesis")
	writeFiles(t, project, map[string]string{"main.tex": importDoc})
	other := filepath.Join(t.TempDir(), "Articulo")
	writeFiles(t, other, map[string]string{"main.tex": importDoc})

	a.pickFolder = func(string) (string, error) { return project, nil }
	_, err := a.OpenFolderDialog()
	require.NoError(t, err)

	b.pickFolder = func(string) (string, error) { return project, nil }
	_, err = b.OpenFolderDialog()
	require.ErrorContains(t, err, "ya está abierto en otra ventana")
	folder, _ := b.workspace()
	assert.Empty(t, folder, "the second window changed nothing")

	b.pickFolder = func(string) (string, error) { return other, nil }
	_, err = b.OpenFolderDialog()
	require.NoError(t, err, "another project is fine")

	// The first window moves on: the project is free again.
	a.pickFolder = func(string) (string, error) { return other, nil }
	_, err = a.OpenFolderDialog()
	require.ErrorContains(t, err, "ya está abierto en otra ventana")
	a.releaseLocation()
	b.pickFolder = func(string) (string, error) { return project, nil }
	_, err = b.OpenFolderDialog()
	assert.NoError(t, err, "released on close (shutdown) or when switching folders")
}

func TestNewWindowStartsAnotherInstance(t *testing.T) {
	a, _ := newRepoApp(t)
	started := 0
	a.launchInstance = func() error { started++; return nil }
	if devBuild {
		assert.ErrorIs(t, a.NewWindow(), errNewWindowInDev)
		return
	}
	require.NoError(t, a.NewWindow())
	assert.Equal(t, 1, started)
}

func TestMacAppBundle(t *testing.T) {
	assert.Equal(t, filepath.FromSlash("/Applications/Creative-ink.app"),
		macAppBundle(filepath.FromSlash("/Applications/Creative-ink.app/Contents/MacOS/creative-ink")))
	assert.Empty(t, macAppBundle(filepath.FromSlash("/home/u/build/bin/creative-ink")))
}

func TestSaveNewDialogSavesTheUntitledDocumentInTheProject(t *testing.T) {
	a, _ := newTestApp(t, okCompile)
	folder, _ := a.workspace()
	a.pickSavePath = func(dir, name string) (string, error) {
		assert.Equal(t, folder, dir, "the dialog opens in the open project")
		assert.Equal(t, "sin-titulo.tex", name)
		return filepath.Join(dir, "Capítulo Dos"), nil
	}
	st, err := a.SaveNewDialog("\\documentclass{article}\nHola")
	require.NoError(t, err)
	want := filepath.Join(folder, "Capitulo-Dos.tex")
	assert.Equal(t, want, st.FilePath, "normalized with '-' and .tex added")
	data, err := os.ReadFile(want)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hola")
}
