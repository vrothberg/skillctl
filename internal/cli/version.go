package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/version"
)

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("skillctl %s (%s)\n", version.Version, version.Commit)
		},
	}
}
