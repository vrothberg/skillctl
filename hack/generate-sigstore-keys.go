// One-shot script to generate sigstore key pair for testing.
// Run: go run -mod=vendor hack/generate-sigstore-keys.go
package main

import (
	"fmt"
	"os"

	"go.podman.io/image/v5/signature/sigstore"
)

func main() {
	passphrase := []byte("test")

	result, err := sigstore.GenerateKeyPair(passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generating key pair: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("test/testdata", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "creating directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("test/testdata/sigstore.key", result.PrivateKey, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "writing private key: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("test/testdata/sigstore.pub", result.PublicKey, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing public key: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("test/testdata/sigstore.pass", passphrase, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing passphrase: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated test/testdata/sigstore.{key,pub,pass}")
}
