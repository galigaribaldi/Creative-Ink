//go:build !windows

package system

import (
	"os/exec"
	"syscall"
)

// KillTreeOnCancel makes a cmd created with exec.CommandContext run in its
// own process group and, on cancellation or timeout, kill the whole group:
// latexmk and the xelatex/biber it spawned, not just latexmk. Without it a
// cancelled build leaves xelatex writing the same .aux as the next run.
func KillTreeOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Negative PID = the process group created by Setpgid.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
