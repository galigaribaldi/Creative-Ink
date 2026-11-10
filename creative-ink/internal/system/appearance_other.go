//go:build !darwin && !windows && !linux

package system

// prefersDark: no portable way to ask; assume light.
func prefersDark() bool { return false }
