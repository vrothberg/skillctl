package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vrothberg/skillctl/internal/skillfs"
	"go.podman.io/image/v5/pkg/compression"
)

type tarEntry struct {
	header tar.Header
	data   string
}

func tarLayer(t *testing.T, format string, entries ...tarEntry) []byte {
	t.Helper()
	var plain bytes.Buffer
	writer := tar.NewWriter(&plain)
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
	var compressed bytes.Buffer
	switch format {
	case "gzip":
		gzipWriter := gzip.NewWriter(&compressed)
		if _, err := gzipWriter.Write(plain.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := gzipWriter.Close(); err != nil {
			t.Fatal(err)
		}
	case "zstd":
		zstdWriter, err := compression.CompressStream(&compressed, compression.Zstd, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := zstdWriter.Write(plain.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := zstdWriter.Close(); err != nil {
			t.Fatal(err)
		}
	default:
		return plain.Bytes()
	}
	return compressed.Bytes()
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

func TestTarLayersPreserveTree(t *testing.T) {
	modified := time.Unix(1234567890, 0)
	for _, compression := range []string{"plain", "gzip", "zstd"} {
		t.Run(compression, func(t *testing.T) {
			t.Chdir(t.TempDir())
			layer := tarLayer(t, compression,
				tarEntry{header: tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o750, ModTime: modified}},
				tarEntry{header: tar.Header{Name: "scripts/", Typeflag: tar.TypeDir, Mode: 0o750, ModTime: modified}},
				tarEntry{header: tar.Header{Name: "scripts/check.sh", Typeflag: tar.TypeReg, Mode: 0o751, ModTime: modified}, data: "#!/bin/sh\necho test\n"},
				tarEntry{header: tar.Header{Name: "references/deep/", Typeflag: tar.TypeDir, Mode: 0o700}},
				tarEntry{header: tar.Header{Name: "references/deep/.config", Typeflag: tar.TypeReg, Mode: 0o640}, data: "settings"},
				tarEntry{header: tar.Header{Name: "empty/", Typeflag: tar.TypeDir, Mode: 0o700}},
				tarEntry{header: tar.Header{Name: "check", Typeflag: tar.TypeSymlink, Linkname: "scripts/check.sh"}},
				tarEntry{header: tar.Header{Name: "check-hard", Typeflag: tar.TypeLink, Linkname: "scripts/check.sh", Mode: 0o751, ModTime: modified}},
			)
			if err := installLayers(layer); err != nil {
				t.Fatal(err)
			}
			const base = ".agents/skills/test/"
			for path, want := range map[string]string{"scripts/check.sh": "#!/bin/sh\necho test\n", "references/deep/.config": "settings"} {
				data, err := os.ReadFile(base + path)
				if err != nil || string(data) != want {
					t.Errorf("%s: got %q, %v; want %q", path, data, err, want)
				}
			}
			for path, mode := range map[string]os.FileMode{".": 0o750, "scripts": 0o750, "scripts/check.sh": 0o751, "references/deep/.config": 0o640, "empty": 0o700} {
				info, err := os.Stat(base + path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != mode {
					t.Errorf("%s: got permissions %o, want %o", path, info.Mode().Perm(), mode)
				}
			}
			for _, path := range []string{"scripts", "scripts/check.sh"} {
				info, err := os.Stat(base + path)
				if err != nil || !info.ModTime().Equal(modified) {
					t.Errorf("%s: modification time not preserved: %v, %v", path, info, err)
				}
			}
			if link, err := os.Readlink(base + "check"); err != nil || link != "scripts/check.sh" {
				t.Errorf("symlink: got %q, %v", link, err)
			}
			first, err := os.Stat(base + "scripts/check.sh")
			if err != nil {
				t.Fatal(err)
			}
			second, err := os.Stat(base + "check-hard")
			if err != nil || !os.SameFile(first, second) {
				t.Errorf("hard link not preserved: %v", err)
			}
		})
	}
}

func TestTarLayersApplyInOrder(t *testing.T) {
	t.Chdir(t.TempDir())
	first := tarLayer(t, "gzip",
		tarEntry{header: tar.Header{Name: "references/", Typeflag: tar.TypeDir, Mode: 0o500}},
		tarEntry{header: tar.Header{Name: "references/value", Typeflag: tar.TypeReg, Mode: 0o600}, data: "old"},
	)
	second := tarLayer(t, "plain", tarEntry{header: tar.Header{Name: "references/value", Typeflag: tar.TypeReg, Mode: 0o640}, data: "new"})
	if err := installLayers(first, second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(".agents/skills/test/references/value")
	if err != nil || string(data) != "new" {
		t.Fatalf("later layer did not replace the file: %q, %v", data, err)
	}
}

func TestTarLayersConfineWrites(t *testing.T) {
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
			err = installLayers(tarLayer(t, "gzip", entries...))
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

func TestTarLayersPreserveSymlinkTargets(t *testing.T) {
	t.Chdir(t.TempDir())
	links := map[string]string{
		"relative": "../outside", "absolute": "/tmp/outside",
		"dangling": "missing/value", "cycle": "cycle", "alias": ".",
		"chain": "alias/../outside",
	}
	var entries []tarEntry
	for name, target := range links {
		entries = append(entries, tarEntry{header: tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target}})
	}
	if err := installLayers(tarLayer(t, "gzip", entries...)); err != nil {
		t.Fatal(err)
	}
	for name, want := range links {
		got, err := os.Readlink(".agents/skills/test/" + name)
		if err != nil || got != want {
			t.Errorf("symlink %s changed: %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestTarLayersRejectInvalidData(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		data []byte
		want error
	}{
		{[]byte("raw file"), io.ErrUnexpectedEOF},
		{bytes.Repeat([]byte("x"), 1024), tar.ErrHeader},
	} {
		if err := installLayers(tc.data); !errors.Is(err, tc.want) {
			t.Errorf("expected %v, got %v", tc.want, err)
		}
	}
}
