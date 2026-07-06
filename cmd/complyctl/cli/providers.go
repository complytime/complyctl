// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/provider"
)

type providersOptions struct {
	*Common
	providerDir string
	cacheDir    string
}

func providersCmd(common *Common) *cobra.Command {
	o := &providersOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List discovered scanning providers and their health status",
		Long: `List discovered scanning providers and their health status.

The output includes a COMPLYPACK column showing the cached complypack version
for each provider. Providers without a cached complypack display "none".`,
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
	if o.cacheDir == "" {
		o.cacheDir, err = complytime.ResolveCacheDir()
		if err != nil {
			return fmt.Errorf("failed to resolve cache directory: %w", err)
		}
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

	cc := cache.NewComplypackCache(o.cacheDir, nil)
	rows := buildProviderRows(ctx, providers, o.providerDir, cc)
	headers := []string{"PROVIDER ID", "PATH", "STATUS", "VERSION", "COMPLYPACK"}
	terminal.ShowPlainTable(os.Stdout, headers, rows)
	return nil
}

func buildProviderRows(ctx context.Context, providers []*provider.LoadedProvider, providerDir string, cc *cache.ComplypackCache) [][]string {
	rows := make([][]string, 0, len(providers))
	for _, lp := range providers {
		status, version := describeProvider(ctx, lp)
		relPath, relErr := filepath.Rel(providerDir, lp.Info.ExecutablePath)
		if relErr != nil {
			relPath = lp.Info.ExecutablePath
		}
		packVersion := lookupComplypackVersion(cc, lp.Info.EvaluatorID)
		rows = append(rows, []string{lp.Info.EvaluatorID, relPath, status, version, packVersion})
	}
	return rows
}

// lookupComplypackVersion returns the cached complypack version for the given
// evaluator-id, or "none" if no complypack is cached. Errors during lookup
// are treated as "none" — the providers table is informational and should not
// fail due to cache read issues.
func lookupComplypackVersion(cc *cache.ComplypackCache, evaluatorID string) string {
	_, cfg, err := cc.LookupByEvaluatorID(evaluatorID)
	if err != nil || cfg == nil || cfg.Version == "" {
		return "none"
	}
	return cfg.Version
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
