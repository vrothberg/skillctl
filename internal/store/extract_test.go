package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
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
		directories := map[string]tarMetadata{".": {mode: 0o755}}
		for _, layer := range layers {
			if err := unpackLayer(context.Background(), root, bytes.NewReader(layer), directories); err != nil {
				return err
			}
		}
		return finishDirectories(root, directories)
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
				tarEntry{header: tar.Header{Name: "check-hard", Typeflag: tar.TypeLink, Linkname: "scripts/check.sh"}},
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

func TestTarLayersRejectUnsafeEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header tar.Header
		want   string
	}{
		{"traversal", tar.Header{Name: "../escape", Typeflag: tar.TypeReg}, "escapes"},
		{"absolute", tar.Header{Name: "/tmp/escape", Typeflag: tar.TypeReg}, "escapes"},
		{"symlink", tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "../escape"}, "escapes"},
		{"absolute symlink", tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "/tmp/escape"}, "escapes"},
		{"hardlink", tar.Header{Name: "escape", Typeflag: tar.TypeLink, Linkname: "../escape"}, "escapes"},
		{"root file", tar.Header{Name: ".", Typeflag: tar.TypeReg}, "archive root must be a directory"},
		{"device", tar.Header{Name: "device", Typeflag: tar.TypeChar}, "unsupported tar entry"},
		{"fifo", tar.Header{Name: "pipe", Typeflag: tar.TypeFifo}, "unsupported tar entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll(".agents/skills/test", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(".agents/skills/test/SKILL.md", []byte("old skill"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := installLayers(tarLayer(t, "gzip", tarEntry{header: tc.header}))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			data, err := os.ReadFile(".agents/skills/test/SKILL.md")
			if err != nil || string(data) != "old skill" {
				t.Fatalf("failed extraction changed installed skill: %q, %v", data, err)
			}
		})
	}
}

func TestTarLayerRejectsInvalidData(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, data := range [][]byte{[]byte("raw file"), bytes.Repeat([]byte("x"), 1024)} {
		err := installLayers(data)
		if err == nil || !strings.Contains(err.Error(), "invalid tar archive") {
			t.Errorf("expected invalid-tar error, got %v", err)
		}
	}
	data := tarLayer(t, "gzip", tarEntry{header: tar.Header{Name: "file", Typeflag: tar.TypeReg}, data: "data"})
	data[len(data)-8] ^= 0xff
	if err := installLayers(data); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("expected compression-checksum error, got %v", err)
	}
}

func TestTarLayerRejectsSymlinkParent(t *testing.T) {
	t.Chdir(t.TempDir())
	layer := tarLayer(t, "gzip",
		tarEntry{header: tar.Header{Name: "alias", Typeflag: tar.TypeSymlink, Linkname: "."}},
		tarEntry{header: tar.Header{Name: "alias/escape", Typeflag: tar.TypeSymlink, Linkname: "../outside"}},
	)
	if err := installLayers(layer); err == nil || !strings.Contains(err.Error(), "symlink parent") {
		t.Fatalf("expected symlink-parent error, got %v", err)
	}
	if _, err := os.Lstat(".agents/skills/test"); !os.IsNotExist(err) {
		t.Fatalf("failed extraction installed a skill: %v", err)
	}
}
