package core

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestOutputSpoolReadsInChunksAtRuneBoundaries(t *testing.T) {
	s, err := NewOutputSpool()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	id, w, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("página ñ ", 20000) // multi-byte runes everywhere
	_, _ = w.Write([]byte(text))

	var got strings.Builder
	var offset int64
	for {
		chunk, err := s.Read(id, offset, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Done {
			t.Fatal("the run is still open")
		}
		if strings.ContainsRune(chunk.Data, '�') {
			t.Fatal("a chunk split a UTF-8 character")
		}
		got.WriteString(chunk.Data)
		if chunk.Next == offset {
			break
		}
		offset = chunk.Next
	}
	if got.String() != text {
		t.Fatalf("read %d bytes, want %d", got.Len(), len(text))
	}
	w.Close()
	if chunk, _ := s.Read(id, offset, 0); !chunk.Done || chunk.Size != int64(len(text)) {
		t.Fatalf("after close = %+v", chunk)
	}
}

func TestOutputSpoolKeepsOnlyRecentRunsAndCleansUp(t *testing.T) {
	s, err := NewOutputSpool()
	if err != nil {
		t.Fatal(err)
	}
	first, w, _ := s.Start()
	w.Close()
	for i := 0; i < maxSpoolRuns; i++ {
		_, w, _ := s.Start()
		w.Close()
	}
	if _, err := s.Read(first, 0, 0); !errors.Is(err, ErrUnknownRun) {
		t.Fatalf("the oldest run must be gone: %v", err)
	}
	dir := s.dir
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("Close must delete the spool folder")
	}
}
