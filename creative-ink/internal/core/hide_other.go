//go:build !windows

package core

// hidePath is a no-op on macOS and Linux: a leading dot already hides it.
func hidePath(string) error { return nil }
