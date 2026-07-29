// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/registry"
)

// verifierBuilder constructs a VerifyFunc from a VerificationConfig.
// Extracted as a function type so tests can inject a mock that avoids
// network calls (TUF root fetch, key file reads). Production code uses
// buildVerifierFromConfig.
type verifierBuilder func(
	cfg complytime.VerificationConfig,
) (cache.VerifyFunc, error)

// defaultVerifierBuilder is the production verifier constructor.
// Package-level variable allows test override without adding a parameter
// to every call site — consistent with the codebase's pattern of
// package-level logger and lw. Tests restore the original in t.Cleanup.
var defaultVerifierBuilder verifierBuilder = buildVerifierFromConfig

type getOptions struct {
	*Common
	timeout    time.Duration
	cacheDir   string
	dataDir    string
	skipVerify bool
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
	cmd.Flags().BoolVar(&o.skipVerify, "skip-verify", false, "Skip signature verification for fetched artifacts")
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
	o.dataDir, err = complytime.ResolveDataDir()
	if err != nil {
		return fmt.Errorf("failed to resolve data directory: %w", err)
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

	// When --skip-verify is set, pass nil workspace config and nil cache
	// so resolveVerifier returns nil for every entry (FR-003).
	if o.skipVerify {
		fmt.Fprintln(os.Stderr,
			"WARNING: signature verification skipped "+
				"via --skip-verify flag")
		logger.Warn(
			"Signature verification skipped via --skip-verify flag",
		)
		return o.syncAll(ctx, cfg, baseDir, nil, nil)
	}

	// Chain policy and complypack sync into a single error return.
	// syncComplypacks is a no-op when no complypacks are configured.
	// Per-entry verification is resolved inside syncAll (D3).
	return o.syncAll(ctx, cfg, baseDir, cfg.Verification, nil)
}

// syncAll runs policy sync followed by complypack sync. Creates the
// verifier cache (D4) shared across all entries within this invocation.
// wsCfg is nil when --skip-verify is set (FR-003).
func (o *getOptions) syncAll(
	ctx context.Context,
	cfg *complytime.WorkspaceConfig,
	baseDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	// Lazy-init verifier cache on first call. When --skip-verify
	// is set both wsCfg and vfCache are nil; resolveVerifier
	// handles nil cache by skipping the lookup.
	if wsCfg != nil && vfCache == nil {
		vfCache = make(
			map[complytime.VerificationConfig]cache.VerifyFunc,
		)
	}

	// Collect errors across both groups so that a policy failure
	// does not prevent complypack sync (FR-006).
	policyErr := o.syncPolicies(ctx, cfg, wsCfg, vfCache)
	complypackErr := o.syncComplypacks(
		ctx, cfg, baseDir, wsCfg, vfCache,
	)
	return errors.Join(policyErr, complypackErr)
}

// resolveVerifier determines the effective verifier for a single
// policy/complypack entry using the resolution priority chain (FR-003,
// FR-001, FR-002):
//
//  1. --skip-verify flag → caller passes nil wsCfg + nil cache
//  2. entry.SkipVerify   → return nil (no verifier)
//  3. entry.Verification → build from entry config
//  4. wsCfg              → build from workspace config
//  5. nothing configured → return nil (no verifier)
//
// The cache (D4) ensures at most one verifier per distinct
// VerificationConfig within a single complyctl get invocation.
func resolveVerifier(
	entry complytime.PolicyEntry,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) ([]cache.SyncOption, error) {
	// Entry-level opt-out (FR-002).
	if entry.SkipVerify {
		logger.Debug("Verification skipped: entry skip_verify",
			"policy", entry.EffectiveID())
		return nil, nil
	}

	// Determine the effective config: entry-level overrides
	// workspace-level (D1: standalone blocks, no merging).
	var effective *complytime.VerificationConfig
	if entry.Verification != nil && entry.Verification.IsConfigured() {
		effective = entry.Verification
	} else if wsCfg != nil && wsCfg.IsConfigured() {
		effective = wsCfg
	}

	if effective == nil {
		logger.Debug("Verification skipped: no config",
			"policy", entry.EffectiveID())
		return nil, nil
	}

	// Cache lookup (D4). A nil cache means --skip-verify was set;
	// we should not reach here because wsCfg is also nil in that
	// case, but guard defensively.
	if vfCache == nil {
		return nil, nil
	}

	cfg := *effective
	if vf, ok := vfCache[cfg]; ok {
		return []cache.SyncOption{cache.WithVerifier(vf)}, nil
	}

	// Cache miss — construct a new verifier and store it.
	vf, err := defaultVerifierBuilder(cfg)
	if err != nil {
		return nil, fmt.Errorf(
			"verification configured but initialization failed "+
				"(use --skip-verify to bypass): %w", err,
		)
	}
	vfCache[cfg] = vf
	return []cache.SyncOption{cache.WithVerifier(vf)}, nil
}

// buildVerifierFromConfig constructs a VerifyFunc from a
// VerificationConfig. This is the production implementation of
// verifierBuilder — it calls NewKeyedVerifier or NewKeylessVerifier,
// which perform network I/O (key file read / TUF root fetch).
func buildVerifierFromConfig(
	cfg complytime.VerificationConfig,
) (cache.VerifyFunc, error) {
	if cfg.Key != "" {
		return cache.NewKeyedVerifier(cfg.Key)
	}
	return cache.NewKeylessVerifier(cfg.Issuer, cfg.Identity, cfg.TrustedRoot)
}

func (o *getOptions) syncPolicies(
	ctx context.Context,
	cfg *complytime.WorkspaceConfig,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	cacheMgr := cache.NewCache(o.cacheDir)

	state, err := cache.LoadState(o.dataDir)
	if err != nil {
		logger.Error("Cache state load failed",
			"data_dir", o.dataDir, "error", err)
		return fmt.Errorf("failed to load cache state: %w", err)
	}

	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		logger.Error("Credential resolution failed", "error", err)
		return fmt.Errorf("authentication setup failed: %w", err)
	}

	return syncAllPolicies(
		ctx, cacheMgr, state, credFunc,
		cfg.Policies, o.dataDir, wsCfg, vfCache,
	)
}

