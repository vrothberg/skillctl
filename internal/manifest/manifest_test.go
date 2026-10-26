package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNew(t *testing.T) {
	m := New()
	if m.APIVersion != APIVersion {
		t.Errorf("expected apiVersion %q, got %q", APIVersion, m.APIVersion)
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty skills, got %d", len(m.Skills))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	m := New()
	m.AddOrUpdate(Skill{
		Name:        "translator",
		Image:       "quay.io/redhat/skills/translator:1.0",
		Digest:      "sha256:abc123",
		ExtractPath: ".agents/skills/translator",
	})

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.APIVersion != APIVersion {
		t.Errorf("expected apiVersion %q, got %q", APIVersion, loaded.APIVersion)
	}
	if len(loaded.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded.Skills))
	}

	s := loaded.Skills[0]
	if s.Name != "translator" {
		t.Errorf("expected name %q, got %q", "translator", s.Name)
	}
	if s.Image != "quay.io/redhat/skills/translator:1.0" {
		t.Errorf("expected image %q, got %q", "quay.io/redhat/skills/translator:1.0", s.Image)
	}
	if s.Digest != "sha256:abc123" {
		t.Errorf("expected digest %q, got %q", "sha256:abc123", s.Digest)
	}
}

func TestAddOrUpdate_New(t *testing.T) {
	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1"})
	m.AddOrUpdate(Skill{Name: "b", Image: "img:2"})

	if len(m.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(m.Skills))
	}
}

func TestAddOrUpdate_Existing(t *testing.T) {
	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1", Digest: "sha256:old"})
	m.AddOrUpdate(Skill{Name: "a", Image: "img:2", Digest: "sha256:new"})

	if len(m.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(m.Skills))
	}
	if m.Skills[0].Image != "img:2" {
		t.Errorf("expected updated image %q, got %q", "img:2", m.Skills[0].Image)
	}
	if m.Skills[0].Digest != "sha256:new" {
		t.Errorf("expected updated digest %q, got %q", "sha256:new", m.Skills[0].Digest)
	}
}

func TestFind(t *testing.T) {
	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1"})
	m.AddOrUpdate(Skill{Name: "b", Image: "img:2"})

	s := m.Find("a")
	if s == nil {
		t.Fatal("expected to find skill 'a'")
	}
	if s.Image != "img:1" {
		t.Errorf("expected image %q, got %q", "img:1", s.Image)
	}

	if m.Find("nonexistent") != nil {
		t.Error("expected nil for nonexistent skill")
	}
}

func TestRemove(t *testing.T) {
	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1"})
	m.AddOrUpdate(Skill{Name: "b", Image: "img:2"})

	if !m.Remove("a") {
		t.Error("expected Remove to return true for existing skill")
	}
	if len(m.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(m.Skills))
	}
	if m.Skills[0].Name != "b" {
		t.Errorf("expected remaining skill %q, got %q", "b", m.Skills[0].Name)
	}
}

func TestRemoveNonexistent(t *testing.T) {
	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1"})

	if m.Remove("nonexistent") {
		t.Error("expected Remove to return false for nonexistent skill")
	}
	if len(m.Skills) != 1 {
		t.Fatalf("expected 1 skill unchanged, got %d", len(m.Skills))
	}
}

func TestSkillExtractPath(t *testing.T) {
	path := SkillExtractPath("translator")
	expected := filepath.Join(DefaultSkillDir, "translator")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestSaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1", Digest: "sha256:aaa"})
	if err := m.Save(path); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	m.AddOrUpdate(Skill{Name: "b", Image: "img:2", Digest: "sha256:bbb"})
	if err := m.Save(path); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(loaded.Skills))
	}
}

func TestRemoveAndSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	m := New()
	m.AddOrUpdate(Skill{Name: "a", Image: "img:1"})
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m.Remove("a")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save after remove: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(loaded.Skills))
	}
}

