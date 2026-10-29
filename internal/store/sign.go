package store

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/secure-systems-lab/go-securesystemslib/encrypted"
	"go.podman.io/image/v5/signature/signer"
	"go.podman.io/image/v5/signature/sigstore"
)

type SignOptions struct {
	Key            string
	PassphraseFile string
	None           bool
}

func ParseSign(values []string) (SignOptions, error) {
	var result SignOptions
	seen := map[string]bool{}
	for _, value := range values {
		name, argument, hasValue := strings.Cut(value, "=")
		if seen[name] {
			return result, fmt.Errorf("duplicate --sign option %q", name)
		}
		seen[name] = true
		switch {
		case name == "none" && !hasValue:
			result.None = true
		case name == "key" && hasValue && argument != "":
			result.Key = argument
		case name == "passphrase-file" && hasValue && argument != "":
			result.PassphraseFile = argument
		default:
			return result, fmt.Errorf("invalid --sign option %q: use key=PRIVATE_KEY, passphrase-file=FILE, or none", name)
		}
	}
	return result, result.validate()
}

func (s SignOptions) validate() error {
	if s.None && (s.Key != "" || s.PassphraseFile != "") {
		return fmt.Errorf("--sign none cannot be combined with other signing options")
	}
	if s.PassphraseFile != "" && s.Key == "" {
		return fmt.Errorf("--sign passphrase-file=FILE requires --sign key=PRIVATE_KEY")
	}
	if !s.None && s.Key == "" {
		return fmt.Errorf("no signing declaration: specify --sign key=PRIVATE_KEY or --sign none")
	}
	return nil
}

func (s SignOptions) signer() (*signer.Signer, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if s.None {
		return nil, nil
	}
	// A non-nil empty passphrase handles keys with an empty password without
	// ever asking the library to prompt for one.
	passphrase := []byte{}
	if s.PassphraseFile != "" {
		data, err := os.ReadFile(s.PassphraseFile)
		if err != nil {
			return nil, fmt.Errorf("reading signing passphrase file: %w", err)
		}
		passphrase = bytes.TrimRight(data, "\r\n")
	}
	keyPath := s.Key
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}
	// containers/image accepts encrypted sigstore keys. Adapt unencrypted
	// PKCS#8 keys to that format so they work without a passphrase file too.
	if block, _ := pem.Decode(key); block != nil && block.Type == "PRIVATE KEY" {
		wrapped, err := encrypted.Encrypt(block.Bytes, passphrase)
		if err != nil {
			return nil, fmt.Errorf("preparing signing key: %w", err)
		}
		file, err := os.CreateTemp("", "skillctl-signing-key-")
		if err != nil {
			return nil, err
		}
		defer func() { _ = os.Remove(file.Name()) }()
		encodeErr := pem.Encode(file, &pem.Block{Type: "ENCRYPTED SIGSTORE PRIVATE KEY", Bytes: wrapped})
		closeErr := file.Close()
		if encodeErr != nil {
			return nil, encodeErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		keyPath = file.Name()
	}
	result, err := sigstore.NewSigner(sigstore.WithPrivateKeyFile(keyPath, passphrase))
	if err != nil {
		if s.PassphraseFile == "" {
			return nil, fmt.Errorf("loading signing key (if encrypted, supply --sign passphrase-file=FILE): %w", err)
		}
		return nil, fmt.Errorf("loading signing key: %w", err)
	}
	return result, nil
}
