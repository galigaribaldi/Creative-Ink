//go:build !windows

package compiler

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunPassesContinuesOrStops(t *testing.T) {
	dir := t.TempDir()

	// A failing pass with ContinueOnError does not stop the sequence…
	report, err := RunPasses(context.Background(), dir, []Pass{
		{Tool: "/bin/sh", Args: []string{"-c", "echo '! Undefined control sequence.'; echo 'l.3 \\foo'; exit 1"}, ContinueOnError: true},
		{Tool: "/bin/sh", Args: []string{"-c", "echo ok"}},
	}, func() string { return "después\n" })
	if err != nil {
		t.Fatalf("err = %v\n%s", err, report)
	}
	for _, want := range []string{"✗ falló", "Undefined control sequence", "✓", "después"} {
		if !strings.Contains(report, want) {
			t.Errorf("report must contain %q:\n%s", want, report)
		}
	}

	// …a blocking failure does, and `after` is skipped.
	called := false
	report, err = RunPasses(context.Background(), dir, []Pass{
		{Tool: "/bin/sh", Args: []string{"-c", "exit 2"}},
		{Tool: "/bin/sh", Args: []string{"-c", "echo nunca"}},
	}, func() string { called = true; return "" })
	if err == nil || called || strings.Contains(report, "nunca") {
		t.Fatalf("blocking failure: err=%v called=%v\n%s", err, called, report)
	}
}

func TestRunPassesHoldsTheCompileLock(t *testing.T) {
	dir := t.TempDir()
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_, _ = RunPasses(context.Background(), dir, []Pass{
			{Tool: "/bin/sh", Args: []string{"-c", "sleep 0.3"}},
		}, func() string { return "" })
		close(done)
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(started)
	}()
	<-started
	// While the manual build runs, a compilation (or a clean) must wait.
	acquired := make(chan struct{})
	go func() {
		WithCompileLock(func() {})
		close(acquired)
	}()
	select {
	case <-acquired:
		t.Fatal("the compile lock must be held during a manual build")
	case <-time.After(100 * time.Millisecond):
	}
	<-done
	<-acquired
}

func TestRunPassesMissingToolAndCancel(t *testing.T) {
	if _, err := RunPasses(context.Background(), t.TempDir(), []Pass{{Tool: "no-existe-creative-ink"}}, nil); err == nil {
		t.Error("a missing tool must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if _, err := RunPasses(ctx, t.TempDir(), []Pass{{Tool: "/bin/sh", Args: []string{"-c", "sleep 20"}}}, nil); err == nil {
		t.Error("a cancelled build must fail")
	}
}
