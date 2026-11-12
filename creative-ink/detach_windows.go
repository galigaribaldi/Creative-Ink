//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detachedProcess: the new window does not depend on this one's console.
const detachedProcess = 0x00000008

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}
