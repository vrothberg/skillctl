package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vrothberg/skillctl/internal/manifest"
	"go.podman.io/image/v5/docker/policyconfiguration"
	"go.podman.io/image/v5/docker/reference"
)

// writeTrustConfig generates the signature policy and registries.d
// configuration for the given skills under storePath, and returns the paths
// to hand to the library. Setting those paths on the SystemContext replaces
// any ambient system configuration: skillctl's trust model lives in the
// manifest, not in /etc/containers.
//
// Trust is scoped to each skill's image reference. The generated policy
// requires a valid sigstore signature for references with a pinned key,
// accepts references opted out with "none", and rejects everything else.
// Skills from the same repository may therefore declare different trust.
func writeTrustConfig(storePath string, skills []manifest.Skill, manifestDir string) (string, string, error) {
	type scoped struct {
		repository string
		trust      manifest.Trust
	}
	scopes := map[string]scoped{}
	for _, skill := range skills {
		if !skill.Trust.Declared() {
			return "", "", fmt.Errorf("skill %q has no trust declaration", skill.Name)
		}
		identity, repository, err := policyScope(skill.Image)
		if err != nil {
			return "", "", fmt.Errorf("parsing image reference %q: %w", skill.Image, err)
		}
		if existing, ok := scopes[identity]; ok && existing.trust != skill.Trust {
			return "", "", fmt.Errorf("conflicting trust declarations for %q: skills referencing the same image must declare the same trust", identity)
		}
		scopes[identity] = scoped{repository: repository, trust: skill.Trust}
	}

	policy := policyConfig{
		Default:    []json.RawMessage{json.RawMessage(`{"type": "reject"}`)},
		Transports: map[string]map[string][]json.RawMessage{"docker": {}},
	}
	registries := registriesConfig{Docker: map[string]registryNamespace{}}

	for scope, entry := range scopes {
		trust := entry.trust
		switch {
		case trust.Key != "":
			keyPath, err := resolveKeyPath(trust.Key, manifestDir)
			if err != nil {
				return "", "", err
			}
			if _, err := os.Stat(keyPath); err != nil {
				return "", "", fmt.Errorf("trust key %q for %q: %w", trust.Key, scope, err)
			}
			rule, err := json.Marshal(map[string]string{
				"type":    "sigstoreSigned",
				"keyPath": keyPath,
			})
			if err != nil {
				return "", "", fmt.Errorf("building policy rule: %w", err)
			}
			policy.Transports["docker"][scope] = []json.RawMessage{rule}
			// Only key verification needs the registry's sigstore
			// attachments; lookaside signatures are not used. The
			// registries.d lookup is per repository, and fetching
			// attachments is harmless for opted-out references.
			attachments := true
			registries.Docker[entry.repository] = registryNamespace{UseSigstoreAttachments: &attachments}
		case trust.None:
			rule, err := json.Marshal(map[string]string{"type": "insecureAcceptAnything"})
			if err != nil {
				return "", "", fmt.Errorf("building policy rule: %w", err)
			}
			policy.Transports["docker"][scope] = []json.RawMessage{rule}
		}
	}

	return writeVerificationConfig(storePath, policy, registries)
}

// policyScope returns the policy identity of the reference skillctl pulls,
// e.g. "quay.io/team/skill:v1" or "quay.io/team/skill@sha256:...", and its
// repository. Like the pull path, it drops the tag of a reference carrying a
// digest and defaults bare names to "latest", then asks containers/image for
// the identity its policy lookup matches exactly.
func policyScope(imageRef string) (string, string, error) {
	normalized, err := normalizeImageReference(imageRef)
	if err != nil {
		return "", "", err
	}
	ref, err := reference.ParseNormalizedNamed(normalized)
	if err != nil {
		return "", "", err
	}
	ref = reference.TagNameOnly(ref)
	identity, err := policyconfiguration.DockerReferenceIdentity(ref)
	if err != nil {
		return "", "", err
	}
	return identity, ref.Name(), nil
}

// resolveKeyPath resolves a trust key path against the manifest directory,
// since relative paths in the manifest are relative to the manifest, not to
// the process working directory.
func resolveKeyPath(key, manifestDir string) (string, error) {
	if filepath.IsAbs(key) {
		return filepath.Clean(key), nil
	}
	abs, err := filepath.Abs(filepath.Join(manifestDir, key))
	if err != nil {
		return "", fmt.Errorf("resolving trust key path %q: %w", key, err)
	}
	return abs, nil
}
