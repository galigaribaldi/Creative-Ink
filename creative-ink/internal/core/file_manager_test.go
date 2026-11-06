package core

import (
	"os"
	"path/filepath"
	"testing"
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
