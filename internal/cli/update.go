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
	var (
		force     bool
		tlsVerify bool
	)

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

			// Trust comes from the manifest and only the manifest. Refuse
			// to touch anything before every skill declares how it is
			// verified, so a partial manifest never partially updates.
			for _, skill := range m.Skills {
				if !skill.Trust.Declared() {
					return fmt.Errorf("skill %q has no trust declaration in the manifest (re-pull it with --trust or add a trust entry to %s)", skill.Name, manifest.DefaultFile)
				}
			}

			storePath, err := store.Path(storeFlag)
			if err != nil {
				return err
			}

			s, err := store.NewWithOptions(storePath, store.StoreOptions{
				Skills:                m.Skills,
				ManifestDir:           filepath.Dir(manifestPath),
				InsecureSkipTLSVerify: !tlsVerify,
			})
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			return store.WithLock(storePath, func() error {
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
						Trust:       skill.Trust,
					})

					if err := m.Save(manifestPath); err != nil {
						return fmt.Errorf("updating manifest: %w", err)
					}

					fmt.Printf("  Updated %s (%s)\n", skill.Name, d.String()[:19])
					updated++
				}

				fmt.Printf("Updated %d of %d skill(s)\n", updated, len(m.Skills))
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force extraction even if digests are unchanged")
	cmd.Flags().BoolVar(&tlsVerify, "tls-verify", true, "require HTTPS and verify TLS certificates")

	return cmd
}
