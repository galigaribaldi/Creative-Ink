//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach starts the new window in its own session: closing this window (or
// Ctrl+C in a terminal) does not close it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
