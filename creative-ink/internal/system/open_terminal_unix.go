//go:build !darwin && !windows

package system

import (
	"os"
	"os/exec"
	"syscall"
)

// terminalCommand opens the desktop's terminal emulator in dir (see
// linuxTerminal). Setsid detaches it from the app's process group, so a
// signal to the app (Ctrl+C in `wails dev`) does not close it.
func terminalCommand(dir string) (*exec.Cmd, string, error) {
	path, args, name, err := linuxTerminal(dir, os.Getenv, exec.LookPath)
	if err != nil {
		return nil, "", err
	}
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd, name, nil
}
