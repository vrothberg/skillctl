package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vrothberg/skillctl/internal/manifest"
)

func writeKeyFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("key-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteTrustConfig(t *testing.T) {
	dir := t.TempDir()
	key := writeKeyFile(t, dir, "foo.pub")

	storePath := filepath.Join(t.TempDir(), "store")
	skills := []manifest.Skill{
		{Name: "trusted", Image: "quay.io/example/trusted:v1", Trust: manifest.Trust{Key: key}},
		{Name: "untrusted", Image: "quay.io/example/untrusted:v1", Trust: manifest.Trust{None: true}},
	}

	policyPath, registriesDir, err := writeTrustConfig(storePath, skills, dir)
	if err != nil {
		t.Fatalf("writeTrustConfig: %v", err)
	}
	if policyPath != filepath.Join(storePath, "policy.json") {
		t.Errorf("unexpected policy path %q", policyPath)
	}

	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	var policy map[string]any
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatalf("policy is not valid JSON: %v\n%s", err, data)
	}

	// Everything not covered by a rule must be rejected.
	def := policy["default"].([]any)
	if def[0].(map[string]any)["type"] != "reject" {
		t.Errorf("expected default reject, got:\n%s", data)
	}

	docker := policy["transports"].(map[string]any)["docker"].(map[string]any)
	trusted := docker["quay.io/example/trusted"].([]any)[0].(map[string]any)
	if trusted["type"] != "sigstoreSigned" || trusted["keyPath"] != key {
		t.Errorf("unexpected trusted rule:\n%s", data)
	}
	untrusted := docker["quay.io/example/untrusted"].([]any)[0].(map[string]any)
	if untrusted["type"] != "insecureAcceptAnything" {
		t.Errorf("unexpected untrusted rule:\n%s", data)
	}

	// Sigstore attachments must only be looked up for verified repositories.
	regData, err := os.ReadFile(filepath.Join(registriesDir, "skillctl.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	regYAML := string(regData)
	if !strings.Contains(regYAML, "quay.io/example/trusted:") {
		t.Errorf("expected trusted repo in registries.d:\n%s", regYAML)
	}
	if strings.Contains(regYAML, "untrusted") {
		t.Errorf("unexpected untrusted repo in registries.d:\n%s", regYAML)
	}
	if !strings.Contains(regYAML, "use-sigstore-attachments: true") {
		t.Errorf("expected use-sigstore-attachments in registries.d:\n%s", regYAML)
	}
}

func TestWriteTrustConfigRelativeKey(t *testing.T) {
	dir := t.TempDir()
	writeKeyFile(t, dir, "keys/foo.pub")
	storePath := filepath.Join(t.TempDir(), "store")

	// Relative key paths resolve against the manifest directory, not the
	// process working directory.
	_, _, err := writeTrustConfig(storePath, []manifest.Skill{
		{Name: "a", Image: "quay.io/example/a:v1", Trust: manifest.Trust{Key: "keys/foo.pub"}},
	}, dir)
	if err != nil {
		t.Fatalf("writeTrustConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(storePath, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.Abs(filepath.Join(dir, "keys/foo.pub"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), expected) {
		t.Errorf("expected resolved key path %q in policy:\n%s", expected, data)
	}
}

func TestWriteTrustConfigErrors(t *testing.T) {
	dir := t.TempDir()
	key := writeKeyFile(t, dir, "foo.pub")

	tests := []struct {
		name   string
		skills []manifest.Skill
	}{
		{"undeclared", []manifest.Skill{{Name: "a", Image: "quay.io/example/a:v1"}}},
		{"missing key file", []manifest.Skill{{Name: "a", Image: "quay.io/example/a:v1", Trust: manifest.Trust{Key: "nope.pub"}}}},
		{"invalid image", []manifest.Skill{{Name: "a", Image: ":::invalid:::", Trust: manifest.Trust{None: true}}}},
		{
			// Policy granularity is per repository, so skills sharing a
			// repository must agree on trust.
			"conflicting trust same repo",
			[]manifest.Skill{
				{Name: "a", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{Key: key}},
				{Name: "b", Image: "quay.io/example/skill:v2", Trust: manifest.Trust{None: true}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storePath := filepath.Join(t.TempDir(), "store")
			if _, _, err := writeTrustConfig(storePath, tc.skills, dir); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

func TestWriteTrustConfigSameKeySameRepo(t *testing.T) {
	dir := t.TempDir()
	key := writeKeyFile(t, dir, "foo.pub")
	storePath := filepath.Join(t.TempDir(), "store")

	skills := []manifest.Skill{
		{Name: "a", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{Key: key}},
		{Name: "b", Image: "quay.io/example/skill:v2", Trust: manifest.Trust{Key: key}},
	}
	if _, _, err := writeTrustConfig(storePath, skills, dir); err != nil {
		t.Errorf("expected skills with the same key to merge: %v", err)
	}
}
