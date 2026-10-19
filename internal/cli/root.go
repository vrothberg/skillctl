package cli

import (
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skillctl",
		Short: "Manage AI agent skills as OCI artifacts",
		Long:  "skillctl pulls, verifies, and manages AI agent skills as OCI artifacts with the same supply-chain guarantees as container images.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		initCommand(),
		pullCommand(),
		listCommand(),
	)

	return cmd
}
