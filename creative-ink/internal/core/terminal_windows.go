//go:build windows

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// createNoWindow keeps cmd.exe from flashing a console window: the app is a
// GUI process without a console of its own.
const createNoWindow = 0x08000000

// shellCommand runs the command through cmd.exe. The raw command line is
// passed as-is (CmdLine): cmd.exe has its own quoting rules, and Go's
// argument escaping would mangle quoted paths.
//
// Known limit: on timeout or cancel only cmd.exe is killed; grandchildren
// may survive (a Job Object would be needed to kill the whole tree).
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.CommandContext(ctx, shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `"` + shell + `" /c ` + command,
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
