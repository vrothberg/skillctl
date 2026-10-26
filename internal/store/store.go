package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nightlyone/lockfile"
	"github.com/opencontainers/go-digest"
	"github.com/vrothberg/skillctl/internal/manifest"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/libartifact"
	libartTypes "go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/types"
)

const defaultStoreDir = ".local/share/skillctl/store"

type Store struct {
	artifact *libartifact.ArtifactStore
	sc       *types.SystemContext
}

type StoreOptions struct {
	// Skills are the skills in scope for the operation. Their trust
	// declarations drive the synthesized signature policy and registries.d
	// configuration.
	Skills []manifest.Skill
	// ManifestDir is the directory of the manifest; relative key paths in
	// trust declarations resolve against it.
	ManifestDir string
	// InsecureSkipTLSVerify skips TLS verification of registries.
	InsecureSkipTLSVerify bool
}

func DefaultStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, defaultStoreDir), nil
}

func NewWithOptions(storePath string, opts StoreOptions) (*Store, error) {
	sc := &types.SystemContext{}

	// Signature verification is configured exclusively by the manifest's
	// trust declarations. The generated configuration replaces any ambient
	// system policy and registries.d, so skillctl behaves identically on
	// every machine.
	policyPath, registriesDir, err := writeTrustConfig(storePath, opts.Skills, opts.ManifestDir)
	if err != nil {
		return nil, err
	}
	sc.SignaturePolicyPath = policyPath
	sc.RegistriesDirPath = registriesDir

	if opts.InsecureSkipTLSVerify {
		sc.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}

	as, err := libartifact.NewArtifactStore(storePath, sc)
	if err != nil {
		return nil, fmt.Errorf("creating artifact store: %w", err)
	}

	return &Store{artifact: as, sc: sc}, nil
}

func WithLock(fn func() error) error {
	storePath, err := DefaultStorePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storePath, 0o700); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}

	lock, err := lockfile.New(filepath.Join(storePath, "skillctl.lock"))
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

func (s *Store) Pull(ctx context.Context, imageRef string) (digest.Digest, error) {
	ref, err := libartifact.NewArtifactReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	opts := libimage.CopyOptions{
		// OCI layout does not support storing sigstore signatures, so
		// strip them after verification to avoid a copy error.
		RemoveSignatures: true,
	}
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
