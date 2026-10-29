package cli

import (
	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/locking"
	"github.com/vrothberg/skillctl/internal/manifest"
	"github.com/vrothberg/skillctl/internal/store"
)

// storeFlag holds the --store override, set on the root command so every
// subcommand can use it.
var storeFlag string

func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "skillctl",
		Short:         "Manage AI agent skills as OCI artifacts",
		Long:          "skillctl publishes, pulls, verifies, and manages AI agent skills as OCI artifacts with the same supply-chain guarantees as container images.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&storeFlag, "store", "", "path to the skillctl store (default: ~/.local/share/skillctl/store, or "+store.StoreEnvVar+")")

	cmd.AddCommand(
		withProjectLock(initCommand()),
		withProjectLock(pullCommand()),
		publishCommand(),
		listCommand(),
		withProjectLock(updateCommand()),
		withProjectLock(removeCommand()),
		versionCommand(),
	)

	return cmd
}

// Lock before RunE reads the manifest or selects trust declarations. All
// project writers acquire this lock before the artifact-store lock, including
// invocations that use different stores for the same project.
func withProjectLock(cmd *cobra.Command) *cobra.Command {
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return locking.WithLock(manifest.DefaultFile+".lock", func() error {
			return run(cmd, args)
		})
	}
	return cmd
}
