package compiler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recorder captures hook calls in order.
type recorder struct {
	mu       sync.Mutex
	events   []string
	results  []Result
	finished chan Result
}

func newRecorder() *recorder {
	return &recorder{finished: make(chan Result, 16)}
}

func (r *recorder) hooks() Hooks {
	return Hooks{
		OnStarted: func() {
			r.mu.Lock()
			r.events = append(r.events, "started")
			r.mu.Unlock()
		},
		OnFinished: func(res Result) {
			r.mu.Lock()
			r.events = append(r.events, "finished")
			r.results = append(r.results, res)
			r.mu.Unlock()
			r.finished <- res
		},
	}
}

func (r *recorder) waitFinished(t *testing.T) Result {
	t.Helper()
	select {
	case res := <-r.finished:
		return res
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
		return Result{}
	}
}

func TestSchedulerRequestReturnsImmediately(t *testing.T) {
	release := make(chan struct{})
	rec := newRecorder()
	s := NewScheduler(func(ctx context.Context) (string, error) {
		<-release
		return "ok", nil
	}, rec.hooks())
	defer s.Close(time.Second)

	start := time.Now()
	if !s.Request() {
		t.Fatal("Request on an open scheduler must be accepted")
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("Request blocked for %s", elapsed)
	}

	close(release)
	if res := rec.waitFinished(t); !res.Success || res.Logs != "ok" {
		t.Fatalf("result = %+v", res)
	}
}

func TestSchedulerCoalescesBurstsAndNeverOverlaps(t *testing.T) {
	var runs, active, maxActive atomic.Int32
	release := make(chan struct{})
	firstStarted := make(chan struct{})
	rec := newRecorder()

	s := NewScheduler(func(ctx context.Context) (string, error) {
		n := runs.Add(1)
		if cur := active.Add(1); cur > maxActive.Load() {
			maxActive.Store(cur)
		}
		defer active.Add(-1)
		if n == 1 {
			close(firstStarted)
			<-release // hold the first run while the burst arrives
		}
		return "", nil
	}, rec.hooks())
	defer s.Close(time.Second)

	s.Request()
	<-firstStarted
	// Three quick saves while compiling → a single follow-up run.
	s.Request()
	s.Request()
	s.Request()
	close(release)

	rec.waitFinished(t)
	rec.waitFinished(t)
	select {
	case res := <-rec.finished:
		t.Fatalf("unexpected third run: %+v", res)
	case <-time.After(100 * time.Millisecond):
	}

	if got := runs.Load(); got != 2 {
		t.Fatalf("runs = %d, want 2 (1 running + 1 coalesced)", got)
	}
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent compilations = %d, want 1", got)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	want := []string{"started", "finished", "started", "finished"}
	for i, ev := range want {
		if i >= len(rec.events) || rec.events[i] != ev {
			t.Fatalf("events = %v, want %v", rec.events, want)
		}
	}
}

func TestSchedulerReportsFailures(t *testing.T) {
	rec := newRecorder()
	s := NewScheduler(func(ctx context.Context) (string, error) {
		return "! Undefined control sequence.\nl.4 \\foo", errors.New("latexmk falló")
	}, rec.hooks())
	defer s.Close(time.Second)

	s.Request()
	res := rec.waitFinished(t)
	if res.Success {
		t.Fatal("a failed compilation must report Success=false")
	}
	if want := "latexmk falló\n\n! Undefined control sequence.\nl.4 \\foo"; res.Logs != want {
		t.Fatalf("logs = %q, want %q", res.Logs, want)
	}
}

func TestSchedulerRecoversFromPanic(t *testing.T) {
	rec := newRecorder()
	var calls atomic.Int32
	s := NewScheduler(func(ctx context.Context) (string, error) {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return "ok", nil
	}, rec.hooks())
	defer s.Close(time.Second)

	s.Request()
	if res := rec.waitFinished(t); res.Success {
		t.Fatalf("a panic must be reported as a failure, got %+v", res)
	}
	// The scheduler must still work after the panic.
	s.Request()
	if res := rec.waitFinished(t); !res.Success {
		t.Fatalf("scheduler unusable after panic: %+v", res)
	}
}

func TestSchedulerCloseCancelsAndRejects(t *testing.T) {
	rec := newRecorder()
	started := make(chan struct{})
	cancelled := make(chan struct{})
	s := NewScheduler(func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done() // like latexmk being killed by CommandContext
		close(cancelled)
		return "", ctx.Err()
	}, rec.hooks())

	s.Request()
	<-started
	s.Close(time.Second)

	select {
	case <-cancelled:
	default:
		t.Fatal("Close must cancel the running compilation before returning")
	}
	if s.Request() {
		t.Fatal("Request after Close must be rejected")
	}
	select {
	case res := <-rec.finished:
		t.Fatalf("no compile:finished must be emitted after Close, got %+v", res)
	case <-time.After(50 * time.Millisecond):
	}
}
