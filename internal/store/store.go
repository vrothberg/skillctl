package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/libartifact"
	libartTypes "go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/types"
)

const defaultStoreDir = ".local/share/skillctl/store"

type Store struct {
	artifact *libartifact.ArtifactStore
}

func DefaultStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, defaultStoreDir), nil
}

func New(storePath string) (*Store, error) {
	sc := &types.SystemContext{}
	as, err := libartifact.NewArtifactStore(storePath, sc)
	if err != nil {
		return nil, fmt.Errorf("creating artifact store: %w", err)
	}
	return &Store{artifact: as}, nil
}

func (s *Store) Pull(ctx context.Context, imageRef string) (digest.Digest, error) {
	ref, err := libartifact.NewArtifactReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	opts := libimage.CopyOptions{}
	d, err := s.artifact.Pull(ctx, ref, opts)
	if err != nil {
		return "", fmt.Errorf("pulling %q: %w", imageRef, err)
	}
	return d, nil
}

func (s *Store) Extract(ctx context.Context, imageRef string, targetDir string) error {
	asr, err := libartifact.NewArtifactStorageReference(imageRef)
	if err != nil {
		return fmt.Errorf("parsing store reference %q: %w", imageRef, err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating target directory %q: %w", targetDir, err)
	}

	opts := &libartTypes.ExtractOptions{}
	if err := s.artifact.Extract(ctx, asr, targetDir, opts); err != nil {
		return fmt.Errorf("extracting %q to %q: %w", imageRef, targetDir, err)
	}
	return nil
}

func (s *Store) List(ctx context.Context) (libartifact.ArtifactList, error) {
	return s.artifact.List(ctx)
}
