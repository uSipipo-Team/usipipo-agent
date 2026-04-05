//go:build windows

package vpn

import (
	"os"
	"time"
)

// flockFile is a no-op on Windows (file locking handled by mutex)
func flockFile(lockFile *os.File) error {
	// Windows file locking is handled differently
	// The mutex in WireGuardClient provides sufficient synchronization
	return nil
}

// unflockFile is a no-op on Windows
func unflockFile(lockFile *os.File) error {
	return nil
}

// acquireLockWithTimeout is a no-op on Windows (mutex handles synchronization)
func acquireLockWithTimeout(lockFile *os.File, timeout time.Duration) error {
	// Windows file locking is handled differently
	// The mutex in WireGuardClient provides sufficient synchronization
	// Just close the lock file as it's not needed on Windows
	_ = lockFile.Close()
	return nil
}

// releaseLock is a no-op on Windows
func releaseLock(lockFile *os.File) error {
	return nil
}
