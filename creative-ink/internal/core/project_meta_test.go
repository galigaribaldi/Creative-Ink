package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectMetaRoundTrip(t *testing.T) {
	folder := t.TempDir()
	mkfiles(t, folder, "src/main.tex", "chapters/intro.tex")

	if meta, err := LoadProjectMeta(folder); err != nil || meta.MainFile != "" {
		t.Fatalf("missing meta = %+v, %v; want zero value", meta, err)
	}

	rel, err := RelativeMainFile(folder, filepath.Join(folder, "src", "main.tex"))
	if err != nil || rel != "src/main.tex" {
		t.Fatalf("RelativeMainFile = %q, %v (must use forward slashes)", rel, err)
	}
	if err := SaveProjectMeta(folder, ProjectMeta{MainFile: rel}); err != nil {
		t.Fatal(err)
	}
	meta, err := LoadProjectMeta(folder)
	if err != nil || meta.MainFile != "src/main.tex" || meta.Version != projectMetaVersion {
		t.Fatalf("LoadProjectMeta = %+v, %v", meta, err)
	}
	if got := meta.MainFilePath(folder); got != filepath.Join(folder, "src", "main.tex") {
		t.Fatalf("MainFilePath = %q", got)
	}

	// The meta file is hidden from the explorer.
	tree, _, _ := ScanTree(folder)
	for _, n := range tree.Children {
		if n.Name == MetaFileName {
			t.Fatal(".creative-ink-meta must not appear in the tree")
		}
	}
}

func TestProjectMetaMainFilePathRejectsInvalid(t *testing.T) {
	folder := t.TempDir()
	mkfiles(t, folder, "notes.txt")
	outside := filepath.Join(t.TempDir(), "x.tex")
	mkfiles(t, filepath.Dir(outside), "x.tex")

	for _, rel := range []string{"", "missing.tex", "notes.txt", "../" + filepath.Base(filepath.Dir(outside)) + "/x.tex", outside} {
		if got := (ProjectMeta{MainFile: rel}).MainFilePath(folder); got != "" {
			t.Errorf("MainFile %q resolved to %q, want \"\"", rel, got)
		}
	}
}

func TestLoadProjectMetaCorrupt(t *testing.T) {
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, MetaFileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProjectMeta(folder); err == nil {
		t.Fatal("a corrupt meta file must be reported")
	}
}
