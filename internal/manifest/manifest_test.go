package manifest

import (
	"os"
	"path/filepath"
	"testing"
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
		ExtractPath: "skills/translator",
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

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/skillctl.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
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
