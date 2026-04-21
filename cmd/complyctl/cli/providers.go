// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/provider"
)

type providersOptions struct {
	*Common
	providerDir string
}

func providersCmd(common *Common) *cobra.Command {
	o := &providersOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:               "providers",
		Short:             "List discovered scanning providers and their health status",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := o.complete(); err != nil {
				return err
			}
			return o.run(cmd.Context())
		},
	}
	return cmd
}

func (o *providersOptions) complete() error {
	var err error
	o.providerDir, err = complytime.ResolveProviderDir()
	if err != nil {
		return fmt.Errorf("failed to resolve provider directory: %w", err)
	}
	return nil
}

func (o *providersOptions) run(ctx context.Context) error {
	mgr, err := provider.NewManager(o.providerDir, logger)
	if err != nil {
		return fmt.Errorf("provider manager init failed: %w", err)
	}
	defer mgr.Cleanup()

	if err := mgr.LoadProviders(); err != nil {
		return fmt.Errorf("provider discovery failed: %w", err)
	}

	providers := mgr.ListProviders()
	if len(providers) == 0 {
		fmt.Fprintf(os.Stderr, "No scanning providers found in %s\n", o.providerDir)
		return nil
	}

	rows := buildProviderRows(ctx, providers, o.providerDir)
	headers := []string{"PROVIDER ID", "PATH", "STATUS", "VERSION"}
	terminal.ShowPlainTable(os.Stdout, headers, rows)
	return nil
}

func buildProviderRows(ctx context.Context, providers []*provider.LoadedProvider, providerDir string) [][]string {
	rows := make([][]string, 0, len(providers))
	for _, lp := range providers {
		status, version := describeProvider(ctx, lp)
		relPath, relErr := filepath.Rel(providerDir, lp.Info.ExecutablePath)
		if relErr != nil {
			relPath = lp.Info.ExecutablePath
		}
		rows = append(rows, []string{lp.Info.EvaluatorID, relPath, status, version})
	}
	return rows
}

func describeProvider(ctx context.Context, lp *provider.LoadedProvider) (string, string) {
	resp, err := lp.Client.Describe(ctx, &provider.DescribeRequest{})
	if err != nil {
		return "ERROR", ""
	}
	if !resp.Healthy {
		return "unhealthy", ""
	}
	return "healthy", resp.Version
}
