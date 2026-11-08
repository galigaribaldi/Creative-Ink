package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// ErrSnapshotsClosed is returned by Publish after Close.
var ErrSnapshotsClosed = errors.New("las instantáneas de la vista previa están cerradas")

// PDFSnapshots keeps immutable, versioned copies of compiled PDFs for the
// preview.
//
// Why: PDF.js loads PDFs lazily with HTTP Range requests, fetching chunks
// while it renders, possibly long after the compilation ended. latexmk
// rewrites <doc>.pdf in place on the next run, so reading ranges from it
// could mix bytes of two versions (corrupt pages). A snapshot never changes:
// a document always reads the version it was opened with.
type PDFSnapshots struct {
	dir  string
	keep int // newest snapshots kept on disk; older ones are deleted

	mu     sync.Mutex
	next   uint64
	order  []uint64 // published versions, oldest first
	closed bool
}

// NewPDFSnapshots stores snapshots in a fresh private folder inside parent
// (the OS temp dir if ""), so several app instances never collide and the
// OS eventually removes leftovers after a crash.
func NewPDFSnapshots(parent string, keep int) (*PDFSnapshots, error) {
	if keep < 1 {
		keep = 1
	}
	dir, err := os.MkdirTemp(parent, "creative-ink-preview-*")
	if err != nil {
		return nil, fmt.Errorf("create preview snapshots dir: %w", err)
	}
	return &PDFSnapshots{dir: dir, keep: keep}, nil
}

// Publish copies src into a new snapshot and returns its version (>= 1).
// The copy streams through a small buffer (no RAM spike for big PDFs) and
// is renamed into place, so a snapshot is never visible half-written.
func (s *PDFSnapshots) Publish(src string) (uint64, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSnapshotsClosed
	}
	s.next++
	version := s.next
	s.mu.Unlock()

	final := s.path(version)
	if err := copyFile(src, final); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = os.Remove(final)
		return 0, ErrSnapshotsClosed
	}
	s.order = append(s.order, version)
	for len(s.order) > s.keep {
		_ = os.Remove(s.path(s.order[0]))
		s.order = s.order[1:]
	}
	return version, nil
}

// Path returns the file of a published snapshot that is still kept.
func (s *PDFSnapshots) Path(version uint64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.order {
		if v == version {
			return s.path(v), true
		}
	}
	return "", false
}

// Latest returns the newest snapshot, if any.
func (s *PDFSnapshots) Latest() (uint64, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) == 0 {
		return 0, "", false
	}
	v := s.order[len(s.order)-1]
	return v, s.path(v), true
}

// Close deletes every snapshot. Later Publish calls fail.
func (s *PDFSnapshots) Close() error {
	s.mu.Lock()
	s.closed = true
	s.order = nil
	s.mu.Unlock()
	return os.RemoveAll(s.dir)
}

func (s *PDFSnapshots) path(version uint64) string {
	return filepath.Join(s.dir, strconv.FormatUint(version, 10)+".pdf")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %q: %w", src, err)
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %q: %w", tmp, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy %q: %w", src, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("publish %q: %w", dst, err)
	}
	return nil
}
