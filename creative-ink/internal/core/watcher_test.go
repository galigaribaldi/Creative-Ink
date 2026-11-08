package core

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const testDebounce = 80 * time.Millisecond

// startWatcher watches dir and counts onChange calls.
func startWatcher(t *testing.T, dir string) (*atomic.Int32, chan struct{}) {
	t.Helper()
	var calls atomic.Int32
	changed := make(chan struct{}, 16)
	fw, err := WatchFolder(dir, testDebounce, func() {
		calls.Add(1)
		changed <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fw.Close() })
	return &calls, changed
}

func waitChange(t *testing.T, changed chan struct{}) {
	t.Helper()
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for onChange")
	}
}

func expectQuiet(t *testing.T, changed chan struct{}) {
	t.Helper()
	select {
	case <-changed:
		t.Fatal("unexpected onChange")
	case <-time.After(4 * testDebounce):
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherReportsCreateRemoveRename(t *testing.T) {
	dir := t.TempDir()
	_, changed := startWatcher(t, dir)

	img := filepath.Join(dir, "figura.png")
	write(t, img) // e.g. an image pasted to use in LaTeX
	waitChange(t, changed)

	if err := os.Rename(img, filepath.Join(dir, "fig1.png")); err != nil {
		t.Fatal(err)
	}
	waitChange(t, changed)

	if err := os.Remove(filepath.Join(dir, "fig1.png")); err != nil {
		t.Fatal(err)
	}
	waitChange(t, changed)
}

func TestWatcherDebouncesBursts(t *testing.T) {
	dir := t.TempDir()
	calls, changed := startWatcher(t, dir)

	for i := 0; i < 20; i++ {
		write(t, filepath.Join(dir, "f"+string(rune('a'+i))+".tex"))
	}
	waitChange(t, changed)
	expectQuiet(t, changed)
	if got := calls.Load(); got != 1 {
		t.Fatalf("onChange calls = %d, want 1 for a burst", got)
	}
}

func TestWatcherIgnoresNoise(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.tex"))
	_, changed := startWatcher(t, dir)

	// Autosave writes and latexmk by-products must not refresh the tree.
	write(t, filepath.Join(dir, "main.tex"))
	for _, n := range []string{"main.aux", "main.log", "main.fls", "main.tex.tmp", ".creative-ink-meta"} {
		write(t, filepath.Join(dir, n))
	}
	expectQuiet(t, changed)
}

func TestWatcherFollowsNewSubfolders(t *testing.T) {
	dir := t.TempDir()
	_, changed := startWatcher(t, dir)

	sub := filepath.Join(dir, "capitulos")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	waitChange(t, changed)

	// fsnotify is not recursive: the new folder must have been added.
	write(t, filepath.Join(sub, "intro.tex"))
	waitChange(t, changed)
}

func TestWatcherCloseIsIdempotent(t *testing.T) {
	fw, err := WatchFolder(t.TempDir(), testDebounce, func() {})
	if err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = fw.Close()
}

func TestTreeSignature(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, "main.tex")
	tree1, tr1, _ := ScanTree(dir)
	same, trSame, _ := ScanTree(dir)
	if TreeSignature(tree1, tr1) != TreeSignature(same, trSame) {
		t.Fatal("same tree must give the same signature")
	}

	write(t, filepath.Join(dir, "main.aux")) // hidden from the tree
	tree2, tr2, _ := ScanTree(dir)
	if TreeSignature(tree1, tr1) != TreeSignature(tree2, tr2) {
		t.Fatal("a build artifact must not change the signature")
	}

	mkfiles(t, dir, "fig.png")
	tree3, tr3, _ := ScanTree(dir)
	if TreeSignature(tree1, tr1) == TreeSignature(tree3, tr3) {
		t.Fatal("a new visible file must change the signature")
	}
}
