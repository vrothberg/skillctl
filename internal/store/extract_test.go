package store

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vrothberg/skillctl/internal/skillfs"
)

type tarEntry struct {
	header tar.Header
	data   string
}

func tarLayer(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	for _, entry := range entries {
		if entry.header.Typeflag == tar.TypeReg {
			entry.header.Size = int64(len(entry.data))
		}
		if err := writer.WriteHeader(&entry.header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func installLayers(layers ...[]byte) error {
	return skillfs.Install(".agents/skills/test", func(root *os.Root) error {
		for _, layer := range layers {
			if err := untar(bytes.NewReader(layer), root.Name()); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestUntarKeepsOwnershipLocal(t *testing.T) {
	t.Chdir(t.TempDir())
	layer := tarLayer(t, tarEntry{
		header: tar.Header{Name: "parent/file", Typeflag: tar.TypeReg, Mode: 0o640, Uid: 123456, Gid: 234567},
		data:   "content",
	})
	if err := installLayers(layer); err != nil {
		t.Fatal(err)
	}
	// The wrapper must handle archive owners and implicit parent directories
	// without preserving foreign ownership or needing privileged chown calls.
	for _, name := range []string{"parent", "parent/file"} {
		info, err := os.Stat(".agents/skills/test/" + name)
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if stat.Uid != uint32(os.Getuid()) || stat.Gid != uint32(os.Getgid()) {
			t.Errorf("%s: got ownership %d:%d, want %d:%d", name, stat.Uid, stat.Gid, os.Getuid(), os.Getgid())
		}
	}
}

func TestUntarConfinesWrites(t *testing.T) {
	for _, name := range []string{"traversal", "absolute", "symlink parent", "hardlink traversal", "root file"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll(".agents/skills/outside-dir", 0o755); err != nil {
				t.Fatal(err)
			}
			const sentinel = ".agents/skills/outside-dir/sentinel"
			if err := os.WriteFile(sentinel, []byte("preserve me"), 0o644); err != nil {
				t.Fatal(err)
			}
			absolute, err := filepath.Abs(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			entry := tarEntry{header: tar.Header{Typeflag: tar.TypeReg, Mode: 0o644}, data: "archive contents"}
			var entries []tarEntry
			var wantErr string
			switch name {
			case "traversal":
				entry.header.Name, wantErr = "../outside-dir/sentinel", "outside of"
			case "absolute":
				entry.header.Name = absolute
			case "symlink parent":
				entries = append(entries, tarEntry{header: tar.Header{Name: "alias", Typeflag: tar.TypeSymlink, Linkname: "../outside-dir"}})
				entry.header.Name = "alias/sentinel"
			case "hardlink traversal":
				entry = tarEntry{header: tar.Header{Name: "hardlink", Typeflag: tar.TypeLink, Linkname: "../outside-dir/sentinel"}}
				wantErr = "invalid hardlink"
			case "root file":
				entry.header.Name, wantErr = ".", "archive root"
			}
			entries = append(entries, entry)
			err = installLayers(tarLayer(t, entries...))
			if wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), wantErr) {
					t.Fatalf("expected %q error, got %v", wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(sentinel)
			if err != nil || string(data) != "preserve me" {
				t.Fatalf("extraction changed an outside file: %q, %v", data, err)
			}
		})
	}
}

func TestFailedUntarPreservesInstalledSkill(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".agents/skills/test", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".agents/skills/test/SKILL.md", []byte("installed"), 0o644); err != nil {
		t.Fatal(err)
	}
	layer := tarLayer(t, tarEntry{header: tar.Header{Name: "partial", Typeflag: tar.TypeReg, Mode: 0o644}, data: "new"})
	// Replace tar's two EOF blocks with an incomplete header, after a valid
	// file has already been extracted into staging.
	layer = append(layer[:len(layer)-1024], []byte("truncated header")...)
	if err := installLayers(layer); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected unexpected EOF, got %v", err)
	}
	data, err := os.ReadFile(".agents/skills/test/SKILL.md")
	if err != nil || string(data) != "installed" {
		t.Fatalf("failed extraction changed installed files: %q, %v", data, err)
	}
	entries, err := os.ReadDir(".agents/skills")
	if err != nil || len(entries) != 1 || entries[0].Name() != "test" {
		t.Fatalf("staging files remain after failure: %v, %v", entries, err)
	}
}
