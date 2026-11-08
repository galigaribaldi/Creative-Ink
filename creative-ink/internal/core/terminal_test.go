//go:build !windows

package core

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteCommandCombinedOutputInWorkdir(t *testing.T) {
	dir := t.TempDir()
	out, err := ExecuteCommand("pwd; echo salida; echo error >&2", dir)
	if err != nil {
		t.Fatalf("err = %v, output:\n%s", err, out)
	}
	real, _ := filepath.EvalSymlinks(dir)
	for _, want := range []string{real, "salida", "error"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q must contain %q", out, want)
		}
	}
}

func TestExecuteCommandExitCode(t *testing.T) {
	out, err := ExecuteCommand("echo antes; exit 3", t.TempDir())
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("err = %v, want exit code 3", err)
	}
	if !strings.Contains(out, "antes") {
		t.Errorf("output before the failure must be kept, got %q", out)
	}
}

func TestExecuteCommandIsNonInteractive(t *testing.T) {
	start := time.Now()
	out, err := ExecuteCommand(`read x; echo "leído:[$x]"; echo "$GIT_TERMINAL_PROMPT"`, t.TempDir())
	if err != nil || !strings.Contains(out, "leído:[]") || !strings.Contains(out, "0") {
		t.Fatalf("out=%q err=%v: stdin must be empty and prompts disabled", out, err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("a command reading stdin must not hang")
	}
}

func TestExecuteCommandTimeoutKillsTheWholeGroup(t *testing.T) {
	old := terminalTimeout
	terminalTimeout = 200 * time.Millisecond
	t.Cleanup(func() { terminalTimeout = old })

	start := time.Now()
	// A child that outlives sh would keep the pipe open without the group kill.
	_, err := ExecuteCommand("sleep 20 & sleep 20", t.TempDir())
	if !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("err = %v, want ErrCommandTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %s: the process group was not killed", elapsed)
	}
}

func TestExecuteCommandCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if _, err := ExecuteCommandContext(ctx, "sleep 20", t.TempDir(), nil); !errors.Is(err, ErrCommandCanceled) {
		t.Fatalf("err = %v, want ErrCommandCanceled", err)
	}
}

func TestExecuteCommandCapsOutput(t *testing.T) {
	out, err := ExecuteCommand("head -c 300000 /dev/zero | tr '\\0' 'a'", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxTerminalOutput+100 || !strings.Contains(out, "salida truncada") {
		t.Fatalf("len = %d, want capped with a notice", len(out))
	}
}

func TestExecuteCommandValidation(t *testing.T) {
	if _, err := ExecuteCommand("   ", t.TempDir()); !errors.Is(err, ErrEmptyCommand) {
		t.Errorf("blank: err = %v", err)
	}
	if _, err := ExecuteCommand("ls", ""); !errors.Is(err, ErrNoFolder) {
		t.Errorf("no folder: err = %v", err)
	}
	if _, err := ExecuteCommand("ls", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing workdir must fail")
	}
}

func TestExecuteCommandExtraEnvOverrides(t *testing.T) {
	out, err := ExecuteCommandContext(context.Background(), `echo "$CI_PROBE"`, t.TempDir(), []string{"CI_PROBE=ok"})
	if err != nil || strings.TrimSpace(out) != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
