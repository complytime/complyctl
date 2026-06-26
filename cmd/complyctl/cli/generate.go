// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/provider"
)

type generateOptions struct {
	*Common
	policyID    string
	timeout     time.Duration
	cacheDir    string
	providerDir string
}

func generateCmd(common *Common) *cobra.Command {
	o := &generateOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:               "generate [flags]",
		Short:             "Generate policy graph and invoke providers",
		Example:           `complyctl generate --policy-id nist-800-53-r5`,
		SilenceUsage:      true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := o.complete(); err != nil {
				return err
			}
			return o.run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&o.policyID, "policy-id", "p", "", "Policy ID to generate (see complyctl list)")
	cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", complytime.DefaultCommandTimeout, "Maximum time for the generate operation (e.g. 5m, 10m, 1h)")
	if err := cmd.MarkFlagRequired("policy-id"); err != nil {
		logger.Error("Failed to mark policy-id as required", "error", err)
	}
	if err := cmd.RegisterFlagCompletionFunc("policy-id", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.Error("Failed to register policy-id completion", "error", err)
	}
	return cmd
}

func (o *generateOptions) complete() error {
	var err error
	o.cacheDir, err = complytime.ResolveCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}
	o.providerDir, err = complytime.ResolveProviderDir()
	if err != nil {
		return fmt.Errorf("failed to resolve provider directory: %w", err)
	}
	return nil
}

func (o *generateOptions) run(ctx context.Context) error {
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

	entry, found := complytime.FindPolicy(cfg.Policies, o.policyID)
	if !found {
		return fmt.Errorf("policy %q not found in config — run complyctl list to see available policy IDs", o.policyID)
	}

	return o.generatePolicy(ctx, cfg, *entry, baseDir)
}

func (o *generateOptions) generatePolicy(ctx context.Context, cfg *complytime.WorkspaceConfig, entry complytime.PolicyEntry, baseDir string) error {
	ref, err := complytime.ParsePolicyRef(entry.URL)
	if err != nil {
		return fmt.Errorf("invalid policy reference %q: %w", entry.URL, err)
	}
	eid := entry.EffectiveID()

	version, graph, err := resolveVersionAndGraph(o.cacheDir, ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Resolved %s version: %s\n", ref.Repository, version)

	mgr, err := loadProviders(o.providerDir)
	if err != nil {
		return err
	}
	defer mgr.Cleanup()

	configs := policy.ExtractAssessmentConfigs(graph)
	groups := policy.GroupByEvaluator(configs, graph)
	policyTargets := filterTargetsForPolicy(cfg.Targets, eid)

	evaluatorIDs, planRows, err := invokeGenerate(ctx, o.cacheDir, baseDir, mgr, groups, policyTargets, cfg.Variables)
	if err != nil {
		return err
	}

	return saveGenerationAndPrint(o.cacheDir, baseDir, ref.Repository, eid, evaluatorIDs, planRows)
}

func invokeGenerate(ctx context.Context, cacheDir, baseDir string, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig, configVars map[string]string) ([]string, []output.ExecutionPlanRow, error) {
	spin := terminal.NewSpinner("Generating policy artifacts...")
	spin.Start()
	defer spin.Stop()

	globalVars := complytime.WithWorkspaceVar(configVars, baseDir)
	if err := generateForAllTargets(ctx, cacheDir, mgr, groups, policyTargets, globalVars); err != nil {
		return nil, nil, err
	}

	evaluatorIDs, planRows := buildExecutionPlan(mgr, groups, policyTargets)
	return evaluatorIDs, planRows, nil
}

func buildExecutionPlan(mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig) ([]string, []output.ExecutionPlanRow) {
	var evaluatorIDs []string
	var planRows []output.ExecutionPlanRow
	for evalID, group := range groups {
		evaluatorIDs = append(evaluatorIDs, evalID)
		status := providerStatus(mgr, evalID)
		for _, target := range policyTargets {
			planRows = append(planRows, output.ExecutionPlanRow{
				TargetID:         target.ID,
				ProviderID:       evalID,
				RequirementCount: len(group.Configs),
				Status:           status,
			})
		}
	}
	return evaluatorIDs, planRows
}

func providerStatus(mgr *provider.Manager, evalID string) string {
	if _, err := mgr.GetProvider(evalID); err != nil {
		return "ERROR"
	}
	return "healthy"
}

func saveGenerationAndPrint(cacheDir, baseDir, repository, eid string, evaluatorIDs []string, planRows []output.ExecutionPlanRow) error {
	cacheState, err := cache.LoadState(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to load cache state: %w", err)
	}
	policyState, _ := cacheState.GetPolicyState(repository)
	cpDigests := complypackDigestsByEvaluator(cacheState)
	genState := policy.NewGenerationState(repository, policyState.Digest, evaluatorIDs, cpDigests)
	if err := policy.SaveGenerationState(baseDir, repository, genState); err != nil {
		return fmt.Errorf("failed to save generation state: %w", err)
	}

	fmt.Print(output.FormatExecutionPlan(eid, planRows))
	return nil
}
