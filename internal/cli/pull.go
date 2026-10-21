package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
	"github.com/vrothberg/skillctl/internal/store"
)

func pullCommand() *cobra.Command {
	var (
		name            string
		force           bool
		signaturePolicy string
		registriesDir   string
		tlsVerify       bool
	)

	cmd := &cobra.Command{
		Use:   "pull IMAGE",
		Short: "Pull a skill from an OCI registry and extract it locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			ctx := context.Background()

			manifestPath := filepath.Join(".", manifest.DefaultFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
			}

			storePath, err := store.DefaultStorePath()
			if err != nil {
				return err
			}

			s, err := store.NewWithOptions(storePath, store.StoreOptions{
				SignaturePolicyPath:    signaturePolicy,
				RegistriesDirPath:     registriesDir,
				InsecureSkipTLSVerify: !tlsVerify,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Pulling %s ...\n", imageRef)
			d, err := s.Pull(ctx, imageRef)
			if err != nil {
				return err
			}

			if name == "" {
				name = skillName(imageRef)
			}

			if err := manifest.ValidateSkillName(name); err != nil {
				return err
			}

			if !force {
				if existing := m.Find(name); existing != nil && existing.Digest == d.String() {
					fmt.Printf("%s is up to date (%s)\n", name, d.String()[:19])
					return nil
				}
			}

			extractPath := filepath.Join(".", manifest.SkillExtractPath(name))

			if err := os.RemoveAll(extractPath); err != nil {
				return fmt.Errorf("removing old skill files: %w", err)
			}

			if err := s.Extract(ctx, imageRef, extractPath); err != nil {
				return err
			}

			m.AddOrUpdate(manifest.Skill{
				Name:        name,
				Image:       imageRef,
				Digest:      d.String(),
				ExtractPath: manifest.SkillExtractPath(name),
			})

			if err := m.Save(manifestPath); err != nil {
				_ = os.RemoveAll(extractPath)
				return fmt.Errorf("updating manifest: %w", err)
			}

			fmt.Printf("Pulled %s (%s)\n", name, d.String()[:19])
			fmt.Printf("Extracted to %s\n", extractPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "override the skill name (default: derived from image reference)")
	cmd.Flags().BoolVar(&force, "force", false, "force pull and extraction even if the digest is unchanged")
	cmd.Flags().StringVar(&signaturePolicy, "signature-policy", "", "path to a signature policy.json file")
	cmd.Flags().StringVar(&registriesDir, "registries-dir", "", "path to a registries.d directory")
	cmd.Flags().BoolVar(&tlsVerify, "tls-verify", true, "require HTTPS and verify TLS certificates")

	return cmd
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
