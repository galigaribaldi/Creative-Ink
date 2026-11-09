//go:build windows

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// createNoWindow keeps console tools (latexmk, xelatex, taskkill) from
// flashing a window: the app is a GUI process without a console.
const createNoWindow = 0x08000000

// KillTreeOnCancel makes a cmd created with exec.CommandContext kill its
// whole process tree on cancellation or timeout (taskkill /T), so latexmk's
// xelatex/biber children do not survive it. Falls back to killing the
// process alone if taskkill fails.
func KillTreeOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Cancel = func() error {
		taskkill := filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe")
		kill := exec.Command(taskkill, "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		if err := kill.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
