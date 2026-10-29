package store

import (
	"archive/tar"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.podman.io/image/v5/pkg/compression"
)

type tarMetadata struct {
	mode     os.FileMode
	modified time.Time
}

func unpackLayer(ctx context.Context, root *os.Root, blob io.Reader, directories map[string]tarMetadata) (retErr error) {
	stream, _, err := compression.AutoDecompress(blob)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, stream.Close()) }()
	buffer := bufio.NewReader(stream)
	if _, err := buffer.Peek(512); err != nil {
		return fmt.Errorf("invalid tar archive: %w", err)
	}
	reader := tar.NewReader(buffer)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if err == io.EOF {
			// Read through the compression trailer to verify its checksum.
			// Avoid pgzip's WriterTo shortcut after a partially read stream.
			_, err := io.Copy(io.Discard, struct{ io.Reader }{buffer})
			return err
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}
		name, err := archivePath(header.Name)
		if err != nil {
			return err
		}
		if name == "." && header.Typeflag != tar.TypeDir {
			return fmt.Errorf("archive root must be a directory")
		}
		if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		// A symlink parent can change the depth used to validate relative
		// link targets. Require real directories for archive entry parents.
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			info, err := root.Lstat(parent)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("archive path %q has a symlink parent", header.Name)
			}
		}
		if header.Typeflag == tar.TypeDir {
			info, err := root.Lstat(name)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && !info.IsDir() {
				if err := root.RemoveAll(name); err != nil {
					return err
				}
			}
			if err := root.MkdirAll(name, 0o755); err != nil {
				return err
			}
			directories[name] = tarMetadata{mode: header.FileInfo().Mode().Perm(), modified: header.ModTime}
			continue
		}
		if err := unpackEntry(root, name, header, reader); err != nil {
			return fmt.Errorf("extracting %q: %w", header.Name, err)
		}
		for directory := range directories {
			if directory == name || strings.HasPrefix(directory, name+string(os.PathSeparator)) {
				delete(directories, directory)
			}
		}
	}
}

func archivePath(name string) (string, error) {
	path := filepath.FromSlash(name)
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("archive path %q escapes the skill directory", name)
	}
	return filepath.Clean(path), nil
}

func unpackEntry(root *os.Root, name string, header *tar.Header, reader io.Reader) error {
	switch header.Typeflag {
	case tar.TypeReg, tar.TypeSymlink, tar.TypeLink:
	default:
		return fmt.Errorf("unsupported tar entry type %d", header.Typeflag)
	}
	if err := root.RemoveAll(name); err != nil {
		return err
	}
	switch header.Typeflag {
	case tar.TypeSymlink:
		link := filepath.FromSlash(header.Linkname)
		if filepath.IsAbs(link) || !filepath.IsLocal(filepath.Join(filepath.Dir(name), link)) {
			return fmt.Errorf("symlink %q escapes the skill directory", header.Linkname)
		}
		return root.Symlink(header.Linkname, name)
	case tar.TypeLink:
		link, err := archivePath(header.Linkname)
		if err != nil {
			return err
		}
		info, err := root.Lstat(link)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("hard link target %q is not a regular file", header.Linkname)
		}
		return root.Link(link, name)
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	return applyMetadata(root, name, tarMetadata{mode: header.FileInfo().Mode().Perm(), modified: header.ModTime})
}

func applyMetadata(root *os.Root, name string, metadata tarMetadata) error {
	if err := root.Chmod(name, metadata.mode); err != nil {
		return err
	}
	if !metadata.modified.IsZero() {
		return root.Chtimes(name, metadata.modified, metadata.modified)
	}
	return nil
}

func finishDirectories(root *os.Root, directories map[string]tarMetadata) error {
	names := make([]string, 0, len(directories))
	for name := range directories {
		names = append(names, name)
	}
	// Set permissions and times after children, including across layers.
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, name := range names {
		if err := applyMetadata(root, name, directories[name]); err != nil {
			return err
		}
	}
	return nil
}
