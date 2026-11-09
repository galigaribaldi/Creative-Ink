package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveDocumentCreatesDirsAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "nested", "main.tex")
	fm := NewFileManager()

	const content = "\\documentclass{article}\n\\begin{document}\nHola\n\\end{document}\n"
	if err := fm.SaveDocument(content, path); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	got, err := fm.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument: %v", err)
	}
	if got != content {
		t.Fatalf("LoadDocument = %q, want %q", got, content)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file must not remain after save, stat err = %v", err)
	}
}

func TestSaveDocumentOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.tex")
	fm := NewFileManager()

	for _, c := range []string{"first version, longer", "second"} {
		if err := fm.SaveDocument(c, path); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := fm.LoadDocument(path); got != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}
}

func TestLoadDocumentMissingFileIsEmpty(t *testing.T) {
	got, err := NewFileManager().LoadDocument(filepath.Join(t.TempDir(), "absent.tex"))
	if err != nil || got != "" {
		t.Fatalf("LoadDocument(missing) = %q, %v; want \"\", nil", got, err)
	}
}

// ── Archetype tests (testify) ──────────────────────────────────────────────

// TestLoadTextGuardsTheEditor: what the editor may open. Anything the IPC
// bridge could alter (binary, non-UTF-8, huge) is refused, so an autosave
// can never rewrite it mangled.
func TestLoadTextGuardsTheEditor(t *testing.T) {
	dir := t.TempDir()
	fm := NewFileManager()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, data, 0o644))
		return p
	}

	got, err := fm.LoadText(write("ok.bib", []byte("@book{clave, title={Año}}")))
	require.NoError(t, err)
	assert.Equal(t, "@book{clave, title={Año}}", got)

	_, err = fm.LoadText(write("latin1.bib", []byte{'a', 0xF1, 'o'}))
	assert.ErrorIs(t, err, ErrNotUTF8)
	_, err = fm.LoadText(write("binario.tex", []byte{'a', 0, 'b'}))
	assert.ErrorIs(t, err, ErrNotUTF8)
	_, err = fm.LoadText(write("enorme.tex", bytes.Repeat([]byte("x"), MaxEditableBytes+1)))
	assert.ErrorIs(t, err, ErrFileTooLarge)
	_, err = fm.LoadText(filepath.Join(dir, "no-existe.tex"))
	assert.Error(t, err)
}

// TestConcurrentSavesNeverTearAFile: autosaves from two panes, ⌘S and Save
// As can overlap. Writes are serialised and atomic: the file always holds
// one complete version and no .tmp is left behind.
func TestConcurrentSavesNeverTearAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capitulos", "cap1.tex")
	fm := NewFileManager()
	versions := make([]string, 20)
	for i := range versions {
		versions[i] = strings.Repeat(fmt.Sprintf("versión %02d ", i), 2000)
	}

	var wg sync.WaitGroup
	for _, v := range versions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, fm.SaveDocument(v, path))
		}()
	}
	wg.Wait()

	got, err := fm.LoadText(path)
	require.NoError(t, err)
	assert.Contains(t, versions, got, "the file must hold exactly one complete version")
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "no temp file may remain")
}
