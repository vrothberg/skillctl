package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
	"github.com/vrothberg/skillctl/internal/store"
)

func pullCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull IMAGE",
		Short: "Pull a skill from an OCI registry and extract it locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			ctx := context.Background()

			storePath, err := store.DefaultStorePath()
			if err != nil {
				return err
			}

			s, err := store.New(storePath)
			if err != nil {
				return err
			}

			fmt.Printf("Pulling %s ...\n", imageRef)
			d, err := s.Pull(ctx, imageRef)
			if err != nil {
				return err
			}

			name := skillName(imageRef)
			extractPath := filepath.Join(".", manifest.SkillExtractPath(name))

			if err := s.Extract(ctx, imageRef, extractPath); err != nil {
				return err
			}

			manifestPath := filepath.Join(".", manifest.DefaultFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
			}

			m.AddOrUpdate(manifest.Skill{
				Name:        name,
				Image:       imageRef,
				Digest:      d.String(),
				ExtractPath: manifest.SkillExtractPath(name),
			})

			if err := m.Save(manifestPath); err != nil {
				return fmt.Errorf("updating manifest: %w", err)
			}

			fmt.Printf("Pulled %s (%s)\n", name, d.Encoded()[:12])
			fmt.Printf("Extracted to %s\n", extractPath)
			return nil
		},
	}
}

func skillName(imageRef string) string {
	// quay.io/redhat/skills/translator:1.0 -> translator
	ref := imageRef
	if at := strings.LastIndex(ref, "@"); at != -1 {
		ref = ref[:at]
	}
	if colon := strings.LastIndex(ref, ":"); colon != -1 {
		ref = ref[:colon]
	}
	if slash := strings.LastIndex(ref, "/"); slash != -1 {
		ref = ref[slash+1:]
	}
	return ref
}
