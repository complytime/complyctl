// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/complytime/complytime/cmd/complytime/option"
)

// New creates a new cobra.Command root for ComplyTime
func New() *cobra.Command {
	o := option.Common{}

	o.IOStreams = option.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	cmd := &cobra.Command{
		Use:           "complytime [command]",
		SilenceErrors: false,
		SilenceUsage:  false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	o.BindFlags(cmd.PersistentFlags())
	cmd.AddCommand(versionCmd(&o))
	cmd.AddCommand(scanCmd(&o))
	cmd.AddCommand(generateCmd(&o))

	return cmd
}
