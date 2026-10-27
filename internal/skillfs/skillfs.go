// Package skillfs confines skill installation and removal to the project root.
package skillfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
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

// Install copies regular files from a private extraction directory into a
// rooted staging directory before replacing the installed skill. libartifact
// never receives a path under the potentially untrusted project directory.
func Install(source, target string) error {
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
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer func() { _ = sourceRoot.Close() }()
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return fmt.Errorf("extracted file %q is not a regular file", entry.Name())
		}
		if err := copyFile(sourceRoot, stageRoot, entry.Name()); err != nil {
			return err
		}
	}
	if err := root.Chmod(stage, 0o755); err != nil {
		return err
	}
	if err := root.RemoveAll(target); err != nil {
		return fmt.Errorf("removing old skill files: %w", err)
	}
	return root.Rename(stage, target)
}

func copyFile(source, target *os.Root, name string) error {
	in, err := source.Open(name)
	if err != nil {
		return err
	}
	out, err := target.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		_ = in.Close()
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, in.Close(), out.Close())
}
