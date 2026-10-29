package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vrothberg/skillctl/internal/manifest"
	"go.podman.io/image/v5/docker/reference"
)

// writeTrustConfig generates the signature policy and registries.d
// configuration for the given skills under storePath, and returns the paths
// to hand to the library. Setting those paths on the SystemContext replaces
// any ambient system configuration: skillctl's trust model lives in the
// manifest, not in /etc/containers.
//
// The generated policy requires a valid sigstore signature for every
// repository with a pinned key, accepts the repositories of skills opted out
// with "none", and rejects everything else.
func writeTrustConfig(storePath string, skills []manifest.Skill, manifestDir string) (string, string, error) {
	// Merge trust per repository: the containers/image policy granularity
	// is per repository, so skills sharing a repository must agree.
	scopes := map[string]manifest.Trust{}
	for _, skill := range skills {
		if !skill.Trust.Declared() {
			return "", "", fmt.Errorf("skill %q has no trust declaration", skill.Name)
		}
		scope, err := repoScope(skill.Image)
		if err != nil {
			return "", "", fmt.Errorf("parsing image reference %q: %w", skill.Image, err)
		}
		if existing, ok := scopes[scope]; ok && existing != skill.Trust {
			return "", "", fmt.Errorf("conflicting trust declarations for repository %q: skills sharing a repository must declare the same trust", scope)
		}
		scopes[scope] = skill.Trust
	}

	policy := policyConfig{
		Default:    []json.RawMessage{json.RawMessage(`{"type": "reject"}`)},
		Transports: map[string]map[string][]json.RawMessage{"docker": {}},
	}
	registries := registriesConfig{Docker: map[string]registryNamespace{}}

	for scope, trust := range scopes {
		switch {
		case trust.Key != "":
			keyPath, err := resolveKeyPath(trust.Key, manifestDir)
			if err != nil {
				return "", "", err
			}
			if _, err := os.Stat(keyPath); err != nil {
				return "", "", fmt.Errorf("trust key %q for repository %q: %w", trust.Key, scope, err)
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
			// attachments; lookaside signatures are not used.
			attachments := true
			registries.Docker[scope] = registryNamespace{UseSigstoreAttachments: &attachments}
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

// repoScope returns the policy namespace of an image reference: its
// repository without tag or digest, e.g. "quay.io/vrothberg/skill".
func repoScope(imageRef string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", err
	}
	return reference.Domain(ref) + "/" + reference.Path(ref), nil
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
