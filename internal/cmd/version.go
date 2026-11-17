package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/version"
)

func versionString() string { return version.String() }

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionString())

			return err
		},
	}
}
