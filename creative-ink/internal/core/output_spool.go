package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Terminal output spool (docs/MAP.md §4.3.4, §4.3.6): the full output of
// every terminal run is written to a file in a private temp folder while it
// runs, instead of being held in memory. The UI reads it on demand, in
// chunks (ReadTerminalOutput), only when the user opens "Salida completa".
const (
	// maxSpoolRuns: older runs' files are deleted.
	maxSpoolRuns = 8
	// maxSpoolBytes caps one run on disk; beyond it the output is dropped
	// (the terminal still gets its summary).
	maxSpoolBytes = 64 << 20
	// MaxOutputChunk bounds one ReadTerminalOutput answer (one IPC message).
	MaxOutputChunk = 64 * 1024
)

// ErrUnknownRun is returned for a run id the spool no longer has.
var ErrUnknownRun = errors.New("esa salida ya no está disponible")

// OutputSpool stores the output of terminal runs on disk.
type OutputSpool struct {
	dir string

	mu    sync.Mutex
	next  int
	runs  map[int]*SpoolWriter
	order []int // oldest first
}

// OutputChunk is a piece of a run's output.
type OutputChunk struct {
	RunID  int    `json:"runId"`
	Data   string `json:"data"`
	Offset int64  `json:"offset"` // where Data starts
	Next   int64  `json:"next"`   // offset for the next read
	Size   int64  `json:"size"`   // bytes written so far
	Done   bool   `json:"done"`   // the run ended: Size is final
	// Dropped counts bytes over maxSpoolBytes that were not stored.
	Dropped int64 `json:"dropped"`
}

// NewOutputSpool creates the spool in a private temp folder.
func NewOutputSpool() (*OutputSpool, error) {
	dir, err := os.MkdirTemp("", "creative-ink-terminal-")
	if err != nil {
		return nil, fmt.Errorf("create terminal spool: %w", err)
	}
	return &OutputSpool{dir: dir, runs: map[int]*SpoolWriter{}}, nil
}

// Start creates the file of a new run. Close the writer when the run ends.
func (s *OutputSpool) Start() (int, *SpoolWriter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := s.next
	path := filepath.Join(s.dir, "run-"+strconv.Itoa(id)+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, nil, fmt.Errorf("create %q: %w", path, err)
	}
	w := &SpoolWriter{path: path, file: f}
	s.runs[id] = w
	s.order = append(s.order, id)
	for len(s.order) > maxSpoolRuns {
		old := s.order[0]
		s.order = s.order[1:]
		if r := s.runs[old]; r != nil {
			r.Close()
			_ = os.Remove(r.path)
		}
		delete(s.runs, old)
	}
	return id, w, nil
}

// Read returns up to max bytes (≤ MaxOutputChunk) of run id from offset,
// cut at a UTF-8 boundary so the IPC never splits a character.
func (s *OutputSpool) Read(id int, offset int64, max int) (OutputChunk, error) {
	s.mu.Lock()
	w := s.runs[id]
	s.mu.Unlock()
	if w == nil {
		return OutputChunk{}, ErrUnknownRun
	}
	size, done, dropped := w.state()
	chunk := OutputChunk{RunID: id, Offset: offset, Next: offset, Size: size, Done: done, Dropped: dropped}
	if max <= 0 || max > MaxOutputChunk {
		max = MaxOutputChunk
	}
	if offset < 0 || offset >= size {
		chunk.Offset, chunk.Next = min(max64(offset, 0), size), min(max64(offset, 0), size)
		return chunk, nil
	}
	f, err := os.Open(w.path)
	if err != nil {
		return OutputChunk{}, ErrUnknownRun
	}
	defer f.Close()
	buf := make([]byte, min(int64(max), size-offset))
	n, _ := f.ReadAt(buf, offset)
	buf = buf[:n]
	if offset+int64(n) < size { // more follows: do not cut a rune in half
		buf = trimPartialRune(buf)
	}
	chunk.Data = strings.ToValidUTF8(string(buf), "�")
	chunk.Next = offset + int64(len(buf))
	return chunk, nil
}

// Close deletes every stored run.
func (s *OutputSpool) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.runs {
		w.Close()
	}
	s.runs = map[int]*SpoolWriter{}
	s.order = nil
	return os.RemoveAll(s.dir)
}

// SpoolWriter is the file of one run. Safe for concurrent use.
type SpoolWriter struct {
	path string

	mu      sync.Mutex
	file    *os.File
	size    int64
	dropped int64
	done    bool
}

// Write never fails: a full disk must not break the command.
func (w *SpoolWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done || w.file == nil {
		return len(p), nil
	}
	keep := min(int64(len(p)), maxSpoolBytes-w.size)
	if keep > 0 {
		n, _ := w.file.Write(p[:keep])
		w.size += int64(n)
	}
	w.dropped += int64(len(p)) - max64(keep, 0)
	return len(p), nil
}

// Close marks the run as finished (its size is final).
func (w *SpoolWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.done = true
}

func (w *SpoolWriter) state() (size int64, done bool, dropped int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size, w.done, w.dropped
}

// trimPartialRune drops an incomplete UTF-8 sequence at the end of b.
func trimPartialRune(b []byte) []byte {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if !utf8.FullRune(b[i:]) {
				return b[:i]
			}
			break
		}
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
