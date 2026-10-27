package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
	"github.com/vrothberg/skillctl/internal/skillfs"
	"github.com/vrothberg/skillctl/internal/store"
)

func removeCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove NAME",
		Aliases: []string{"rm"},
		Short:   "Remove a skill from the manifest and delete its extracted files",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := manifest.ValidateSkillName(name); err != nil {
				return err
			}

			storePath, err := store.Path(storeFlag)
			if err != nil {
				return err
			}

			return store.WithLock(storePath, func() error {
				manifestPath := filepath.Join(".", manifest.DefaultFile)
				m, err := manifest.Load(manifestPath)
				if err != nil {
					return fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
				}

				if !m.Remove(name) {
					if force {
						return nil
					}
					return fmt.Errorf("skill %q not found in manifest", name)
				}

				extractPath := filepath.Join(".", manifest.SkillExtractPath(name))
				if err := skillfs.RemoveAll(extractPath); err != nil {
					return fmt.Errorf("removing extracted files: %w", err)
				}
				if err := m.Save(manifestPath); err != nil {
					return fmt.Errorf("updating manifest: %w", err)
				}

				fmt.Printf("Removed %s\n", name)
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "do not error if the skill does not exist")

	return cmd
}
