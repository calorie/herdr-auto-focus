package autofocus

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type fileLock struct {
	file *os.File
	held bool
}

func acquireFileLock(path string, nonblocking bool) (*fileLock, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open lock file: %w", err)
	}
	operation := syscall.LOCK_EX
	if nonblocking {
		operation |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(file.Fd()), operation); err != nil {
		closeErr := file.Close()
		if nonblocking && (errors.Is(err, syscall.EWOULDBLOCK) ||
			errors.Is(err, syscall.EAGAIN)) {
			return nil, false, closeErr
		}
		return nil, false, errors.Join(fmt.Errorf("acquire file lock: %w", err), closeErr)
	}
	return &fileLock{file: file, held: true}, true, nil
}

func (lock *fileLock) Unlock() error {
	if lock == nil || !lock.held {
		return nil
	}
	unlockErr := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.held = false
	return errors.Join(unlockErr, closeErr)
}
