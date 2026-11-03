// Package skillfs confines skill installation and removal to the project root.
package skillfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func MkdirAll(path string) error {
	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.MkdirAll(path, 0o755)
}

func RemoveAll(path string) error {
	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.RemoveAll(path)
}

// Install populates a private staging directory before replacing the installed
// skill. The extractor is responsible for confining writes to that directory.
func Install(target string, extract func(*os.Root) error) error {
	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	parent := filepath.Dir(target)
	if err := root.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}
	stage := filepath.Join(parent, ".skillctl-stage-"+rand.Text())
	if err := root.Mkdir(stage, 0o700); err != nil {
		return err
	}
	defer func() { _ = root.RemoveAll(stage) }()
	stageRoot, err := root.OpenRoot(stage)
	if err != nil {
		return err
	}
	defer func() { _ = stageRoot.Close() }()
	if err := extract(stageRoot); err != nil {
		return err
	}
	old := filepath.Join(parent, ".skillctl-old-"+rand.Text())
	hadTarget := false
	err = root.Rename(target, old)
	switch {
	case err == nil:
		hadTarget = true
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("moving old skill files aside: %w", err)
	}
	if err := root.Rename(stage, target); err != nil {
		if hadTarget {
			// Best effort: keep the previous skill if the replacement fails.
			_ = root.Rename(old, target)
		}
		return fmt.Errorf("installing skill files: %w", err)
	}
	if hadTarget {
		// The new skill is installed; a leftover backup directory would be
		// hidden and harmless, so its removal is best effort.
		_ = root.RemoveAll(old)
	}
	return nil
}
