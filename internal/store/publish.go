package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/libartifact"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/pkg/blobinfocache/none"
	"go.podman.io/image/v5/signature/signer"
	"go.podman.io/image/v5/types"
	"gopkg.in/yaml.v3"
)

type PublishOptions struct {
	Sign                  SignOptions
	InsecureSkipTLSVerify bool
}

// ParsePublishReference validates an explicit tag and normalizes the registry
// name so the published reference can be copied directly into pull.
func ParsePublishReference(image string) (libartifact.ArtifactReference, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return libartifact.ArtifactReference{}, fmt.Errorf("parsing publish reference %q: %w", image, err)
	}
	if _, ok := ref.(reference.Digested); ok {
		return libartifact.ArtifactReference{}, fmt.Errorf("publish reference must not contain a digest")
	}
	if _, ok := ref.(reference.NamedTagged); !ok {
		return libartifact.ArtifactReference{}, fmt.Errorf("publish reference requires an explicit tag")
	}
	return libartifact.NewArtifactReference(ref.String())
}

// Publish stages a reproducible artifact in a private temporary OCI store.
// It never adds artifacts to the consumer's persistent store or manifest.
func Publish(ctx context.Context, image, directory string, opts PublishOptions) (digest.Digest, error) {
	ref, err := ParsePublishReference(image)
	if err != nil {
		return "", err
	}
	signing, err := opts.Sign.signer()
	if err != nil {
		return "", err
	}
	if signing != nil {
		defer func() { _ = signing.Close() }()
	}
	stage, err := os.MkdirTemp("", "skillctl-publish-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	sourcePath, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("opening source directory: %w", err)
	}
	sourcePath, err = filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	stage, err = filepath.EvalSymlinks(stage)
	if err != nil {
		return "", err
	}
	if relative, err := filepath.Rel(sourcePath, stage); err == nil && filepath.IsLocal(relative) {
		return "", fmt.Errorf("temporary staging directory is inside the source directory; set TMPDIR outside the source directory")
	}
	layer, err := os.Create(filepath.Join(stage, "skill.tar.gz"))
	if err != nil {
		return "", err
	}
	packageErr := packageDirectory(ctx, directory, layer)
	closeErr := layer.Close()
	if packageErr != nil {
		return "", fmt.Errorf("packaging skill: %w", packageErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	storePath := filepath.Join(stage, "store")
	if err := stageArtifact(ctx, storePath, ref.String(), layer.Name()); err != nil {
		return "", fmt.Errorf("staging artifact: %w", err)
	}
	sc, err := publishContext(stage, storePath, ref.RepoName(), opts.InsecureSkipTLSVerify)
	if err != nil {
		return "", err
	}
	artifact, err := libartifact.NewArtifactStore(storePath, sc)
	if err != nil {
		return "", err
	}
	copyOpts := libimage.CopyOptions{}
	if signing != nil {
		copyOpts.Signers = []*signer.Signer{signing}
	}
	d, err := artifact.Push(ctx, ref, ref, copyOpts)
	if err != nil {
		return "", fmt.Errorf("publishing %q (registry content and signatures may have been partially uploaded): %w", image, err)
	}
	return d, nil
}

func stageArtifact(ctx context.Context, storePath, name, layer string) error {
	ref, err := layout.NewReference(storePath, name)
	if err != nil {
		return err
	}
	dest, err := ref.NewImageDestination(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dest.Close() }()
	d, size, err := layout.PutBlobFromLocalFile(ctx, dest, layer)
	if err != nil {
		return err
	}
	if _, err := dest.PutBlob(ctx, bytes.NewReader(oci.DescriptorEmptyJSON.Data), types.BlobInfo{
		Digest: oci.DescriptorEmptyJSON.Digest, Size: oci.DescriptorEmptyJSON.Size,
	}, none.NoCache, true); err != nil {
		return err
	}
	// libartifact.Add stamps a wall-clock creation annotation. Construct the
	// manifest here so neither timestamps nor local paths affect its digest.
	manifest := oci.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: oci.MediaTypeImageManifest,
		Config: oci.DescriptorEmptyJSON,
		Layers: []oci.Descriptor{{MediaType: oci.MediaTypeImageLayerGzip, Digest: d, Size: size}},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := dest.PutManifest(ctx, raw, nil); err != nil {
		return err
	}
	return dest.Commit(ctx, nil)
}

func publishContext(stage, storePath, repository string, insecure bool) (*types.SystemContext, error) {
	localRef, err := layout.NewReference(storePath, "")
	if err != nil {
		return nil, err
	}
	// Accept only the artifact we just built locally. Publishing must not
	// depend on ambient signature policy or registries.d configuration.
	policy := policyConfig{
		Default: []json.RawMessage{json.RawMessage(`{"type":"reject"}`)},
		Transports: map[string]map[string][]json.RawMessage{
			"oci": {localRef.PolicyConfigurationIdentity(): {json.RawMessage(`{"type":"insecureAcceptAnything"}`)}},
		},
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	policyPath := filepath.Join(stage, "policy.json")
	if err := os.WriteFile(policyPath, raw, 0o600); err != nil {
		return nil, err
	}
	registries := registriesConfig{Docker: map[string]registryNamespace{}}
	attachments := true
	registries.Docker[repository] = registryNamespace{UseSigstoreAttachments: &attachments}
	registriesDir := filepath.Join(stage, "registries.d")
	if err := os.MkdirAll(registriesDir, 0o700); err != nil {
		return nil, err
	}
	registriesYAML, err := yaml.Marshal(registries)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(registriesDir, "skillctl.yaml"), registriesYAML, 0o600); err != nil {
		return nil, err
	}
	sc := &types.SystemContext{SignaturePolicyPath: policyPath, RegistriesDirPath: registriesDir}
	if insecure {
		sc.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}
	return sc, nil
}
