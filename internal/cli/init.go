package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
)

func initCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a skillctl project in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := filepath.Join(".", manifest.DefaultFile)

			if _, err := os.Stat(manifestPath); err == nil {
				return fmt.Errorf("%s already exists", manifest.DefaultFile)
			}

			m := manifest.New()
			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("creating manifest: %w", err)
			}

			skillDir := filepath.Join(".", manifest.DefaultSkillDir)
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				return fmt.Errorf("creating skills directory: %w", err)
			}

			fmt.Printf("Initialized skillctl project: %s and %s/\n", manifest.DefaultFile, manifest.DefaultSkillDir)
			return nil
		},
	}
}
