package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vrothberg/skillctl/internal/manifest"
	"go.podman.io/image/v5/signature"
)

func TestStoresKeepIndependentTrust(t *testing.T) {
	root := t.TempDir()
	key := writeKeyFile(t, root, "key.pub")
	storePath := filepath.Join(root, "store")
	trusted, err := NewWithOptions(storePath, StoreOptions{Skills: []manifest.Skill{
		{Name: "skill", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{Key: key}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trusted.Close() })
	err = WithLock(storePath, func() error {
		other, err := NewWithOptions(storePath, StoreOptions{Skills: []manifest.Skill{
			{Name: "skill", Image: "quay.io/example/skill:v2", Trust: manifest.Trust{None: true}},
		}})
		if err != nil {
			return err
		}
		defer func() { _ = other.Close() }()
		for _, tc := range []struct {
			store *Store
			scope string
			want  string
		}{
			{trusted, "quay.io/example/skill:v1", "sigstoreSigned"},
			{other, "quay.io/example/skill:v2", "insecureAcceptAnything"},
		} {
			policy, err := signature.DefaultPolicy(tc.store.sc)
			if err != nil {
				return err
			}
			rule, err := json.Marshal(policy.Transports["docker"][tc.scope])
			if err != nil {
				return err
			}
			if !strings.Contains(string(rule), tc.want) {
				t.Errorf("expected %s policy, got %s", tc.want, rule)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if trusted.sc.SignaturePolicyPath == filepath.Join(storePath, "policy.json") {
		t.Fatal("verification must not use the shared policy path")
	}
	if err := trusted.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(trusted.trustDir); !os.IsNotExist(err) {
		t.Fatalf("private trust directory was not removed: %v", err)
	}
}

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

func TestStoreTrustConfig(t *testing.T) {
	dir := t.TempDir()
	key := writeKeyFile(t, dir, "foo.pub")

	storePath := filepath.Join(t.TempDir(), "store")
	skills := []manifest.Skill{
		{Name: "trusted", Image: "quay.io/example/trusted:v1", Trust: manifest.Trust{Key: key}},
		{Name: "untrusted", Image: "quay.io/example/untrusted:v1", Trust: manifest.Trust{None: true}},
	}

	s, err := NewWithOptions(storePath, StoreOptions{Skills: skills, ManifestDir: dir})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	policyPath, registriesDir := s.sc.SignaturePolicyPath, s.sc.RegistriesDirPath
	if policyPath != filepath.Join(s.trustDir, "policy.json") || registriesDir != filepath.Join(s.trustDir, "registries.d") {
		t.Fatalf("store uses unexpected verification paths: %s, %s", policyPath, registriesDir)
	}
	info, err := os.Stat(s.trustDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("trust directory permissions: got %o, want 700", info.Mode().Perm())
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
	trusted := docker["quay.io/example/trusted:v1"].([]any)[0].(map[string]any)
	if trusted["type"] != "sigstoreSigned" || trusted["keyPath"] != key {
		t.Errorf("unexpected trusted rule:\n%s", data)
	}
	untrusted := docker["quay.io/example/untrusted:v1"].([]any)[0].(map[string]any)
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
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{policyPath, registriesDir, s.trustDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("verification path %q was not cleaned up: %v", path, err)
		}
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
		want   string
		skills []manifest.Skill
	}{
		{"undeclared", `skill "a" has no trust declaration`, []manifest.Skill{{Name: "a", Image: "quay.io/example/a:v1"}}},
		{"missing key file", `trust key "nope.pub" for "quay.io/example/a:v1"`, []manifest.Skill{{Name: "a", Image: "quay.io/example/a:v1", Trust: manifest.Trust{Key: "nope.pub"}}}},
		{"invalid image", `parsing image reference ":::invalid:::"`, []manifest.Skill{{Name: "a", Image: ":::invalid:::", Trust: manifest.Trust{None: true}}}},
		{
			// Skills pulling the same content must agree on trust.
			"conflicting trust same image",
			`conflicting trust declarations for "quay.io/example/skill:v1": skills "a" (trust: key:` + key + `) and "b" (trust: none)`,
			[]manifest.Skill{
				{Name: "a", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{Key: key}},
				{Name: "b", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{None: true}},
			},
		},
		{
			// The tag of a tagged and digested reference is ignored.
			"conflicting trust same digest",
			`conflicting trust declarations for "quay.io/example/skill@` + testDigest + `"`,
			[]manifest.Skill{
				{Name: "a", Image: "quay.io/example/skill@" + testDigest, Trust: manifest.Trust{Key: key}},
				{Name: "b", Image: "quay.io/example/skill:v1@" + testDigest, Trust: manifest.Trust{None: true}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storePath := filepath.Join(t.TempDir(), "store")
			_, _, err := writeTrustConfig(storePath, tc.skills, dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPath(t *testing.T) {
	t.Setenv(StoreEnvVar, "/from-env")

	// The flag wins over the environment variable.
	got, err := Path("/from-flag")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != "/from-flag" {
		t.Errorf("expected flag path to win, got %q", got)
	}

	// The environment variable wins over the default.
	got, err = Path("")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != "/from-env" {
		t.Errorf("expected env path, got %q", got)
	}

	// Without flag and environment variable, the default is used.
	t.Setenv(StoreEnvVar, "")
	got, err = Path("")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".local", "share", "skillctl", "store")) {
		t.Errorf("expected default store path, got %q", got)
	}
}

const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestWriteTrustConfigMixedTrustSameRepo(t *testing.T) {
	dir := t.TempDir()
	key := writeKeyFile(t, dir, "foo.pub")
	storePath := filepath.Join(t.TempDir(), "store")

	skills := []manifest.Skill{
		{Name: "signed", Image: "quay.io/example/skill:v2", Trust: manifest.Trust{Key: key}},
		{Name: "legacy", Image: "quay.io/example/skill:v1", Trust: manifest.Trust{None: true}},
		{Name: "alias", Image: "quay.io/example/skill:v2", Trust: manifest.Trust{Key: key}},
	}
	policyPath, registriesDir, err := writeTrustConfig(storePath, skills, dir)
	if err != nil {
		t.Fatalf("skills from one repository must keep their own trust: %v", err)
	}
	policy, err := signature.NewPolicyFromFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	docker := policy.Transports["docker"]
	if len(docker) != 2 {
		t.Fatalf("expected one rule per reference, got %v", docker)
	}
	for scope, want := range map[string]string{
		"quay.io/example/skill:v2": "sigstoreSigned",
		"quay.io/example/skill:v1": "insecureAcceptAnything",
	} {
		rule, err := json.Marshal(docker[scope])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(rule), want) {
			t.Errorf("%s: expected %s, got %s", scope, want, rule)
		}
	}
	if _, ok := docker["quay.io/example/skill"]; ok {
		t.Error("trust must not apply to the whole repository")
	}
	regData, err := os.ReadFile(filepath.Join(registriesDir, "skillctl.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(regData), "quay.io/example/skill:\n        use-sigstore-attachments: true") {
		t.Errorf("expected repository-wide sigstore attachments:\n%s", regData)
	}
}

func TestSameImage(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"quay.io/team/skill:v1", "quay.io/team/skill:v1", true},
		{"quay.io/team/skill", "quay.io/team/skill:latest", true},
		{"quay.io/team/skill:v1@" + testDigest, "quay.io/team/skill:v2@" + testDigest, true},
		{"skill:v1", "docker.io/library/skill:v1", true},
		{"quay.io/team/skill:v1", "quay.io/team/skill:v2", false},
		{"quay.io/team/skill:v1", "quay.io/team/skill@" + testDigest, false},
		{"quay.io/team/skill:v1", "quay.io/team/other:v1", false},
		{":::invalid:::", ":::invalid:::", false},
	} {
		if got := SameImage(tc.a, tc.b); got != tc.want {
			t.Errorf("SameImage(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestPolicyScope(t *testing.T) {
	for _, tc := range []struct {
		input, identity, repository string
	}{
		{"quay.io/team/skill:v1", "quay.io/team/skill:v1", "quay.io/team/skill"},
		{"quay.io/team/skill", "quay.io/team/skill:latest", "quay.io/team/skill"},
		{"quay.io/team/skill@" + testDigest, "quay.io/team/skill@" + testDigest, "quay.io/team/skill"},
		{"quay.io/team/skill:v1@" + testDigest, "quay.io/team/skill@" + testDigest, "quay.io/team/skill"},
		{"localhost:5000/skill:v1", "localhost:5000/skill:v1", "localhost:5000/skill"},
		{"skill:v1", "docker.io/library/skill:v1", "docker.io/library/skill"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			identity, repository, err := policyScope(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if identity != tc.identity || repository != tc.repository {
				t.Errorf("got (%q, %q), want (%q, %q)", identity, repository, tc.identity, tc.repository)
			}
		})
	}
}
