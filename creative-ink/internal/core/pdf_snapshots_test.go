package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPDFSnapshotsAreImmutableAndBounded(t *testing.T) {
	s, err := NewPDFSnapshots(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "main.pdf")

	publish := func(content string) uint64 {
		t.Helper()
		if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		v, err := s.Publish(src)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	read := func(v uint64) string {
		t.Helper()
		path, ok := s.Path(v)
		if !ok {
			t.Fatalf("version %d not found", v)
		}
		data, _ := os.ReadFile(path)
		return string(data)
	}

	v1 := publish("version 1")
	// latexmk rewrites main.pdf in place: the snapshot must not change.
	if err := os.WriteFile(src, []byte("rewritten by latexmk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := read(v1); got != "version 1" {
		t.Fatalf("snapshot changed to %q", got)
	}

	v2 := publish("version 2")
	v3 := publish("version 3")
	if v1 >= v2 || v2 >= v3 {
		t.Fatalf("versions must increase: %d %d %d", v1, v2, v3)
	}
	if _, ok := s.Path(v1); ok {
		t.Fatal("only the newest `keep` snapshots must be kept")
	}
	if read(v2) != "version 2" || read(v3) != "version 3" {
		t.Fatal("kept snapshots must keep their content")
	}
	if v, _, ok := s.Latest(); !ok || v != v3 {
		t.Fatalf("Latest = %d, %v", v, ok)
	}

	entries, _ := os.ReadDir(s.dir)
	if len(entries) != 2 {
		t.Fatalf("files on disk = %d, want 2 (no leftovers or temp files)", len(entries))
	}
}

func TestPDFSnapshotsClose(t *testing.T) {
	s, err := NewPDFSnapshots(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "main.pdf")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(src); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Fatal("Close must delete the snapshots folder")
	}
	if _, err := s.Publish(src); !errors.Is(err, ErrSnapshotsClosed) {
		t.Fatalf("Publish after Close: err = %v", err)
	}
}

func TestPDFSnapshotsMissingSource(t *testing.T) {
	s, err := NewPDFSnapshots(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.Publish(filepath.Join(t.TempDir(), "missing.pdf")); err == nil {
		t.Fatal("publishing a missing PDF must fail")
	}
	if _, _, ok := s.Latest(); ok {
		t.Fatal("a failed publish must not register a version")
	}
}
