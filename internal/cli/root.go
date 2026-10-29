package cli

import (
	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/store"
)

// storeFlag holds the --store override, set on the root command so every
// subcommand can use it.
var storeFlag string

func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skillctl",
		Short: "Manage AI agent skills as OCI artifacts",
		Long:  "skillctl publishes, pulls, verifies, and manages AI agent skills as OCI artifacts with the same supply-chain guarantees as container images.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&storeFlag, "store", "", "path to the skillctl store (default: ~/.local/share/skillctl/store, or "+store.StoreEnvVar+")")

	cmd.AddCommand(
		initCommand(),
		pullCommand(),
		publishCommand(),
		listCommand(),
		updateCommand(),
		removeCommand(),
		versionCommand(),
	)

	return cmd
}
