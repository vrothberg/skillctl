// Package locking serializes cooperating skillctl processes using PID files.
package locking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nightlyone/lockfile"
)

// WithLock waits for the PID-file lock at path, giving up when ctx is done,
// and runs fn while holding it.
func WithLock(ctx context.Context, path string, fn func() error) error {
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
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := lock.TryLock()
		if err == nil {
			break
		}
		if err != lockfile.ErrBusy {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}
