package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vrothberg/skillctl/internal/store"
)

func publishCommand() *cobra.Command {
	var sign []string
	var tlsVerify bool
	cmd := &cobra.Command{
		Use:   "publish IMAGE DIRECTORY",
		Short: "Package a directory and publish it as an OCI artifact",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			signing, err := store.ParseSign(sign)
			if err != nil {
				return err
			}
			image, err := store.ParsePublishReference(args[0])
			if err != nil {
				return err
			}
			d, err := store.Publish(cmd.Context(), image.String(), args[1], store.PublishOptions{
				Sign: signing, InsecureSkipTLSVerify: !tlsVerify,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published %s\nDigest: %s\nReference: %s@%s\n", image.String(), d, image.String(), d)
			return err
		},
	}
	cmd.Flags().StringArrayVar(&sign, "sign", nil, "signing declaration (repeatable): key=PRIVATE_KEY, passphrase-file=FILE, or none")
	cmd.Flags().BoolVar(&tlsVerify, "tls-verify", true, "require HTTPS and verify TLS certificates")
	return cmd
}
