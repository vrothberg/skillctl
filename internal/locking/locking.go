// Package locking serializes cooperating skillctl processes using PID files.
package locking

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nightlyone/lockfile"
)

func WithLock(path string, fn func() error) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}
	lock, err := lockfile.New(path)
	if err != nil {
		return fmt.Errorf("creating lock file: %w", err)
	}
	for err := lock.TryLock(); err != nil; err = lock.TryLock() {
		if err != lockfile.ErrBusy {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}
