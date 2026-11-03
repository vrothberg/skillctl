package locking

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/nightlyone/lockfile"
)

func TestWithLockReturnsWhenContextIsDone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	// The lock is held by another live process; the library would
	// re-acquire a lock recorded with the test process's own PID.
	owner := exec.Command("sleep", "10")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = owner.Process.Kill()
		_ = owner.Wait()
	})
	if err := os.WriteFile(path, fmt.Appendf(nil, "%d\n", owner.Process.Pid), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	called := false
	err := WithLock(ctx, path, func() error {
		called = true
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if called {
		t.Fatal("function ran without acquiring the lock")
	}
}

func TestWithLockRunsAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	if err := WithLock(context.Background(), path, func() error { return nil }); err != nil {
		t.Fatal(err)
	}

	lock, err := lockfile.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.TryLock(); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	_ = lock.Unlock()
}
