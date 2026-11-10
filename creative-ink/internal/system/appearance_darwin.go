//go:build darwin

package system

import (
	"context"
	"os/exec"
	"time"
)

// prefersDark asks macOS for the current appearance (also correct in
// "Auto" mode: the key reflects the appearance in use). No shell.
func prefersDark() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return false // the key does not exist in light mode
	}
	return parseMacInterfaceStyle(string(out))
}
