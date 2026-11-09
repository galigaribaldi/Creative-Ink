package compiler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Result is the outcome of one compilation, sent to the frontend as the
// payload of the "compile:finished" event.
type Result struct {
	Success    bool   `json:"success"`
	Logs       string `json:"logs"`
	DurationMs int64  `json:"durationMs"`
	// Canceled: an automatic run was stopped because a newer request made it
	// obsolete. Another run follows at once; the UI keeps "compiling".
	Canceled bool `json:"canceled"`
}

// Trigger tells why a compilation was requested.
type Trigger int

const (
	// TriggerAuto: the editor's idle timer or opening a file. It can be
	// cancelled by a newer request (see Scheduler.Request).
	TriggerAuto Trigger = iota
	// TriggerManual: ⌘S, the Compilar button. Never cancelled.
	TriggerManual
)

// Cancellation limits (docs/MAP.md §4.5): an automatic run is only
// cancelled while it is young, so a long document is never restarted over
// and over without finishing.
const (
	// unknownCancelWindow applies before any run of this scheduler finished.
	unknownCancelWindow = 5 * time.Second
)

// CompileFunc runs one compilation. It must honour ctx cancellation.
type CompileFunc func(ctx context.Context) (logs string, err error)

// Hooks notify progress. They run on the scheduler goroutine, never
// concurrently with each other.
type Hooks struct {
	OnStarted  func()
	OnFinished func(Result)
}

// Scheduler runs compilations in the background, one at a time, coalescing
// bursts: requests that arrive while a compilation runs collapse into a
// single follow-up run. Three quick saves → at most two compilations, and the
// last one always sees the latest file on disk.
//
// Bounded cancellation: a request that arrives while an AUTOMATIC run is
// young (less than half the last run's duration, or 5 s before the first
// one finished) cancels it and the follow-up starts at once, so latexmk does
// not finish a version that is already obsolete. Guards against
// starvation: a manual run is never cancelled, and a run that follows a
// cancelled one cannot be cancelled either (never twice in a row). The
// CompileFunc must kill its whole process tree on ctx cancellation.
//
// It has no Wails dependency: the App injects Hooks that emit events.
type Scheduler struct {
	compile CompileFunc
	hooks   Hooks

	ctx    context.Context // cancelled by Close: kills a running latexmk
	cancel context.CancelFunc
	wg     sync.WaitGroup // tracks the worker goroutine

	mu            sync.Mutex
	running       bool // a worker goroutine is active
	pending       bool // a request arrived while running
	pendingManual bool // ...and at least one of them was manual
	closed        bool
	current       *runState     // the run in progress (nil between runs)
	lastDuration  time.Duration // of the last run that was not cancelled
}

// runState is the run in progress, guarded by Scheduler.mu.
type runState struct {
	trigger   Trigger
	started   time.Time
	cancel    context.CancelFunc
	canceled  bool // cancelled by a newer request
	protected bool // follows a cancelled run: cannot be cancelled
}

// NewScheduler creates an idle scheduler.
func NewScheduler(compile CompileFunc, hooks Hooks) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{compile: compile, hooks: hooks, ctx: ctx, cancel: cancel}
}

// Request asks for a compilation and returns immediately. It reports false
// only if the scheduler is closed.
func (s *Scheduler) Request(trigger Trigger) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case s.closed:
		return false
	case s.running:
		s.pending = true // coalesce: one more run after the current one
		s.pendingManual = s.pendingManual || trigger == TriggerManual
		if cur := s.current; cur != nil && s.cancellable(cur) {
			cur.canceled = true
			cur.cancel() // the follow-up starts as soon as latexmk dies
		}
		return true
	}
	s.running = true
	s.wg.Add(1)
	go s.worker(trigger)
	return true
}

// cancellable applies the limits described on Scheduler. Called with mu held.
func (s *Scheduler) cancellable(cur *runState) bool {
	if cur.trigger != TriggerAuto || cur.canceled || cur.protected {
		return false
	}
	window := unknownCancelWindow
	if s.lastDuration > 0 {
		window = s.lastDuration / 2
	}
	return time.Since(cur.started) < window
}

// Close cancels any running compilation, drops pending requests and waits
// up to timeout for the worker to exit, so latexmk does not outlive the app.
func (s *Scheduler) Close(timeout time.Duration) {
	s.mu.Lock()
	s.closed = true
	s.pending = false
	s.mu.Unlock()
	s.cancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Printf("[compiler] la compilación no terminó en %s tras cancelarla", timeout)
	}
}

func (s *Scheduler) worker(trigger Trigger) {
	defer s.wg.Done()
	protected := false
	for {
		canceled := s.runOnce(trigger, protected)

		s.mu.Lock()
		if s.closed || !s.pending {
			s.running = false
			s.mu.Unlock()
			return
		}
		trigger = TriggerAuto
		if s.pendingManual {
			trigger = TriggerManual
		}
		s.pending, s.pendingManual = false, false
		s.mu.Unlock()
		protected = canceled // never cancel twice in a row
	}
}

// runOnce performs one compilation and reports whether it was cancelled by
// a newer request. A panic anywhere in it is recovered and reported as a
// failed Result: in Go, a panic in a goroutine kills the app.
func (s *Scheduler) runOnce(trigger Trigger, protected bool) (canceled bool) {
	start := time.Now()
	ctx, cancel := context.WithCancel(s.ctx)
	run := &runState{trigger: trigger, started: start, cancel: cancel, protected: protected}
	s.mu.Lock()
	s.current = run
	s.mu.Unlock()

	var res Result
	defer func() {
		if r := recover(); r != nil {
			res = Result{Success: false, Logs: fmt.Sprintf("error interno durante la compilación: %v", r)}
		}
		cancel()
		elapsed := time.Since(start)
		s.mu.Lock()
		s.current = nil
		canceled = run.canceled
		if !canceled {
			s.lastDuration = elapsed
		}
		s.mu.Unlock()
		if canceled {
			res = Result{Canceled: true, Logs: "Compilación cancelada: había una versión más reciente."}
		}
		res.DurationMs = elapsed.Milliseconds()
		// After Close the frontend is going away: do not notify it.
		if s.ctx.Err() == nil && s.hooks.OnFinished != nil {
			safeCall(func() { s.hooks.OnFinished(res) })
		}
	}()

	if s.hooks.OnStarted != nil {
		s.hooks.OnStarted()
	}
	logs, err := s.compile(ctx)
	res = toResult(logs, err)
	return false // overwritten by the deferred function
}

// toResult turns CompileLaTeX's (logs, error) into the frontend payload.
func toResult(logs string, err error) Result {
	if err == nil {
		return Result{Success: true, Logs: logs}
	}
	msg := err.Error()
	if logs != "" {
		msg += "\n\n" + logs
	}
	return Result{Success: false, Logs: msg}
}

// safeCall keeps a panicking hook from crashing the worker goroutine.
func safeCall(f func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[compiler] panic en un hook: %v", r)
		}
	}()
	f()
}
