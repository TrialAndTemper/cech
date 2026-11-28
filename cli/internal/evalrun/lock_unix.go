//go:build !windows

package evalrun

import (
	"errors"
	"os"
	"syscall"
)

// ErrLocked is returned by Lock when another cech eval holds the same
// commit's results; the caller exits with status 3, since a retry may
// succeed (criterion 29).
var ErrLocked = errors.New("another cech eval holds this commit's results")

// Lock takes the lock file at path, which the operating system releases
// when the process ends, so a killed cech eval leaves no stale hold
// (criterion 29). The returned function releases it.
func Lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- the lock file in the commit's results
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { // #nosec G115 -- a file descriptor fits an int
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() { _ = f.Close() }, nil
}
