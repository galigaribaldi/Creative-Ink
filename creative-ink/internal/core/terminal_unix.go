//go:build !windows

package core

import (
	"context"
	"os/exec"
	"syscall"
)

// shellCommand runs the command through /bin/sh in its own process group,
// so a timeout or cancel kills the whole pipeline (sh and every child),
// not just the shell.
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Negative PID = the process group created by Setpgid.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return cmd
}
