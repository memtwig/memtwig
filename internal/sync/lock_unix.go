//go:build !windows

package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func acquireRepoLock(root string) (func(), error) {
	lockPath := filepath.Join(root, ".git", "memtwig-sync.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open sync lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another memtwig sync is already running on %s", root)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