func TestValidateSkillName(t *testing.T) {
	valid := []string{"translator", "my-skill", "skill_v2", "skill.config"}
	for _, name := range valid {
		if err := ValidateSkillName(name); err != nil {
			t.Errorf("expected %q to be valid, got: %v", name, err)
		}
	}

	invalid := []string{"", "../../etc", "../foo", "foo/bar", "foo\\bar", ".", ".."}
	for _, name := range invalid {
		if err := ValidateSkillName(name); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/skillctl.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadRejectsInvalidSkillName(t *testing.T) {
	tests := []struct {
		name     string
		skillName string
	}{
		{"path traversal", "../../etc/evil"},
		{"path separator", "foo/bar"},
		{"dot-dot", ".."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")

			m := New()
			m.Skills = append(m.Skills, Skill{
				Name:  tc.skillName,
				Image: "quay.io/example/skill:v1",
			})
			data, err := yaml.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(path); err == nil {
				t.Errorf("expected Load to reject skill name %q", tc.skillName)
			}
		})
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestTrustRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		trust Trust
		yaml  string
	}{
		{"none", Trust{None: true}, "none\n"},
		{"key", Trust{Key: "keys/foo.pub"}, "key: keys/foo.pub\n"},
		{"undeclared", Trust{}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")

			m := New()
			m.AddOrUpdate(Skill{
				Name:  "a",
				Image: "quay.io/example/skill:v1",
				Trust: tc.trust,
			})
			if err := m.Save(path); err != nil {
				t.Fatalf("Save: %v", err)
			}

			if tc.yaml != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "trust:") {
					t.Errorf("expected trust in manifest:\n%s", data)
				}
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "trust") {
					t.Errorf("expected no trust in manifest:\n%s", data)
				}
			}

			loaded, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := loaded.Skills[0].Trust
			if got != tc.trust {
				t.Errorf("expected trust %+v, got %+v", tc.trust, got)
			}
			if got.Declared() != tc.trust.Declared() {
				t.Errorf("Declared mismatch for %+v", got)
			}
		})
	}
}

func TestLoadInvalidTrust(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"unknown scalar", "apiVersion: v1\nskills:\n    - name: a\n      image: quay.io/example/skill:v1\n      trust: maybe\n"},
		{"unknown field", "apiVersion: v1\nskills:\n    - name: a\n      image: quay.io/example/skill:v1\n      trust:\n          fingerprint: abc\n"},
		{"empty key", "apiVersion: v1\nskills:\n    - name: a\n      image: quay.io/example/skill:v1\n      trust:\n          key: \"\"\n"},
		{"key missing", "apiVersion: v1\nskills:\n    - name: a\n      image: quay.io/example/skill:v1\n      trust: {}\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(path); err == nil {
				t.Errorf("expected Load to reject trust:\n%s", tc.yaml)
			}
		})
	}
}

func TestLoadLegacyManifestWithoutTrust(t *testing.T) {
	// Manifests written before trust existed load fine; the trust
	// declaration is enforced when the skills are pulled or updated.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	legacy := "apiVersion: v1\nskills:\n    - name: a\n      image: quay.io/example/skill:v1\n      digest: sha256:abc\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Skills[0].Trust.Declared() {
		t.Error("expected undeclared trust for legacy manifest")
	}
}

func TestParseTrust(t *testing.T) {
	tests := []struct {
		input string
		want  Trust
	}{
		{"none", Trust{None: true}},
		{"key=keys/foo.pub", Trust{Key: "keys/foo.pub"}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseTrust(tc.input)
			if err != nil {
				t.Fatalf("ParseTrust(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("expected %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestParseTrustInvalid(t *testing.T) {
	tests := []string{"", "foo", "key=", "keypath", "fulcio=x"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseTrust(input); err == nil {
				t.Errorf("expected ParseTrust(%q) to fail", input)
			}
		})
	}
}
