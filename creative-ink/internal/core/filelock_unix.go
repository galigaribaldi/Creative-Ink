//go:build !windows

package core

import (
	"errors"
	"syscall"
)

// lockFile uses flock: the lock belongs to the open file, so it also
// serialises goroutines of the same process, and the OS releases it if the
// process dies.
func lockFile(path string, wait bool) (func(), error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX
	if !wait {
		how |= syscall.LOCK_NB
	}
	for {
		err = syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
