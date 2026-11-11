//go:build windows

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// createNewConsole gives cmd.exe a console window of its own (the app is a
// GUI process without one).
const createNewConsole = 0x00000010

// terminalCommand opens Windows Terminal in dir (`wt.exe -d .`, with dir as
// its working directory, so the path never goes through wt's own parser,
// where ";" separates commands) or, without it, cmd.exe in a new console.
func terminalCommand(dir string) (*exec.Cmd, string, error) {
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command(wt, "-d", "."), "Windows Terminal", nil
	}
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	return cmd, "Símbolo del sistema", nil
}
