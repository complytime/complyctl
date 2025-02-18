// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"

	"github.com/complytime/complytime/cmd/complytime/option"
	"github.com/complytime/complytime/pkg/log"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
)

var logger hclog.Logger

func enableDebug(opts *option.Common) {
	if opts.Debug {
		logger.SetLevel(hclog.Debug)
	}
}

// New creates a new cobra.Command root for ComplyTime
func New() *cobra.Command {

	logger = log.NewLogger(os.Stdout)

	cmd := &cobra.Command{
		Use:           "complytime [command]",
		SilenceErrors: true,
		SilenceUsage:  false,
	}

	opts := option.Common{
		Output: option.Output{
			Out:    cmd.OutOrStdout(),
			ErrOut: cmd.ErrOrStderr(),
		},
	}
	opts.BindFlags(cmd.PersistentFlags())

	cmd.AddCommand(
		versionCmd(&opts),
		scanCmd(&opts),
		generateCmd(&opts),
		planCmd(&opts),
		listCmd(&opts),
	)

	return cmd
}
