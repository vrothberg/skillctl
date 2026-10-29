package skillfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRejectSymlinkEscape(t *testing.T) {
	for _, link := range []string{".agents", ".agents/skills"} {
		for _, operation := range []string{"install", "remove", "mkdir"} {
			t.Run(link+"/"+operation, func(t *testing.T) {
				project, outside := t.TempDir(), t.TempDir()
				t.Chdir(project)
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, link); err != nil {
					t.Fatal(err)
				}
				sentinel := filepath.Join(outside, "victim", "secret")
				writeFile(t, sentinel, "preserve me")
				target := filepath.Join(link, "victim")
				var err error
				switch operation {
				case "install":
					err = Install(target, func(root *os.Root) error {
						return root.WriteFile("SKILL.md", []byte("new skill"), 0o644)
					})
				case "remove":
					err = RemoveAll(target)
				case "mkdir":
					err = MkdirAll(filepath.Join(link, "new-directory"))
				}
				if err == nil || !strings.Contains(err.Error(), "escapes") {
					t.Fatalf("expected path escape error, got %v", err)
				}
				data, err := os.ReadFile(sentinel)
				if err != nil || string(data) != "preserve me" {
					t.Fatalf("outside file was changed: %q, %v", data, err)
				}
			})
		}
	}
}

func TestInstallReplacesFiles(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	writeFile(t, ".agents/skills/test/stale", "old")
	if err := Install(".agents/skills/test", func(root *os.Root) error {
		return root.WriteFile("SKILL.md", []byte("new"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(".agents/skills/test/SKILL.md")
	if err != nil || string(data) != "new" {
		t.Fatalf("installed content: %q, %v", data, err)
	}
	if _, err := os.Stat(".agents/skills/test/stale"); !os.IsNotExist(err) {
		t.Fatalf("stale file remains: %v", err)
	}
	entries, err := os.ReadDir(".agents/skills")
	if err != nil || len(entries) != 1 || entries[0].Name() != "test" {
		t.Fatalf("unexpected staging files remain: %v, %v", entries, err)
	}
}

func TestInvalidExtractionPreservesInstalledSkill(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, ".agents/skills/test/SKILL.md", "old skill")
	err := Install(".agents/skills/test", func(root *os.Root) error {
		if err := root.WriteFile("new-file", []byte("partial extraction"), 0o644); err != nil {
			return err
		}
		return errors.New("invalid archive")
	})
	if err == nil || !strings.Contains(err.Error(), "invalid archive") {
		t.Fatalf("expected invalid extraction error, got %v", err)
	}
	data, err := os.ReadFile(".agents/skills/test/SKILL.md")
	if err != nil || string(data) != "old skill" {
		t.Fatalf("installed skill was changed: %q, %v", data, err)
	}
}
