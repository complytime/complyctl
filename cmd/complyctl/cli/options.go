// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"

	"github.com/spf13/pflag"

	"github.com/complytime/complyctl/internal/complytime"
)

type Common struct {
	Debug     bool
	Workspace string
	Output
}

type Output struct {
	Out    io.Writer
	ErrOut io.Writer
}

func (o *Common) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVarP(&o.Debug, "debug", "d", false, "output debug logs to stderr and log file")
	fs.StringVarP(&o.Workspace, "workspace", "w", "", "workspace directory (env: COMPLYTIME_WORKSPACE, default: current directory)")
}

// ResolveWorkspace resolves the workspace directory using precedence rules:
// --workspace flag > COMPLYTIME_WORKSPACE env > current directory
func (o *Common) ResolveWorkspace() (string, error) {
	return complytime.ResolveWorkspaceDir(o.Workspace)
}
