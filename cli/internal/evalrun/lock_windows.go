//go:build windows

package evalrun

import (
	"errors"
	"syscall"
)

// ErrLocked is returned by Lock when another cech eval holds the same
// commit's results; the caller exits with status 3, since a retry may
// succeed (criterion 29).
var ErrLocked = errors.New("another cech eval holds this commit's results")

// Lock opens the lock file at path for this process alone, which
// Windows releases when the process ends, so a killed cech eval leaves
// no stale hold (criterion 29). The returned function releases it.
func Lock(path string) (func(), error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		// ERROR_SHARING_VIOLATION: another process has the file open.
		if errors.Is(err, syscall.Errno(32)) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() { _ = syscall.CloseHandle(h) }, nil
}
