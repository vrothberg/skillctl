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

const defaultPolicyJSON = `{"default":[{"type":"insecureAcceptAnything"}]}`

type Store struct {
	artifact *libartifact.ArtifactStore
	sc       *types.SystemContext
}

type StoreOptions struct {
	SignaturePolicyPath    string
	RegistriesDirPath     string
	InsecureSkipTLSVerify bool
}

func DefaultStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, defaultStoreDir), nil
}

func New(storePath string) (*Store, error) {
	return NewWithOptions(storePath, StoreOptions{})
}

func NewWithOptions(storePath string, opts StoreOptions) (*Store, error) {
	sc := &types.SystemContext{}

	if opts.SignaturePolicyPath != "" {
		sc.SignaturePolicyPath = opts.SignaturePolicyPath
	} else {
		policyPath, err := ensurePolicyJSON(storePath)
		if err != nil {
			return nil, err
		}
		if policyPath != "" {
			sc.SignaturePolicyPath = policyPath
		}
	}

	if opts.RegistriesDirPath != "" {
		sc.RegistriesDirPath = opts.RegistriesDirPath
	}

	if opts.InsecureSkipTLSVerify {
		sc.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}

	as, err := libartifact.NewArtifactStore(storePath, sc)
	if err != nil {
		return nil, fmt.Errorf("creating artifact store: %w", err)
	}
	return &Store{artifact: as, sc: sc}, nil
}

// ensurePolicyJSON returns a path to a policy.json file. If one already exists
// at the standard system locations, it returns "" so the library uses the
// system default. Otherwise it creates a permissive default under storePath.
func ensurePolicyJSON(storePath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}

	for _, p := range []string{
		filepath.Join(home, ".config/containers/policy.json"),
		"/etc/containers/policy.json",
		"/usr/share/containers/policy.json",
	} {
		if _, err := os.Stat(p); err == nil {
			return "", nil
		}
	}

	policyPath := filepath.Join(storePath, "policy.json")
	if _, err := os.Stat(policyPath); err == nil {
		return policyPath, nil
	}

	if err := os.MkdirAll(storePath, 0o700); err != nil {
		return "", fmt.Errorf("creating store directory: %w", err)
	}
	if err := os.WriteFile(policyPath, []byte(defaultPolicyJSON), 0o644); err != nil {
		return "", fmt.Errorf("writing default policy.json: %w", err)
	}
	return policyPath, nil
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
