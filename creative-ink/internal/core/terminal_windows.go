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
// The line is `cmd.exe /d /s /c "<command>"` (cmdLine). Without /s, a
// command that starts with a quote and has more than two quotes
// (`"C:\Program Files\Git\bin\git.exe" log "--format=%h %s"`) loses its
// first and last quote and breaks; with /s, cmd.exe strips exactly the
// pair added here. /d skips the AutoRun commands from the registry, so a
// command runs the same on every machine.
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
		CmdLine:       cmdLine(shell, command),
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
