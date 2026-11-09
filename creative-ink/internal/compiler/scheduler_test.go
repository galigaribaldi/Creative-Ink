package compiler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	if !s.Request(TriggerManual) {
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

	s.Request(TriggerManual)
	<-firstStarted
	// Three quick saves while compiling → a single follow-up run.
	s.Request(TriggerManual)
	s.Request(TriggerManual)
	s.Request(TriggerManual)
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

	s.Request(TriggerManual)
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

	s.Request(TriggerManual)
	if res := rec.waitFinished(t); res.Success {
		t.Fatalf("a panic must be reported as a failure, got %+v", res)
	}
	// The scheduler must still work after the panic.
	s.Request(TriggerManual)
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

	s.Request(TriggerManual)
	<-started
	s.Close(time.Second)

	select {
	case <-cancelled:
	default:
		t.Fatal("Close must cancel the running compilation before returning")
	}
	if s.Request(TriggerManual) {
		t.Fatal("Request after Close must be rejected")
	}
	select {
	case res := <-rec.finished:
		t.Fatalf("no compile:finished must be emitted after Close, got %+v", res)
	case <-time.After(50 * time.Millisecond):
	}
}

// blockingCompile blocks every run until ctx is cancelled or release is
// closed, and reports which runs saw their context cancelled.
func blockingCompile(release <-chan struct{}, started chan<- int, canceled *atomic.Int32) CompileFunc {
	var n atomic.Int32
	return func(ctx context.Context) (string, error) {
		started <- int(n.Add(1))
		select {
		case <-ctx.Done():
			canceled.Add(1)
			return "", ctx.Err()
		case <-release:
			return "ok", nil
		}
	}
}

func TestSchedulerCancelsAnObsoleteAutoRun(t *testing.T) {
	release := make(chan struct{})
	started := make(chan int, 8)
	var canceled atomic.Int32
	rec := newRecorder()
	s := NewScheduler(blockingCompile(release, started, &canceled), rec.hooks())
	defer s.Close(time.Second)

	s.Request(TriggerAuto)
	<-started
	s.Request(TriggerAuto) // young auto run: cancelled, the follow-up starts
	if res := rec.waitFinished(t); !res.Canceled || res.Success {
		t.Fatalf("first run = %+v, want Canceled", res)
	}
	<-started
	// The follow-up of a cancelled run is protected: never twice in a row.
	s.Request(TriggerAuto)
	time.Sleep(50 * time.Millisecond)
	if canceled.Load() != 1 {
		t.Fatalf("canceled = %d, want 1", canceled.Load())
	}
	close(release)
	if res := rec.waitFinished(t); !res.Success {
		t.Fatalf("protected run = %+v", res)
	}
	if res := rec.waitFinished(t); !res.Success { // the coalesced request
		t.Fatalf("follow-up = %+v", res)
	}
}

func TestSchedulerNeverCancelsManualOrOldRuns(t *testing.T) {
	release := make(chan struct{})
	started := make(chan int, 8)
	var canceled atomic.Int32
	rec := newRecorder()
	s := NewScheduler(blockingCompile(release, started, &canceled), rec.hooks())
	defer s.Close(time.Second)

	s.Request(TriggerManual)
	<-started
	s.Request(TriggerAuto)
	time.Sleep(50 * time.Millisecond)
	if canceled.Load() != 0 {
		t.Fatal("a manual run (⌘S) must never be cancelled")
	}
	close(release)
	rec.waitFinished(t)
	rec.waitFinished(t)

	// An auto run older than half the last duration is left to finish.
	release2 := make(chan struct{})
	s2 := NewScheduler(blockingCompile(release2, started, &canceled), rec.hooks())
	defer s2.Close(time.Second)
	s2.mu.Lock()
	s2.lastDuration = 20 * time.Millisecond
	s2.mu.Unlock()
	s2.Request(TriggerAuto)
	<-started
	time.Sleep(30 * time.Millisecond) // > lastDuration / 2
	s2.Request(TriggerAuto)
	time.Sleep(30 * time.Millisecond)
	if canceled.Load() != 0 {
		t.Fatal("an auto run past its cancel window must finish")
	}
	close(release2)
	rec.waitFinished(t)
	rec.waitFinished(t)
}

