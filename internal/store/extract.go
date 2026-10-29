package store

import (
	"io"
	"os"
	"path/filepath"

	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/idtools"
)

func untar(reader io.Reader, directory string) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	// Keep ownership local, including parent directories implicitly created
	// by Untar. Archive UIDs/GIDs must not require privileged chown calls.
	return archive.Untar(reader, directory, &archive.TarOptions{
		NoLchown:  true,
		ChownOpts: &idtools.IDPair{},
		UIDMaps:   []idtools.IDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GIDMaps:   []idtools.IDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	})
}
