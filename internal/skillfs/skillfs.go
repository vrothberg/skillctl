// Package skillfs confines skill installation and removal to the project root.
package skillfs

import (
	"crypto/rand"
	"fmt"
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
	if err := root.RemoveAll(target); err != nil {
		return fmt.Errorf("removing old skill files: %w", err)
	}
	return root.Rename(stage, target)
}
