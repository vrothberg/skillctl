package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// writeVerificationConfig handles file layout and serialization for both pull
// and publish. Callers decide which policy and registry scopes to allow.
func writeVerificationConfig(directory string, policy policyConfig, registries registriesConfig) (string, string, error) {
	registriesDir := filepath.Join(directory, "registries.d")
	if err := os.MkdirAll(registriesDir, 0o700); err != nil {
		return "", "", fmt.Errorf("creating verification directory: %w", err)
	}
	policyJSON, err := json.MarshalIndent(policy, "", "    ")
	if err != nil {
		return "", "", fmt.Errorf("marshaling policy: %w", err)
	}
	policyPath := filepath.Join(directory, "policy.json")
	if err := os.WriteFile(policyPath, policyJSON, 0o600); err != nil {
		return "", "", fmt.Errorf("writing policy.json: %w", err)
	}
	registriesYAML, err := yaml.Marshal(registries)
	if err != nil {
		return "", "", fmt.Errorf("marshaling registries.d: %w", err)
	}
	if err := os.WriteFile(filepath.Join(registriesDir, "skillctl.yaml"), registriesYAML, 0o600); err != nil {
		return "", "", fmt.Errorf("writing registries.d: %w", err)
	}
	return policyPath, registriesDir, nil
}

// policyConfig mirrors the containers/image policy.json format. Map keys
// sort deterministically in the encoded JSON.
type policyConfig struct {
	Default    []json.RawMessage                       `json:"default"`
	Transports map[string]map[string][]json.RawMessage `json:"transports"`
}

// registriesConfig and registryNamespace mirror the registries.d format.
type registriesConfig struct {
	Docker map[string]registryNamespace `yaml:"docker"`
}

type registryNamespace struct {
	UseSigstoreAttachments *bool `yaml:"use-sigstore-attachments,omitempty"`
}
