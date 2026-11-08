package system

import (
	"os/exec"
	"runtime"
)

// RevealInFileManager opens dir in the OS file manager (Finder, the Linux
// desktop's default, Explorer). It runs the opener directly, without a
// shell; callers must validate dir first.
func RevealInFileManager(dir string) error {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name = "explorer"
	default:
		name = "xdg-open"
	}
	cmd := exec.Command(name, dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	// The opener returns quickly; reap it in the background instead of
	// blocking the binding until the file manager window is up.
	go func() { _ = cmd.Wait() }()
	return nil
}
