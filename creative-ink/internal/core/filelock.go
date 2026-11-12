package core

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrLocked: another process (another Creative-ink window) holds the lock.
var ErrLocked = errors.New("lo está usando otra ventana de Creative-ink")

// LockFile takes an exclusive lock on path (created if needed), waiting for
// other processes to release it. Several Creative-ink processes may run at
// once (a new window, or the binary started twice), so read-modify-write of
// shared files (settings.json) must hold it. Call the returned function to
// release it.
func LockFile(path string) (func(), error) {
	return lockFile(path, true)
}

// TryLockFile is LockFile without waiting: ErrLocked if it is held.
func TryLockFile(path string) (func(), error) {
	return lockFile(path, false)
}

func openLockFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
}
