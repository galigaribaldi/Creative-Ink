//go:build !windows

package compiler

import (
	"context"
	"os"
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
	}, func() string { return "después\n" }, BuildOutput{})
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
	}, func() string { called = true; return "" }, BuildOutput{})
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
		}, func() string { return "" }, BuildOutput{})
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
	if _, err := RunPasses(context.Background(), t.TempDir(), []Pass{{Tool: "no-existe-creative-ink"}}, nil, BuildOutput{}); err == nil {
		t.Error("a missing tool must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if _, err := RunPasses(ctx, t.TempDir(), []Pass{{Tool: "/bin/sh", Args: []string{"-c", "sleep 20"}}}, nil, BuildOutput{}); err == nil {
		t.Error("a cancelled build must fail")
	}
}

func TestRunPassesStreamsOutputAndProgress(t *testing.T) {
	dir := t.TempDir()
	// Like XeLaTeX: page markers (one wrapped), lots of noise, final summary.
	script := `printf '(./main.tex [1] [2{/x/pdftex.map}]\n[3\n]'; ` +
		`i=0; while [ $i -lt 20000 ]; do echo "Overfull \\hbox (1.0pt too wide) in paragraph $i"; i=$((i+1)); done; ` +
		`echo 'Output written on main.pdf (3 pages, 1234 bytes).'`
	var log strings.Builder
	var events []BuildProgress
	report, err := RunPasses(context.Background(), dir, []Pass{
		{Tool: "/bin/sh", Args: []string{"-c", script}},
		{Tool: "/bin/sh", Args: []string{"-c", "echo segunda"}},
	}, nil, BuildOutput{Log: &log, Progress: func(p BuildProgress) { events = append(events, p) }, ExpectedPages: 2})
	if err != nil {
		t.Fatalf("err = %v\n%s", err, report)
	}
	// The full output reaches the log (~1 MB); the report stays compact.
	if !strings.Contains(log.String(), "paragraph 19999") || !strings.Contains(log.String(), "segunda") {
		t.Fatal("the log must receive the whole output of every pass")
	}
	if len(report) > 1024 {
		t.Fatalf("report = %d bytes, want a compact summary", len(report))
	}

	first, last := events[0], events[len(events)-1]
	if first.Pass != 1 || first.Passes != 2 || first.Pages != 2 || first.Done {
		t.Errorf("first = %+v (expected pages come from the previous build)", first)
	}
	sawPage3 := false
	for _, e := range events {
		sawPage3 = sawPage3 || (e.Pass == 1 && e.Page == 3 && e.Pages == 3)
	}
	if !sawPage3 {
		t.Errorf("pass 1 must report page 3 of 3: %+v", events)
	}
	if !last.Done || last.Failed || last.Pass != 2 {
		t.Errorf("last = %+v", last)
	}
	if len(events) > 12 {
		t.Errorf("%d progress events: they must be throttled", len(events))
	}
}

func TestFailureSummaryUsesTheBoundedTail(t *testing.T) {
	var w passWriter
	_, _ = w.Write([]byte(strings.Repeat("ruido\n", 200000))) // 1.2 MB
	_, _ = w.Write([]byte("! Undefined control sequence.\nl.9 \\foo\n"))
	if got := len(w.tail.buf); got > 2*maxTailBytes {
		t.Fatalf("tail holds %d bytes, want <= %d", got, 2*maxTailBytes)
	}
	if s := failureSummary(w.tail.String()); !strings.Contains(s, "Undefined control sequence") {
		t.Fatalf("summary = %q", s)
	}
}

func TestPagesFromLog(t *testing.T) {
	path := t.TempDir() + "/tesis.log"
	if PagesFromLog(path) != 0 {
		t.Fatal("missing log = 0")
	}
	content := strings.Repeat("x\n", 20000) + "Output written on tesis.pdf (212 pages, 9 bytes).\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PagesFromLog(path); got != 212 {
		t.Fatalf("pages = %d", got)
	}
}