// syncComplypacks fetches complypack artifacts listed in the workspace
// config. Skips silently when no complypacks are configured. After sync
// completes, validates that no two complypack entries resolved to the
// same evaluator-id.
func (o *getOptions) syncComplypacks(
	ctx context.Context,
	cfg *complytime.WorkspaceConfig,
	baseDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	if len(cfg.Complypacks) == 0 {
		return nil
	}

	state, err := cache.LoadState(o.dataDir)
	if err != nil {
		logger.Error("Cache state load failed",
			"data_dir", o.dataDir, "error", err)
		return fmt.Errorf("failed to load cache state: %w", err)
	}

	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		logger.Error("Credential resolution failed", "error", err)
		return fmt.Errorf("authentication setup failed: %w", err)
	}

	if err := syncAllComplypacks(
		ctx, state, credFunc,
		cfg.Complypacks, o.cacheDir, o.dataDir, baseDir,
		wsCfg, vfCache,
	); err != nil {
		return err
	}

	return validateUniqueEvaluatorIDs(state, cfg.Complypacks)
}

// syncAllPolicies iterates all policy entries, resolving per-entry
// verification and collecting errors (D5: errors.Join). All entries
// are attempted regardless of individual failures (FR-006).
// Creates a policy.Resolver once (D9) and passes it to each
// syncSinglePolicy call for metadata extraction.
func syncAllPolicies(
	ctx context.Context,
	cacheMgr *cache.Cache,
	state *cache.State,
	credFunc auth.CredentialFunc,
	policies []complytime.PolicyEntry,
	dataDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	logger.Info("Starting policy synchronization",
		"policy_count", len(policies))

	// Create resolver once per invocation (D9) for metadata
	// extraction in syncSinglePolicy.
	loader := policy.NewLoader(cacheMgr)
	resolver := policy.NewResolver(loader)

	total := len(policies)
	var errs []error
	for i, entry := range policies {
		if err := syncSinglePolicy(
			ctx, cacheMgr, state, credFunc,
			entry, i+1, total, dataDir, wsCfg, vfCache,
			resolver,
		); err != nil {
			eid := entry.EffectiveID()
			fmt.Fprintf(os.Stderr,
				"WARNING: policy %q sync failed: %v\n",
				eid, err)
			errs = append(errs, err)
			continue
		}
	}

	// Batch-flush metadata state after all policies are synced,
	// rather than writing to disk per-policy.
	if saveErr := cache.SaveState(
		state, dataDir,
	); saveErr != nil {
		logger.Warn("Failed to save metadata state",
			"error", saveErr)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	logger.Info("Synchronization completed",
		"synced", total, "total", total)
	fmt.Fprintln(os.Stderr, "Synchronization completed.")
	return nil
}

func syncSinglePolicy(
	ctx context.Context,
	cacheMgr *cache.Cache,
	state *cache.State,
	credFunc auth.CredentialFunc,
	entry complytime.PolicyEntry,
	index, total int,
	dataDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
	resolver *policy.Resolver,
) error {
	ref, err := complytime.ParsePolicyRef(entry.URL)
	if err != nil {
		return fmt.Errorf("invalid policy reference %q: %w",
			entry.URL, err)
	}
	version := ref.VersionString()

	// Resolve per-entry verification (FR-001, FR-002, FR-003).
	syncOpts, err := resolveVerifier(entry, wsCfg, vfCache)
	if err != nil {
		return fmt.Errorf("policy %s: %w",
			entry.EffectiveID(), err)
	}

	client := registry.NewClient(ref.Registry, credFunc)
	source := cache.NewRegistrySource(client)
	sync := cache.NewSync(cacheMgr, state, source, dataDir, ref.Registry, syncOpts...)

	if version == "" {
		version = resolveLatestVersion(
			ctx, client, ref.Repository, entry.EffectiveID(),
		)
	}

	fmt.Fprintf(os.Stderr, "Syncing policy %d/%d: %s... ",
		index, total, entry.EffectiveID())
	logger.Info("Syncing policy",
		"policy", ref.Repository, "version", version)
	fetched, err := sync.SyncPolicy(ctx, ref.Repository, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed")
		logger.Error("Policy sync failed",
			"policy", ref.Repository, "error", err)
		return err
	}
	fmt.Fprintln(os.Stderr, "done")

	if fetched {
		ps, _ := state.GetPolicyState(ref.Repository)
		logger.Info("Policy synced",
			"policy", entry.EffectiveID(), "digest", ps.Digest)
		if !ps.Verified {
			fmt.Fprintf(os.Stderr,
				"NOTE: policy %s was fetched without "+
					"signature verification. Configure "+
					"verification: in complytime.yaml to "+
					"enable signature checking.\n",
				entry.EffectiveID())
			logger.Warn("Policy not cryptographically verified",
				"policy", entry.EffectiveID(),
				"digest", ps.Digest)
		}
	} else {
		logger.Info("Policy synced",
			"policy", entry.EffectiveID())
	}

	// Extract and cache metadata after sync (D7 ordering).
	// Needed for fresh fetches or upgrade backfill (D8).
	// hasMetadata() (not just PolicyEvaluator=="") avoids redundant
	// re-extraction for multi-evaluator policies where PolicyEvaluator
	// is intentionally empty.
	ps, _ := state.GetPolicyState(ref.Repository)
	extractMetadata := fetched || !hasMetadata(ps)

	if extractMetadata {
		// Use the concrete version SyncPolicy persisted, not the config
		// tag: a ":latest" pin is stored under the resolved version, so
		// extracting by "latest" would fail store.Resolve() and silently
		// drop all metadata.
		meta, metaErr := resolver.ExtractPolicyMetadata(
			ref.Repository, ps.Version,
		)
		if metaErr != nil {
			// D5: metadata extraction failure is non-fatal.
			logger.Warn(
				"Failed to extract policy metadata",
				"policy", entry.EffectiveID(),
				"error", metaErr,
			)
		} else {
			state.SetPolicyMetadata(
				ref.Repository,
				meta.Title,
				meta.EvaluatorID,
				meta.ControlCount,
				meta.AssessmentCount,
			)
			summary := formatPolicySummary(meta)
			if summary != "" {
				fmt.Fprint(os.Stderr, summary)
			}
		}
	}

	return nil
}

// formatPolicySummary builds a human-readable summary string from
// extracted policy metadata. Returns empty string when no meaningful
// content is available to display.
func formatPolicySummary(meta policy.PolicyMetadata) string {
	var sb strings.Builder

	// Build the "Policy:" line based on available fields.
	hasTitle := meta.Title != ""
	hasEval := meta.EvaluatorID != ""
	switch {
	case hasTitle && hasEval:
		fmt.Fprintf(&sb, "  Policy: %s (%s)\n",
			meta.Title, meta.EvaluatorID)
	case hasTitle:
		fmt.Fprintf(&sb, "  Policy: %s\n", meta.Title)
	case hasEval:
		fmt.Fprintf(&sb, "  Policy: %s\n", meta.EvaluatorID)
	}

	// Always print counts when we have metadata to show.
	if hasTitle || hasEval ||
		meta.ControlCount > 0 || meta.AssessmentCount > 0 {
		fmt.Fprintf(&sb,
			"  Controls: %d | Assessments: %d\n",
			meta.ControlCount, meta.AssessmentCount)
	}

	return sb.String()
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

// syncAllComplypacks iterates all complypack entries, resolving
// per-entry verification and collecting errors (D5: errors.Join).
// All entries are attempted regardless of individual failures (FR-006).
func syncAllComplypacks(
	ctx context.Context,
	state *cache.State,
	credFunc auth.CredentialFunc,
	complypacks []complytime.PolicyEntry,
	cacheDir, dataDir, baseDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	logger.Info("Starting complypack synchronization",
		"complypack_count", len(complypacks))

	total := len(complypacks)
	var errs []error
	for i, entry := range complypacks {
		if err := syncSingleComplypack(
			ctx, state, credFunc,
			entry, i+1, total, cacheDir, dataDir, baseDir,
			wsCfg, vfCache,
		); err != nil {
			eid := entry.EffectiveID()
			fmt.Fprintf(os.Stderr,
				"WARNING: complypack %q sync failed: %v\n",
				eid, err)
			errs = append(errs, err)
			continue
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	logger.Info("Complypack synchronization completed",
		"synced", total, "total", total)
	fmt.Fprintln(os.Stderr,
		"Complypack synchronization completed.")
	return nil
}

func syncSingleComplypack(
	ctx context.Context,
	state *cache.State,
	credFunc auth.CredentialFunc,
	entry complytime.PolicyEntry,
	index, total int,
	cacheDir, dataDir, baseDir string,
	wsCfg *complytime.VerificationConfig,
	vfCache map[complytime.VerificationConfig]cache.VerifyFunc,
) error {
	ref, err := complytime.ParsePolicyRef(entry.URL)
	if err != nil {
		return fmt.Errorf(
			"invalid complypack reference %q: %w",
			entry.URL, err,
		)
	}
	version := ref.VersionString()

	// Resolve per-entry verification (FR-001, FR-002, FR-003).
	syncOpts, err := resolveVerifier(entry, wsCfg, vfCache)
	if err != nil {
		return fmt.Errorf("complypack %s: %w",
			entry.EffectiveID(), err)
	}

	client := registry.NewClient(ref.Registry, credFunc)
	source := cache.NewRegistryComplypackSource(client)
	complypackCache := cache.NewComplypackCache(cacheDir, state)
	cpSync := cache.NewComplypackSync(
		complypackCache, state, source, dataDir, ref.Registry, syncOpts...,
	)

	if version == "" {
		version = resolveLatestVersion(
			ctx, client, ref.Repository, entry.EffectiveID(),
		)
	}

	fmt.Fprintf(os.Stderr, "Syncing complypack %d/%d: %s... ",
		index, total, entry.EffectiveID())
	logger.Info("Syncing complypack",
		"complypack", ref.Repository, "version", version)
	fetched, err := cpSync.SyncComplypack(
		ctx, ref.Repository, version,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed")
		logger.Error("Complypack sync failed",
			"complypack", ref.Repository, "error", err)
		return fmt.Errorf("failed to sync complypack %s: %w",
			ref.Repository, err)
	}
	fmt.Fprintln(os.Stderr, "done")
	logger.Info("Complypack synced",
		"complypack", entry.EffectiveID())

	// Emit unverified warning only when the complypack was freshly
	// fetched and was NOT verified.
	if fetched {
		cpState, exists := state.GetComplypackState(
			ref.Repository,
		)
		if !exists || !cpState.Verified {
			fmt.Fprintf(os.Stderr,
				"NOTE: complypack %s was fetched without "+
					"signature verification. Configure "+
					"verification: in complytime.yaml to "+
					"enable signature checking.\n",
				entry.EffectiveID())
			logger.Warn(
				"Complypack not cryptographically verified",
				"complypack", entry.EffectiveID())
		}
		invalidateGenerationForComplypack(
			state, ref.Repository, baseDir,
		)
	}

	return nil
}

// invalidateGenerationForComplypack removes workspace generation state and
// evaluator artifacts when a complypack has been re-fetched. Errors are logged
// as warnings — they do not fail the get command.
func invalidateGenerationForComplypack(state *cache.State, repository, baseDir string) {
	ps, exists := state.GetComplypackState(repository)
	if !exists || ps.EvaluatorID == "" {
		return
	}
	evalID := ps.EvaluatorID

	skipWarnings, err := policy.InvalidateForEvaluator(baseDir, evalID)
	for _, w := range skipWarnings {
		logger.Debug("Generation state invalidation", "warning", w)
	}
	if err != nil {
		logger.Warn("Failed to invalidate generation state", "evaluator", evalID, "error", err)
		fmt.Fprintf(os.Stderr, "WARNING: failed to invalidate generation state for %s: %v\n", evalID, err)
		return
	}
	if err := policy.RemoveEvaluatorArtifacts(baseDir, evalID); err != nil {
		logger.Warn("Failed to remove evaluator artifacts", "evaluator", evalID, "error", err)
		fmt.Fprintf(os.Stderr, "WARNING: failed to remove evaluator artifacts for %s: %v\n", evalID, err)
		return
	}

	fmt.Fprintf(os.Stderr, "Complypack %s updated — generation cache invalidated for %s\n", repository, evalID)
	logger.Info("Generation cache invalidated after complypack update", "repository", repository, "evaluator", evalID)
}

// validateUniqueEvaluatorIDs checks that no two complypack entries in the
// workspace config resolved to the same evaluator-id. Evaluator-ids are
// only known after sync (they come from the complypack's embedded
// config.json), so this validation runs post-sync.
//
// Returns a descriptive error listing all conflicting repositories when
// duplicates are found. The error causes complyctl get to exit non-zero.
func validateUniqueEvaluatorIDs(state *cache.State, complypacks []complytime.PolicyEntry) error {
	// Build evaluator-id → []repository map from state, scoped to the
	// configured complypack entries.
	evalToRepos := make(map[string][]string)
	for _, entry := range complypacks {
		// ParsePolicyRef cannot fail here: syncAllComplypacks already
		// validated all entries. Skip safely if it does — the entry
		// has no state to contribute to conflict detection.
		ref, err := complytime.ParsePolicyRef(entry.URL)
		if err != nil {
			continue
		}
		ps, exists := state.GetComplypackState(ref.Repository)
		if !exists || ps.EvaluatorID == "" {
			continue
		}
		evalToRepos[ps.EvaluatorID] = append(evalToRepos[ps.EvaluatorID], ref.Repository)
	}

	var conflicts []string
	for evalID, repos := range evalToRepos {
		if len(repos) > 1 {
			sort.Strings(repos)
			lines := make([]string, 0, len(repos))
			for _, r := range repos {
				lines = append(lines, fmt.Sprintf("  - %s", r))
			}
			conflicts = append(conflicts, fmt.Sprintf(
				"duplicate evaluator-id %q found in complypack entries:\n%s",
				evalID, strings.Join(lines, "\n")))
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	sort.Strings(conflicts)
	return fmt.Errorf(
		"%s\nremove one of the conflicting entries from complytime.yaml",
		strings.Join(conflicts, "\n"))
}
