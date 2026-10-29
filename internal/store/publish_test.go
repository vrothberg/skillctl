package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/signature/sigstore"
)

func TestPublishReference(t *testing.T) {
	const hash = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, tc := range []struct{ input, want string }{
		{"quay.io/team/skill:v1", ""},
		{"localhost:5000/team/skill:v1", ""},
		{"quay.io/team/skill", "explicit tag"},
		{"localhost:5000/skill", "explicit tag"},
		{"quay.io/team/skill" + hash, "must not contain a digest"},
		{"quay.io/team/skill:v1" + hash, "must not contain a digest"},
		{":::invalid:::", "parsing publish reference"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			ref, err := ParsePublishReference(tc.input)
			if tc.want == "" {
				if err != nil || ref.String() != tc.input {
					t.Fatalf("got %v, %v; want %q", ref, err, tc.input)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
	ref, err := ParsePublishReference("team/skill:v1")
	if err != nil || ref.String() != "docker.io/team/skill:v1" {
		t.Fatalf("short reference must normalize to a copyable destination: %v, %v", ref, err)
	}
}

func packaged(t *testing.T, directory string) []byte {
	t.Helper()
	var result bytes.Buffer
	if err := packageDirectory(context.Background(), directory, &result); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

func stagedManifest(t *testing.T, layer []byte) []byte {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "layer.tar.gz")
	if err := os.WriteFile(file, layer, 0o600); err != nil {
		t.Fatal(err)
	}
	const name = "quay.io/team/skill:v1"
	path := filepath.Join(dir, "store")
	if err := stageArtifact(context.Background(), path, name, file); err != nil {
		t.Fatal(err)
	}
	ref, err := layout.NewReference(path, name)
	if err != nil {
		t.Fatal(err)
	}
	source, err := ref.NewImageSource(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	raw, _, err := source.GetManifest(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPublishReproducibility(t *testing.T) {
	var previousLayer, previousManifest []byte
	for version := range 2 {
		dir := filepath.Join(t.TempDir(), "skill")
		if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
			t.Fatal(err)
		}
		names := []string{"z.txt", ".hidden", "nested/value.txt"}
		if version == 1 {
			names[0], names[2] = names[2], names[0]
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(name), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Link(filepath.Join(dir, "z.txt"), filepath.Join(dir, "hardlink")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("nested/value.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		// The container runs as root, allowing ownership variation as well
		// as timestamp, creation-order, and absolute source-path variation.
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if os.Geteuid() == 0 {
				if err := os.Lchown(path, 1000+version, 2000+version); err != nil {
					return err
				}
			}
			if entry.Type()&os.ModeSymlink == 0 {
				stamp := time.Unix(int64(1234567890+version*10000), 0)
				return os.Chtimes(path, stamp, stamp)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		layer := packaged(t, dir)
		manifest := stagedManifest(t, layer)
		if version == 1 {
			if !bytes.Equal(layer, previousLayer) {
				t.Error("different metadata or creation order changed the compressed layer")
			}
			if !bytes.Equal(manifest, previousManifest) {
				t.Errorf("artifact digest changed: %s != %s", digest.FromBytes(manifest), digest.FromBytes(previousManifest))
			}
		}
		previousLayer, previousManifest = layer, manifest
	}
	stream, err := gzip.NewReader(bytes.NewReader(previousLayer))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	archive := tar.NewReader(stream)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" || !header.ModTime.Equal(time.Unix(0, 0)) {
			t.Errorf("non-normalized metadata for %s: %+v", header.Name, header)
		}
	}
}

func TestPackagePreservesContents(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(source, ".hidden", "empty"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".hidden", "run"), []byte("#!/bin/sh\necho yes\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte(".hidden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	longName := strings.Repeat("long", 30)
	if err := os.WriteFile(filepath.Join(source, longName), []byte("PAX path"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".hidden/run", filepath.Join(source, "run")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(source, ".hidden", "run"), filepath.Join(source, "hardlink")); err != nil {
		t.Fatal(err)
	}
	layer := packaged(t, source)
	t.Chdir(t.TempDir())
	if err := installLayers(layer); err != nil {
		t.Fatal(err)
	}
	const installed = ".agents/skills/test/"
	data, err := os.ReadFile(installed + ".hidden/run")
	if err != nil || string(data) != "#!/bin/sh\necho yes\n" {
		t.Fatalf("contents changed: %q, %v", data, err)
	}
	if data, err := os.ReadFile(installed + longName); err != nil || string(data) != "PAX path" {
		t.Fatalf("long archive path changed: %q, %v", data, err)
	}
	for name, mode := range map[string]os.FileMode{".hidden/run": 0o751, ".hidden/empty": 0o750, ".gitignore": 0o644} {
		info, err := os.Stat(installed + name)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s: want mode %o, got %v, %v", name, mode, info, err)
		}
	}
	if link, err := os.Readlink(installed + "run"); err != nil || link != ".hidden/run" {
		t.Fatalf("symlink changed: %q, %v", link, err)
	}
	first, err := os.Stat(installed + "hardlink")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(installed + ".hidden/run")
	if err != nil || !os.SameFile(first, second) {
		t.Fatalf("hard link not preserved: %v", err)
	}
}

func TestPackageRejectsUnsupportedEntries(t *testing.T) {
	for _, target := range []string{"../outside", "/tmp/outside"} {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			err := packageDirectory(context.Background(), dir, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "escapes the skill directory") {
				t.Fatalf("expected escaping-link error, got %v", err)
			}
		})
	}
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := packageDirectory(context.Background(), dir, io.Discard); err == nil || !strings.Contains(err.Error(), "unsupported source entry") {
		t.Fatalf("expected unsupported-entry error, got %v", err)
	}
}

func TestParseSign(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   SignOptions
		error  string
	}{
		{[]string{"none"}, SignOptions{None: true}, ""},
		{[]string{"key=key"}, SignOptions{Key: "key"}, ""},
		{[]string{"passphrase-file=pass", "key=key"}, SignOptions{Key: "key", PassphraseFile: "pass"}, ""},
		{[]string{"key=a=b"}, SignOptions{Key: "a=b"}, ""},
		{nil, SignOptions{}, "no signing declaration"},
		{[]string{"passphrase-file=pass"}, SignOptions{}, "requires --sign key"},
		{[]string{"none", "key=key"}, SignOptions{}, "cannot be combined"},
		{[]string{"none", "passphrase-file=pass"}, SignOptions{}, "cannot be combined"},
		{[]string{"none", "none"}, SignOptions{}, "duplicate"},
		{[]string{"key=a", "key=b"}, SignOptions{}, "duplicate"},
		{[]string{"passphrase-file=a", "passphrase-file=b"}, SignOptions{}, "duplicate"},
		{[]string{"key="}, SignOptions{}, "invalid --sign option"},
		{[]string{"passphrase-file="}, SignOptions{}, "invalid --sign option"},
		{[]string{"none=true"}, SignOptions{}, "invalid --sign option"},
		{[]string{"unknown=foo"}, SignOptions{}, "invalid --sign option"},
	} {
		t.Run(strings.Join(tc.values, ","), func(t *testing.T) {
			got, err := ParseSign(tc.values)
			if tc.error == "" {
				if err != nil || got != tc.want {
					t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("expected %q error, got %v", tc.error, err)
			}
		})
	}
}

func TestSignerWithEmptyPassphrase(t *testing.T) {
	keys, err := sigstore.GenerateKeyPair([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, keys.PrivateKey, 0o600); err != nil {
		t.Fatal(err)
	}
	signing, err := (SignOptions{Key: path}).signer()
	if err != nil {
		t.Fatalf("key with an empty passphrase must not prompt: %v", err)
	}
	if err := signing.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSignerWithUnencryptedKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: data}), 0o600); err != nil {
		t.Fatal(err)
	}
	signing, err := (SignOptions{Key: path}).signer()
	if err != nil {
		t.Fatalf("unencrypted key must work without a passphrase file: %v", err)
	}
	if err := signing.Close(); err != nil {
		t.Fatal(err)
	}
}
