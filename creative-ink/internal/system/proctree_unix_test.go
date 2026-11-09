//go:build !windows

package system

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillTreeOnCancelKillsChildren(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// The shell starts a child (like latexmk → xelatex) and prints its PID.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30 & echo $!; wait")
	KillTreeOnCancel(cmd)
	cmd.WaitDelay = time.Second
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	_ = cmd.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(child, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(child, syscall.SIGKILL)
			t.Fatal("the child survived the cancellation")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
