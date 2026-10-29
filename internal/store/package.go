package store

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

// packageDirectory records only portable file metadata. In particular, it
// does not record source ownership, timestamps, xattrs, or host paths.
func packageDirectory(ctx context.Context, directory string, output io.Writer) (retErr error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("opening source directory: %w", err)
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()
	compressed := gzip.NewWriter(output)
	defer func() { retErr = errors.Join(retErr, compressed.Close()) }()
	archive := tar.NewWriter(compressed)
	defer func() { retErr = errors.Join(retErr, archive.Close()) }()
	var files []struct {
		name string
		info fs.FileInfo
	}
	// WalkDir sorts each directory's entries and does not follow symlinks.
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header := &tar.Header{
			Name: name, Mode: int64(info.Mode().Perm()),
			ModTime: time.Unix(0, 0), Format: tar.FormatPAX,
		}
		switch {
		case info.IsDir():
			header.Typeflag = tar.TypeDir
			header.Name += "/"
		case info.Mode().IsRegular():
			header.Typeflag = tar.TypeReg
			header.Size = info.Size()
			for _, previous := range files {
				if os.SameFile(info, previous.info) {
					header.Typeflag, header.Linkname, header.Size = tar.TypeLink, previous.name, 0
					break
				}
			}
			files = append(files, struct {
				name string
				info fs.FileInfo
			}{name, info})
		case info.Mode()&os.ModeSymlink != 0:
			header.Typeflag = tar.TypeSymlink
			link, err := root.Readlink(name)
			if err != nil {
				return err
			}
			header.Linkname = link
		default:
			return fmt.Errorf("unsupported source entry %q (%s)", name, info.Mode().Type())
		}
		if err := archive.WriteHeader(header); err != nil {
			return fmt.Errorf("archiving %q: %w", name, err)
		}
		if header.Typeflag != tar.TypeReg {
			return nil
		}
		file, err := root.Open(name)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, file)
		return errors.Join(copyErr, file.Close())
	})
}
