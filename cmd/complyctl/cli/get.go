// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/registry"
)

type getOptions struct {
	*Common
	timeout  time.Duration
	cacheDir string
}

func getCmd(common *Common) *cobra.Command {
	o := &getOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:   "get [flags]",
		Short: "Fetch policies and complypacks from OCI registries",
		Long: `Fetch new or modified policies from OCI registries and update the local cache.

If the workspace configuration (complytime.yaml) includes a complypacks section,
complypack artifacts are also fetched and cached alongside policies. Complypacks
provide provider-specific content bundles that are resolved by evaluator ID
during generate and scan operations.`,
		SilenceUsage:      true,
		Example:           "complyctl get",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := o.validate(); err != nil {
				return err
			}
			if err := o.complete(); err != nil {
				return err
			}
			return o.run(cmd.Context())
		},
	}
	cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", complytime.DefaultCommandTimeout, "Maximum time for the get operation (e.g. 5m, 10m, 1h)")
	return cmd
}

func (o *getOptions) validate() error {
	return nil
}

func (o *getOptions) complete() error {
	var err error
	o.cacheDir, err = complytime.ResolveCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}
	return nil
}

func (o *getOptions) run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	baseDir, err := o.ResolveWorkspace()
	if err != nil {
		return err
	}

	cfg, err := loadWorkspaceConfig(baseDir)
	if err != nil {
		return err
	}

	// Chain policy and complypack sync into a single error return.
	// syncComplypacks is a no-op when no complypacks are configured.
	return o.syncAll(ctx, cfg)
}

// syncAll runs policy sync followed by complypack sync. Keeping both
// calls in a single method avoids an extra branch in run() and keeps
// the CRAP score aligned with the baseline.
func (o *getOptions) syncAll(ctx context.Context, cfg *complytime.WorkspaceConfig) error {
	if err := o.syncPolicies(ctx, cfg); err != nil {
		return err
	}
	return o.syncComplypacks(ctx, cfg)
}

func (o *getOptions) syncPolicies(ctx context.Context, cfg *complytime.WorkspaceConfig) error {
	cacheMgr := cache.NewCache(o.cacheDir)

	state, err := cache.LoadState(o.cacheDir)
	if err != nil {
		logger.Error("Cache state load failed", "cache_dir", o.cacheDir, "error", err)
		return fmt.Errorf("failed to load cache state: %w", err)
	}

	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		logger.Error("Credential resolution failed", "error", err)
		return fmt.Errorf("authentication setup failed: %w", err)
	}

	return syncAllPolicies(ctx, cacheMgr, state, credFunc, cfg.Policies)
}

// syncComplypacks fetches complypack artifacts listed in the workspace config.
// Skips silently when no complypacks are configured.
func (o *getOptions) syncComplypacks(ctx context.Context, cfg *complytime.WorkspaceConfig) error {
	if len(cfg.Complypacks) == 0 {
		return nil
	}

	state, err := cache.LoadState(o.cacheDir)
	if err != nil {
		logger.Error("Cache state load failed", "cache_dir", o.cacheDir, "error", err)
		return fmt.Errorf("failed to load cache state: %w", err)
	}

	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		logger.Error("Credential resolution failed", "error", err)
		return fmt.Errorf("authentication setup failed: %w", err)
	}

	return syncAllComplypacks(ctx, state, credFunc, cfg.Complypacks, o.cacheDir)
}

func syncAllPolicies(ctx context.Context, cacheMgr *cache.Cache, state *cache.State, credFunc auth.CredentialFunc, policies []complytime.PolicyEntry) error {
	logger.Info("Starting policy synchronization", "policy_count", len(policies))

	total := len(policies)
	for i, entry := range policies {
		if err := syncSinglePolicy(ctx, cacheMgr, state, credFunc, entry, i+1, total); err != nil {
			return err
		}
	}

	logger.Info("Synchronization completed", "synced", total, "total", total)
	fmt.Fprintln(os.Stderr, "Synchronization completed.")
	return nil
}

