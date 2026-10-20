package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
	"github.com/vrothberg/skillctl/internal/store"
)

func updateCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull and extract all skills listed in the manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			manifestPath := filepath.Join(".", manifest.DefaultFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
			}

			if len(m.Skills) == 0 {
				fmt.Println("No skills in manifest")
				return nil
			}

			storePath, err := store.DefaultStorePath()
			if err != nil {
				return err
			}

			s, err := store.New(storePath)
			if err != nil {
				return err
			}

			var updated int
			for _, skill := range m.Skills {
				fmt.Printf("Checking %s ...\n", skill.Name)
				d, err := s.Pull(ctx, skill.Image)
				if err != nil {
					return fmt.Errorf("pulling %s: %w", skill.Name, err)
				}

				if !force && d.String() == skill.Digest {
					fmt.Printf("  %s is up to date\n", skill.Name)
					continue
				}

				extractPath := filepath.Join(".", manifest.SkillExtractPath(skill.Name))
				if err := s.Extract(ctx, skill.Image, extractPath); err != nil {
					return fmt.Errorf("extracting %s: %w", skill.Name, err)
				}

				m.AddOrUpdate(manifest.Skill{
					Name:        skill.Name,
					Image:       skill.Image,
					Digest:      d.String(),
					ExtractPath: manifest.SkillExtractPath(skill.Name),
				})

				fmt.Printf("  Updated %s (%s)\n", skill.Name, d.Encoded()[:12])
				updated++
			}

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("updating manifest: %w", err)
			}

			fmt.Printf("Updated %d of %d skill(s)\n", updated, len(m.Skills))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force extraction even if digests are unchanged")

	return cmd
}
