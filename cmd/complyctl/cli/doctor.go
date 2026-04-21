// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/doctor"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/registry"
)

func doctorCmd(common *Common) *cobra.Command {
	_ = common
	var verbose bool
	cmd := &cobra.Command{
		Use:               "doctor",
		Short:             "Run pre-flight diagnostics on the workspace",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDoctor(verbose)
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "expand per-provider variable detail")
	return cmd
}

// registryVersionResolver adapts registry.Client to doctor.VersionResolver.
// See R55: specs/001-gemara-native-workflow/spec.md
type registryVersionResolver struct {
	timeout time.Duration
}

func (r *registryVersionResolver) ResolveLatestVersion(registryURL, repository string) (string, error) {
	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		credFunc = nil
	}
	client := registry.NewClient(registryURL, credFunc)
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	_, version, err := client.DefinitionVersion(ctx, repository)
	if err != nil {
		return "", err
	}
	return version, nil
}

// See FR-039, R44, R51, R52, R55: specs/001-gemara-native-workflow/spec.md
func runDoctor(verbose bool) error {
	providerDir, err := complytime.ResolveProviderDir()
	if err != nil {
		return fmt.Errorf("failed to resolve provider directory: %w", err)
	}

	cacheDir, err := complytime.ResolveCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	configPath := complytime.WorkspaceConfigFile
	var cfg *complytime.WorkspaceConfig

	loaded, loadErr := complytime.LoadFrom(configPath)
	if loadErr == nil {
		cfg = loaded
	}

	var resolver doctor.PolicyGraphResolver
	cacheMgr := cache.NewCache(cacheDir)
	loader := policy.NewLoader(cacheMgr)
	resolver = policy.NewResolver(loader)

	versionResolver := &registryVersionResolver{timeout: 5 * time.Second}

	results := doctor.Run(cfg, configPath, providerDir, cacheDir, resolver, versionResolver, verbose, logger)

	fmt.Println("Running workspace diagnostics...")
	fmt.Println()

	var passCount, failCount, warnCount int
	hasBlockingFailure := false
	for _, r := range results {
		var emoji string
		switch r.Status {
		case doctor.StatusPass:
			emoji = complytime.StatusPassed
			passCount++
		case doctor.StatusFail:
			emoji = complytime.StatusFailed
			failCount++
		case doctor.StatusWarn:
			emoji = complytime.StatusSkipped
			warnCount++
		}
		fmt.Printf("%s %s: %s\n", emoji, r.Name, r.Message)
		if r.Blocking && r.Status == doctor.StatusFail {
			hasBlockingFailure = true
		}
	}

	total := passCount + failCount + warnCount
	fmt.Printf("\n%d checks: %d passed, %d failed, %d warnings\n", total, passCount, failCount, warnCount)

	if hasBlockingFailure {
		return fmt.Errorf("one or more blocking checks failed")
	}
	return nil
}