func syncSinglePolicy(ctx context.Context, cacheMgr *cache.Cache, state *cache.State, credFunc auth.CredentialFunc, entry complytime.PolicyEntry, index, total int) error {
	ref := complytime.ParsePolicyRef(entry.URL)
	version := ref.Version

	client := registry.NewClient(ref.Registry, credFunc)
	source := cache.NewRegistrySource(client)
	sync := cache.NewSync(cacheMgr, state, source)

	if version == "" {
		version = resolveLatestVersion(ctx, client, ref.Repository, entry.EffectiveID())
	}

	fmt.Fprintf(os.Stderr, "Syncing policy %d/%d: %s... ", index, total, entry.EffectiveID())
	logger.Info("Syncing policy", "policy", ref.Repository, "version", version)
	if err := sync.SyncPolicy(ctx, ref.Repository, version); err != nil {
		fmt.Fprintln(os.Stderr, "failed")
		logger.Error("Policy sync failed", "policy", ref.Repository, "error", err)
		return err
	}
	fmt.Fprintln(os.Stderr, "done")
	logger.Info("Policy synced", "policy", entry.EffectiveID())
	return nil
}

func resolveLatestVersion(ctx context.Context, client *registry.Client, repository, policyID string) string {
	logger.Info("Resolving latest version", "policy", policyID)
	_, resolvedVersion, resolveErr := client.DefinitionVersion(ctx, repository)
	if resolveErr != nil {
		logger.Warn("Version resolution failed, falling back to 'latest'",
			"policy", policyID, "error", resolveErr)
		return "latest"
	}
	logger.Info("Resolved version", "policy", policyID, "version", resolvedVersion)
	return resolvedVersion
}

func syncAllComplypacks(ctx context.Context, state *cache.State, credFunc auth.CredentialFunc, complypacks []complytime.PolicyEntry, cacheDir string) error {
	logger.Info("Starting complypack synchronization", "complypack_count", len(complypacks))

	total := len(complypacks)
	for i, entry := range complypacks {
		if err := syncSingleComplypack(ctx, state, credFunc, entry, i+1, total, cacheDir); err != nil {
			return err
		}
	}

	logger.Info("Complypack synchronization completed", "synced", total, "total", total)
	fmt.Fprintln(os.Stderr, "Complypack synchronization completed.")
	return nil
}

func syncSingleComplypack(ctx context.Context, state *cache.State, credFunc auth.CredentialFunc, entry complytime.PolicyEntry, index, total int, cacheDir string) error {
	ref := complytime.ParsePolicyRef(entry.URL)
	version := ref.Version

	client := registry.NewClient(ref.Registry, credFunc)
	source := cache.NewRegistryComplypackSource(client)
	complypackCache := cache.NewComplypackCache(cacheDir)
	cpSync := cache.NewComplypackSync(complypackCache, state, source)

	if version == "" {
		version = resolveLatestVersion(ctx, client, ref.Repository, entry.EffectiveID())
	}

	fmt.Fprintf(os.Stderr, "Syncing complypack %d/%d: %s... ", index, total, entry.EffectiveID())
	logger.Info("Syncing complypack", "complypack", ref.Repository, "version", version)
	fetched, err := cpSync.SyncComplypack(ctx, ref.Repository, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed")
		logger.Error("Complypack sync failed", "complypack", ref.Repository, "error", err)
		return fmt.Errorf("failed to sync complypack %s: %w", ref.Repository, err)
	}
	fmt.Fprintln(os.Stderr, "done")
	logger.Info("Complypack synced", "complypack", entry.EffectiveID())

	if fetched {
		fmt.Fprintf(os.Stderr, "WARNING: complypack %s has not been cryptographically verified\n", entry.EffectiveID())
		logger.Warn("Complypack not cryptographically verified", "complypack", entry.EffectiveID())
	}

	return nil
}
