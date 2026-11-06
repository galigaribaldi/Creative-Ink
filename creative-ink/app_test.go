package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"creative-ink/internal/compiler"
	"creative-ink/internal/core"
)

// emittedEvent is one call to App.emit.
type emittedEvent struct {
	name string
	data []any
}

// newTestApp builds an App whose compiler is a controllable fake and whose
// emitter records events instead of calling the Wails runtime.
func newTestApp(t *testing.T, compile compiler.CompileFunc) (*App, chan emittedEvent) {
	t.Helper()
	events := make(chan emittedEvent, 16)
	a := &App{
		files:   core.NewFileManager(),
		docPath: filepath.Join(t.TempDir(), "workspace", "main.tex"),
	}
	var mu sync.Mutex
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		events <- emittedEvent{name: name, data: data}
	}
	a.compiles = compiler.NewScheduler(compile, compiler.Hooks{
		OnStarted:  func() { a.emit(EventCompileStarted) },
		OnFinished: func(r compiler.Result) { a.emit(EventCompileFinished, r) },
	})
	t.Cleanup(func() { a.compiles.Close(time.Second) })
	return a, events
}

func nextEvent(t *testing.T, events chan emittedEvent) emittedEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an event")
		return emittedEvent{}
	}
}

func TestSaveContentReturnsBeforeCompilationEnds(t *testing.T) {
	release := make(chan struct{})
	a, events := newTestApp(t, func(ctx context.Context) (string, error) {
		<-release // a slow latexmk
		return "Compilación exitosa: main.pdf", nil
	})

	done := make(chan SaveResult, 1)
	go func() {
		res, err := a.SaveContent("\\documentclass{article}")
		if err != nil {
			t.Errorf("SaveContent: %v", err)
		}
		done <- res
	}()

	select {
	case res := <-done:
		if !res.Saved {
			t.Fatalf("SaveResult = %+v, want Saved=true", res)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SaveContent blocked on the compilation")
	}

	// The file is on disk before compiling.
	if data, err := os.ReadFile(a.docPath); err != nil || string(data) != "\\documentclass{article}" {
		t.Fatalf("saved file = %q, %v", data, err)
	}

	if ev := nextEvent(t, events); ev.name != EventCompileStarted || len(ev.data) != 0 {
		t.Fatalf("first event = %+v, want %s without payload", ev, EventCompileStarted)
	}
	close(release)
	ev := nextEvent(t, events)
	if ev.name != EventCompileFinished || len(ev.data) != 1 {
		t.Fatalf("second event = %+v, want %s with a payload", ev, EventCompileFinished)
	}
	if r, ok := ev.data[0].(compiler.Result); !ok || !r.Success {
		t.Fatalf("payload = %#v, want a successful compiler.Result", ev.data[0])
	}
}

func TestSaveContentDoesNotCompileWhenSaveFails(t *testing.T) {
	a, events := newTestApp(t, func(ctx context.Context) (string, error) {
		t.Error("must not compile when the save failed")
		return "", nil
	})
	// A directory where the file should be makes the atomic rename fail.
	if err := os.MkdirAll(a.docPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := a.SaveContent("x"); err == nil {
		t.Fatal("SaveContent must return the I/O error")
	}
	select {
	case ev := <-events:
		t.Fatalf("no event expected, got %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEmitWailsIsNoOpBeforeStartup(t *testing.T) {
	// runtime.EventsEmit calls log.Fatalf with a nil context; this must not.
	(&App{}).emitWails(EventCompileStarted)
}

func TestRequestCompileQueuesWithoutSaving(t *testing.T) {
	a, events := newTestApp(t, func(ctx context.Context) (string, error) { return "ok", nil })

	if !a.RequestCompile() {
		t.Fatal("RequestCompile must be accepted while the app is running")
	}
	if ev := nextEvent(t, events); ev.name != EventCompileStarted {
		t.Fatalf("first event = %+v", ev)
	}
	if ev := nextEvent(t, events); ev.name != EventCompileFinished {
		t.Fatalf("second event = %+v", ev)
	}
	if _, err := os.Stat(a.docPath); !os.IsNotExist(err) {
		t.Fatalf("RequestCompile must not write the document, stat err = %v", err)
	}

	a.compiles.Close(time.Second)
	if a.RequestCompile() {
		t.Fatal("RequestCompile after shutdown must report false")
	}
}
