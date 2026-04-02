//go:build linux || darwin

package vpn

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// flockFile acquires an exclusive lock on a file descriptor (Unix implementation)
func flockFile(lockFile *os.File) error {
	return syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX)
}

// unflockFile releases a lock on a file descriptor (Unix implementation)
func unflockFile(lockFile *os.File) error {
	return syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
}

// acquireLockWithTimeout acquires an exclusive lock with timeout (Unix implementation)
func acquireLockWithTimeout(lockFile *os.File, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		err := flockFile(lockFile)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout acquiring lock after %v", timeout)
	}
}

// releaseLock releases an exclusive lock (Unix implementation)
func releaseLock(lockFile *os.File) error {
	return unflockFile(lockFile)
}