// ── Archetype tests (testify) ──────────────────────────────────────────────

// compileEvent is what the mock compile:started / compile:finished
// channels carry: which run, and which version of the "file" it compiled.
type compileEvent struct {
	run     int
	version string
	result  Result
}

// TestSchedulerThreeRapidAutoRequestsKeepOnlyTheLatest: three automatic
// requests in < 500 ms while a run is in progress. The intermediate state
// is never compiled to the end: the running compilation is aborted (its
// context is cancelled, as KillTreeOnCancel would kill latexmk) and only
// the last run finishes, with the newest data.
func TestSchedulerThreeRapidAutoRequestsKeepOnlyTheLatest(t *testing.T) {
	var disk atomic.Value // the document on disk, as latexmk would read it
	disk.Store("v1")

	started := make(chan compileEvent, 8)  // mock compile:started
	finished := make(chan compileEvent, 8) // mock compile:finished
	firstRunning := make(chan struct{})
	firstKilled := make(chan struct{})
	dying := make(chan struct{}) // killing latexmk is not instantaneous

	var runs atomic.Int32
	var current atomic.Value // version of the run in progress
	s := NewScheduler(func(ctx context.Context) (string, error) {
		n := runs.Add(1)
		version := disk.Load().(string)
		current.Store(compileEvent{run: int(n), version: version})
		if n == 1 {
			close(firstRunning)
			select {
			case <-ctx.Done():
				close(firstKilled)
				<-dying
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
				return "v1 terminada", nil // must not happen
			}
		}
		return version, nil
	}, Hooks{
		OnStarted: func() {
			// OnStarted runs before the CompileFunc reads the disk: report the run number only.
			started <- compileEvent{run: int(runs.Load()) + 1}
		},
		OnFinished: func(r Result) {
			ev := current.Load().(compileEvent)
			ev.result = r
			finished <- ev
		},
	})
	defer s.Close(time.Second)

	receive := func(ch chan compileEvent, what string) compileEvent {
		t.Helper()
		select {
		case ev := <-ch:
			return ev
		case <-time.After(2 * time.Second):
			require.FailNow(t, "timed out waiting for "+what)
			return compileEvent{}
		}
	}

	start := time.Now()
	require.True(t, s.Request(TriggerAuto))
	<-firstRunning
	disk.Store("v2")
	require.True(t, s.Request(TriggerAuto)) // aborts run 1
	disk.Store("v3")
	require.True(t, s.Request(TriggerAuto)) // coalesces with the previous one
	assert.Less(t, time.Since(start), 500*time.Millisecond, "the three requests must be rapid")

	select {
	case <-firstKilled:
	case <-time.After(time.Second):
		require.FailNow(t, "the running compilation's context was not cancelled")
	}
	close(dying)

	assert.Equal(t, 1, receive(started, "started #1").run)
	first := receive(finished, "finished #1")
	assert.True(t, first.result.Canceled, "run 1 must be aborted, not finished")
	assert.False(t, first.result.Success)
	assert.Equal(t, "v1", first.version)

	assert.Equal(t, 2, receive(started, "started #2").run)
	last := receive(finished, "finished #2")
	assert.True(t, last.result.Success)
	assert.False(t, last.result.Canceled)
	assert.Equal(t, "v3", last.version, "the only finished run must see the newest data")
	assert.Equal(t, "v3", last.result.Logs)

	select {
	case ev := <-started:
		assert.Failf(t, "unexpected extra run", "%+v", ev) // v2 was never compiled on its own
	case <-time.After(150 * time.Millisecond):
	}
	assert.EqualValues(t, 2, runs.Load())
}
