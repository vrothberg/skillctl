package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/manifest"
)

func listCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List skills tracked in the manifest",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := filepath.Join(".", manifest.DefaultFile)
			m, err := manifest.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("loading manifest (run 'skillctl init' first): %w", err)
			}

			if len(m.Skills) == 0 {
				fmt.Println("No skills tracked. Use 'skillctl pull' to add one.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "NAME\tIMAGE\tDIGEST\tPATH\tTRUST"); err != nil {
				return err
			}
			for _, s := range m.Skills {
				shortDigest := s.Digest
				if len(shortDigest) > 19 {
					shortDigest = shortDigest[:19]
				}
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Image, shortDigest, s.ExtractPath, s.Trust.String()); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}
}
