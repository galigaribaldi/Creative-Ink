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
}

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
// It has no Wails dependency: the App injects Hooks that emit events.
type Scheduler struct {
	compile CompileFunc
	hooks   Hooks

	ctx    context.Context // cancelled by Close: kills a running latexmk
	cancel context.CancelFunc
	wg     sync.WaitGroup // tracks the worker goroutine

	mu      sync.Mutex
	running bool // a worker goroutine is active
	pending bool // a request arrived while running
	closed  bool
}

// NewScheduler creates an idle scheduler.
func NewScheduler(compile CompileFunc, hooks Hooks) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{compile: compile, hooks: hooks, ctx: ctx, cancel: cancel}
}

// Request asks for a compilation and returns immediately. It reports false
// only if the scheduler is closed.
func (s *Scheduler) Request() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case s.closed:
		return false
	case s.running:
		s.pending = true // coalesce: one more run after the current one
		return true
	}
	s.running = true
	s.wg.Add(1)
	go s.worker()
	return true
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

func (s *Scheduler) worker() {
	defer s.wg.Done()
	for {
		s.runOnce()

		s.mu.Lock()
		if s.closed || !s.pending {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.pending = false
		s.mu.Unlock()
	}
}

// runOnce performs one compilation. A panic anywhere in it is recovered and
// reported as a failed Result: in Go, a panic in a goroutine kills the app.
func (s *Scheduler) runOnce() {
	start := time.Now()
	var res Result
	defer func() {
		if r := recover(); r != nil {
			res = Result{Success: false, Logs: fmt.Sprintf("error interno durante la compilación: %v", r)}
		}
		res.DurationMs = time.Since(start).Milliseconds()
		// After Close the frontend is going away: do not notify it.
		if s.ctx.Err() == nil && s.hooks.OnFinished != nil {
			safeCall(func() { s.hooks.OnFinished(res) })
		}
	}()

	if s.hooks.OnStarted != nil {
		s.hooks.OnStarted()
	}
	logs, err := s.compile(s.ctx)
	res = toResult(logs, err)
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
