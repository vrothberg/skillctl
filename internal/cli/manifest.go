package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/vrothberg/skillctl/internal/manifest"
)

// loadManifest loads skillctl.yaml, pointing first-time users at init when the
// file does not exist while surfacing parse and validation errors as they are.
func loadManifest(path string) (*manifest.Manifest, error) {
	m, err := manifest.Load(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
		}
		return nil, fmt.Errorf("loading manifest: %w", err)
	}
	return m, nil
}
