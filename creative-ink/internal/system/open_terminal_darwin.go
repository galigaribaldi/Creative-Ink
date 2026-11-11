package system

import "os/exec"

// terminalCommand opens Terminal.app in dir (`open -a Terminal <dir>`).
// LaunchServices starts it with the user's login shell and environment,
// not the app's.
func terminalCommand(dir string) (*exec.Cmd, string, error) {
	return exec.Command("open", "-a", "Terminal", dir), "Terminal", nil
}
