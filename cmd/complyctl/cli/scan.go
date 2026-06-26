// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/provider"
)

type scanOptions struct {
	*Common
	target      string
	policyID    string
	format      string
	timeout     time.Duration
	cacheDir    string
	providerDir string
}

// resolvedMappings groups the ID-resolution maps built during scan setup.
// Bundling them into a struct prevents parameter-transposition bugs across
// the identically-typed map[string]string arguments.
type resolvedMappings struct {
	reqToControl       map[string]string
	reqToPlan          map[string]string
	reqToComplypackRef map[string]string
}

func scanCmd(common *Common) *cobra.Command {
	o := &scanOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:   "scan [target] [flags]",
		Short: "Scan targets and produce compliance reports",
		Long: `Scan targets and produce compliance reports.

Specify a target to scope the scan to a single target from complytime.yaml.
When the target references exactly one policy, --policy-id is inferred.
When no target is given, --policy-id is required and all matching targets are scanned.

Set COMPLYTIME_EXPORT_ENABLED=true to export evidence to a Beacon collector
after the scan completes. Requires a collector section in complytime.yaml.
Export works alongside any --format flag. The variable must be set in the
same shell session or CI job step that invokes complyctl scan.

Exit codes:
  0  Scan completed (findings are reported, not errors)
  1  Operational error (provider failure, bad config, zero assessed)

Policy findings are data, not errors. To gate a pipeline on compliance
results, parse the --format output (SARIF, OSCAL) with your policy engine.`,
		Example: `  # Scan a specific target (policy inferred if target has exactly one)
  complyctl scan prod

  # Scan a specific target for a specific policy
  complyctl scan prod --policy-id nist-800-53-r5

  # Scan all targets for a policy (backward compatible)
  complyctl scan --policy-id nist-800-53-r5

  # Scan with output format
  complyctl scan prod --policy-id nist-800-53-r5 --format pretty
  complyctl scan --policy-id nist-800-53-r5 --format oscal

  # Export evidence to Beacon collector alongside format report
  COMPLYTIME_EXPORT_ENABLED=true complyctl scan prod --format sarif`,
		SilenceUsage:      true,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTargetIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.target = args[0]
			}
			if err := o.validate(); err != nil {
				return err
			}
			if err := o.complete(); err != nil {
				return err
			}
			return o.run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&o.policyID, "policy-id", "p", "", "Policy ID to scan (see complyctl list)")
	cmd.Flags().StringVarP(&o.format, "format", "f", "", "Output format: oscal, pretty, sarif")
	cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", complytime.DefaultCommandTimeout, "Maximum time for the scan operation (e.g. 5m, 10m, 1h)")
	if err := cmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{complytime.OutputFormatOSCAL, complytime.OutputFormatPretty, complytime.OutputFormatSARIF}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.Error("Failed to register format completion", "error", err)
	}
	if err := cmd.RegisterFlagCompletionFunc("policy-id", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.Error("Failed to register policy-id completion", "error", err)
	}
	return cmd
}

// completeTargetIDs provides shell completion for the scan command's
// positional target argument by reading target IDs from complytime.yaml.
func completeTargetIDs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first positional argument.
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	baseDir, err := complytime.ResolveWorkspaceDir("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := loadWorkspaceConfig(baseDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ids := make([]string, 0, len(cfg.Targets))
	for _, t := range cfg.Targets {
		ids = append(ids, t.ID)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

func (o *scanOptions) validate() error {
	if o.format != "" {
		switch o.format {
		case complytime.OutputFormatOSCAL, complytime.OutputFormatPretty, complytime.OutputFormatSARIF:
		default:
			return fmt.Errorf("invalid format %q: must be one of %s, %s, %s",
				o.format, complytime.OutputFormatOSCAL, complytime.OutputFormatPretty, complytime.OutputFormatSARIF)
		}
	}
	return nil
}

func (o *scanOptions) complete() error {
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

func (o *scanOptions) run(ctx context.Context) error {
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

	if len(cfg.Targets) == 0 {
		return fmt.Errorf("no targets in complytime.yaml (add targets with policies)")
	}

	// Resolve target (if specified) and determine the effective policy ID.
	var target *complytime.TargetConfig
	if o.target != "" {
		target, err = resolveTarget(cfg, o.target)
		if err != nil {
			return err
		}
	}

	policyID, err := resolvePolicy(target, o.policyID)
	if err != nil {
		return err
	}

	entry, found := complytime.FindPolicy(cfg.Policies, policyID)
	if !found {
		return fmt.Errorf("policy %q not found in config — run complyctl list to see available policy IDs", policyID)
	}

	return o.scanPolicy(ctx, cfg, *entry, o.target, baseDir)
}

// resolveTarget finds a target by ID in the config's target list.
// Returns an error listing available target IDs if the target is not found.
func resolveTarget(cfg *complytime.WorkspaceConfig, targetID string) (*complytime.TargetConfig, error) {
	for i, t := range cfg.Targets {
		if t.ID == targetID {
			return &cfg.Targets[i], nil
		}
	}
	available := make([]string, 0, len(cfg.Targets))
	for _, t := range cfg.Targets {
		available = append(available, t.ID)
	}
	return nil, fmt.Errorf("target %q not found in complytime.yaml (available targets: %s)",
		targetID, strings.Join(available, ", "))
}

// resolvePolicy determines the policy ID to use based on the target and
// --policy-id flag combination. It handles three cases:
//   - Both target and policy-id: validates the target references the policy
//   - Target without policy-id: infers policy if target has exactly one
//   - No target: requires policy-id (backward compatible)
func resolvePolicy(target *complytime.TargetConfig, policyID string) (string, error) {
	if target != nil && policyID != "" {
		// Both specified — validate the target references the policy.
		if !slices.Contains(target.Policies, policyID) {
			return "", fmt.Errorf("target %q does not reference policy %q (available policies: %s)",
				target.ID, policyID, strings.Join(target.Policies, ", "))
		}
		return policyID, nil
	}

	if target != nil && policyID == "" {
		// Target without policy-id — infer if single policy.
		switch len(target.Policies) {
		case 0:
			return "", fmt.Errorf("target %q has no policies configured", target.ID)
		case 1:
			return target.Policies[0], nil
		default:
			return "", fmt.Errorf("target %q references multiple policies — specify one with --policy-id: %s",
				target.ID, strings.Join(target.Policies, ", "))
		}
	}

	// No target — require policy-id.
	if policyID == "" {
		return "", fmt.Errorf("specify a target or --policy-id (see complyctl scan --help)")
	}
	return policyID, nil
}

func loadWorkspaceConfig(baseDir string) (*complytime.WorkspaceConfig, error) {
	ws := complytime.NewWorkspace(baseDir)
	if err := ws.LoadAndValidate(); err != nil {
		return nil, fmt.Errorf("failed to load workspace config: %w", err)
	}
	return ws.Config(), nil
}

func (o *scanOptions) scanPolicy(ctx context.Context, cfg *complytime.WorkspaceConfig, entry complytime.PolicyEntry, targetID, baseDir string) error {
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

	assessmentConfigs := policy.ExtractAssessmentConfigs(graph)
	groups := policy.GroupByEvaluator(assessmentConfigs, graph)

	mgr, err := loadProviders(o.providerDir)
	if err != nil {
		return err
	}
	defer mgr.Cleanup()

	policyTargets := filterTargetsForPolicy(cfg.Targets, eid)
	evaluatorIDs := evaluatorIDList(groups)

	// Generation runs for ALL targets referencing the policy (per D7:
	// generation freshness is policy-scoped, not target-scoped). Narrowing
	// before generation would silently skip targets that were never generated.
	if err := ensureGenerated(ctx, o.cacheDir, baseDir, mgr, groups, policyTargets, cfg.Variables, ref.Repository, eid, evaluatorIDs); err != nil {
		return err
	}

	// Narrow to the requested target AFTER generation, BEFORE scan execution.
	if targetID != "" {
		policyTargets = filterTargetByID(policyTargets, targetID)
	}

	targetIDs := targetIDList(policyTargets)
	fmt.Println(output.FormatPreScanSummary(len(assessmentConfigs), evaluatorIDs, targetIDs))

	return o.executeScanPhase(ctx, cfg, mgr, groups, policyTargets, ref.Repository, eid, graph, targetIDs, baseDir)
}

func (o *scanOptions) executeScanPhase(ctx context.Context, cfg *complytime.WorkspaceConfig, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig, repository, eid string, graph *policy.DependencyGraph, targetIDs []string, baseDir string) error {
	reqToComplypackRef := buildReqToComplypackRef(o.cacheDir, groups)
	if err := runScanAndReport(ctx, o.format, mgr, groups, reqToComplypackRef, policyTargets, repository, eid, graph, targetIDs, baseDir); err != nil {
		return err
	}
	return o.maybeExport(ctx, cfg, mgr, groups)
}

func (o *scanOptions) maybeExport(ctx context.Context, cfg *complytime.WorkspaceConfig, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup) error {
	enabled, raw, err := complytime.ExportEnabled()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %s=%q is not a recognized boolean value; export disabled (accepted: true, false, 1, 0, t, f)\n",
			complytime.ExportEnabledEnvVar, raw)
		return nil
	}
	if !enabled {
		return nil
	}
	return o.runExport(ctx, cfg, mgr, groups)
}

func resolveVersionAndGraph(cacheDir string, ref complytime.PolicyRef) (string, *policy.DependencyGraph, error) {
	cacheMgr := cache.NewCache(cacheDir)
	loader := policy.NewLoader(cacheMgr)
	resolver := policy.NewResolver(loader)

	version, err := loader.ResolveVersion(ref.Repository, ref.VersionString())
	if err != nil {
		return "", nil, err
	}

	graph, err := resolver.ResolvePolicyGraph(ref.Repository, version)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve policy graph: %w", err)
	}
	return version, graph, nil
}

func loadProviders(providerDir string) (*provider.Manager, error) {
	mgr, err := provider.NewManager(providerDir, logger)
	if err != nil {
		return nil, fmt.Errorf("provider manager init failed: %w", err)
	}
	if err := mgr.LoadProviders(); err != nil {
		mgr.Cleanup()
		return nil, fmt.Errorf("provider discovery failed: %w", err)
	}
	if len(mgr.ListProviders()) == 0 {
		mgr.Cleanup()
		return nil, fmt.Errorf("no providers found in %s (Describe may have failed)", providerDir)
	}
	return mgr, nil
}

func evaluatorIDList(groups map[string]policy.EvaluatorGroup) []string {
	ids := make([]string, 0, len(groups))
	for evalID := range groups {
		ids = append(ids, evalID)
	}
	return ids
}

func targetIDList(targets []complytime.TargetConfig) []string {
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.ID)
	}
	return ids
}

func ensureGenerated(ctx context.Context, cacheDir, baseDir string, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig, configVars map[string]string, repository, eid string, evaluatorIDs []string) error {
	needsGenerate, policyDigest, cpDigests, err := checkGenerationFreshness(cacheDir, baseDir, repository, eid)
	if err != nil {
		return err
	}
	if !needsGenerate {
		return nil
	}
	globalVars := complytime.WithWorkspaceVar(configVars, baseDir)
	return runGeneration(ctx, cacheDir, baseDir, mgr, groups, policyTargets, globalVars, repository, policyDigest, evaluatorIDs, cpDigests)
}

// runScanAndReport executes the scan across all targets and processes the
// combined output (reports + error checking). It delegates post-scan handling
// to processScanOutput.
func runScanAndReport(ctx context.Context, format string, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, reqToComplypackRef map[string]string, policyTargets []complytime.TargetConfig, repository, eid string, graph *policy.DependencyGraph, targetIDs []string, baseDir string) error {
	planToReq := extractPlanToReqMap(graph)
	mappings := resolvedMappings{
		reqToControl:       extractReqToControlMap(graph),
		reqToPlan:          reverseMap(planToReq),
		reqToComplypackRef: reqToComplypackRef,
	}
	targetsWithWorkspace := injectWorkspaceIntoTargets(policyTargets, baseDir)
	scanOut, err := executeScan(ctx, mgr, groups, targetsWithWorkspace)
	if err != nil {
		return err
	}

	resolveAssessmentIDs(scanOut.assessments, planToReq)
	return processScanOutput(format, scanOut, repository, &mappings, policyTargets, eid, targetIDs, baseDir)
}

// processScanOutput handles post-scan output: prints operational warnings to
// stderr, writes evaluation reports, and returns an error when operational
// failures are present (triggering non-zero exit). Reports are always written
// before the error return so partial results remain available.
func processScanOutput(format string, scanOut *scanOutput, repository string, mappings *resolvedMappings, policyTargets []complytime.TargetConfig, eid string, targetIDs []string, baseDir string) error {
	reportOperationalWarnings(scanOut.errors)

	evaluators := buildEvaluators(repository, mappings, policyTargets, scanOut.assessments, scanOut.assessmentTargets)

	outDir := filepath.Join(baseDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
	for _, eval := range evaluators {
		if err := writeScanReports(format, eval, outDir, baseDir, repository); err != nil {
			return err
		}
	}

	fmt.Println(output.FormatScanSummary(scanOut.assessments, scanOut.assessmentTargets, mappings.reqToControl, eid, targetIDs))

	if err := checkOperationalErrors(scanOut.errors); err != nil {
		return err
	}
	return checkNothingAssessed(scanOut.assessments)
}

// reportOperationalWarnings prints provider-reported operational errors as
// WARNING lines to stderr. No output is produced when errors is empty.
func reportOperationalWarnings(errors []string) {
	if warnings := output.FormatOperationalWarnings(errors); warnings != "" {
		fmt.Fprint(os.Stderr, warnings)
	}
}

// checkOperationalErrors returns an error summarizing the count of operational
// failures. The returned error causes cobra to exit non-zero. Returns nil when
// no operational errors occurred.
func checkOperationalErrors(errors []string) error {
	if len(errors) > 0 {
		noun := "errors"
		if len(errors) == 1 {
			noun = "error"
		}
		return fmt.Errorf("scan completed with %d operational %s — some targets could not be evaluated", len(errors), noun)
	}
	return nil
}

// checkNothingAssessed returns an error when zero requirements received a
// definitive pass or fail result. A scan that produces no assessed results
// is a false-confidence hazard and must be surfaced.
func checkNothingAssessed(assessments []provider.AssessmentLog) error {
	if output.NothingAssessed(assessments) {
		fmt.Fprintf(os.Stderr, "\nWARNING: scan completed but no requirements were assessed — all may be not applicable for this target\n")
		return fmt.Errorf("scan completed with zero assessed requirements — verify policy and target compatibility")
	}
	return nil
}

func buildEvaluators(repository string, mappings *resolvedMappings, policyTargets []complytime.TargetConfig, allAssessments []provider.AssessmentLog, assessmentTargets []string) []*output.Evaluator {
	evaluators := make([]*output.Evaluator, 0, len(policyTargets))
	for _, target := range policyTargets {
		eval := output.NewEvaluator(repository, target.ID, mappings.reqToControl, mappings.reqToPlan, mappings.reqToComplypackRef)
		var targetAssessments []provider.AssessmentLog
		for j, a := range allAssessments {
			if assessmentTargets[j] == target.ID {
				targetAssessments = append(targetAssessments, a)
			}
		}
		eval.AddTarget(targetAssessments)
		evaluators = append(evaluators, eval)
	}
	return evaluators
}

// filterTargetByID narrows a target slice to the single target matching the
// given ID. Returns the original slice unchanged if no match is found (the
// caller has already validated the target exists via resolveTarget).
func filterTargetByID(targets []complytime.TargetConfig, targetID string) []complytime.TargetConfig {
	for _, t := range targets {
		if t.ID == targetID {
			return []complytime.TargetConfig{t}
		}
	}
	return targets
}

func filterTargetsForPolicy(targets []complytime.TargetConfig, policyID string) []complytime.TargetConfig {
	var result []complytime.TargetConfig
	for _, t := range targets {
		if slices.Contains(t.Policies, policyID) {
			result = append(result, t)
		}
	}
	return result
}

func checkGenerationFreshness(cacheDir, baseDir, repository, eid string) (needsGenerate bool, digest string, complypackDigests map[string]string, err error) {
	cacheState, err := cache.LoadState(cacheDir)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to load cache state: %w", err)
	}
	policyState, _ := cacheState.GetPolicyState(repository)
	cpDigests := complypackDigestsByEvaluator(cacheState)

	genState, err := policy.LoadGenerationState(baseDir, repository)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to load generation state: %w", err)
	}

	return needsRegeneration(baseDir, genState, policyState.Digest, cpDigests, eid), policyState.Digest, cpDigests, nil
}

func complypackDigestsByEvaluator(cacheState *cache.State) map[string]string {
	m := make(map[string]string)
	for _, ps := range cacheState.Complypacks {
		if ps.EvaluatorID != "" && ps.Digest != "" {
			m[ps.EvaluatorID] = ps.Digest
		}
	}
	return m
}

func needsRegeneration(baseDir string, genState *policy.GenerationState, digest string, complypackDigests map[string]string, eid string) bool {
	if genState == nil {
		fmt.Fprintf(os.Stderr, "No prior generation found — generating artifacts for %s\n", eid)
		return true
	}
	if !genState.IsFresh(digest, complypackDigests) {
		fmt.Fprintf(os.Stderr, "Policy or complypack updated for %s since last generate — regenerating\n", eid)
		return true
	}
	if !evaluatorArtifactsExist(baseDir, genState.EvaluatorIDs) {
		fmt.Fprintf(os.Stderr, "Generated artifacts missing on disk for %s — regenerating\n", eid)
		return true
	}
	fmt.Fprintf(os.Stderr, "Reusing generated artifacts for %s (unchanged since last generate)\n", eid)
	return false
}

func evaluatorArtifactsExist(baseDir string, evaluatorIDs []string) bool {
	for _, evalID := range evaluatorIDs {
		evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, evalID)
		info, err := os.Stat(evalDir)
		if err != nil || !info.IsDir() {
			return false
		}
		entries, err := os.ReadDir(evalDir)
		if err != nil || len(entries) == 0 {
			return false
		}
	}
	return true
}

func runGeneration(ctx context.Context, cacheDir, baseDir string, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig, globalVars map[string]string, repository, policyDigest string, evaluatorIDs []string, complypackDigests map[string]string) error {
	genSpin := terminal.NewSpinner("Generating policy artifacts...")
	genSpin.Start()
	defer genSpin.Stop()

	if err := generateForAllTargets(ctx, cacheDir, mgr, groups, policyTargets, globalVars); err != nil {
		return err
	}

	newGenState := policy.NewGenerationState(repository, policyDigest, evaluatorIDs, complypackDigests)
	if err := policy.SaveGenerationState(baseDir, repository, newGenState); err != nil {
		return fmt.Errorf("failed to save generation state: %w", err)
	}
	return nil
}

func generateForAllTargets(ctx context.Context, cacheDir string, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig, globalVars map[string]string) error {
	complypackCache := cache.NewComplypackCache(cacheDir)
	workspace := globalVars[complytime.WorkspaceVarKey]
	for evalID, group := range groups {
		// Look up cached complypack content for this evaluator-id.
		// If no complypack is cached, contentPath is "" (backward compatible).
		contentPath, _, err := complypackCache.LookupByEvaluatorID(evalID)
		if err != nil {
			return fmt.Errorf("failed to look up complypack for evaluator %s: %w", evalID, err)
		}
		for _, target := range policyTargets {
			targetVars := complytime.WithWorkspaceVar(target.Variables, workspace)
			if err := mgr.RouteGenerate(ctx, evalID, globalVars, targetVars, group.Configs, contentPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func executeScan(ctx context.Context, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig) (*scanOutput, error) {
	scanSpin := terminal.NewSpinner("Scanning targets...")
	scanSpin.Start()
	defer scanSpin.Stop()

	return scanAllTargets(ctx, mgr, groups, policyTargets)
}

func scanAllTargets(ctx context.Context, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, policyTargets []complytime.TargetConfig) (*scanOutput, error) {
	out := &scanOutput{}

	for _, target := range policyTargets {
		results, opErrors, err := scanSingleTarget(ctx, mgr, groups, target)
		if err != nil {
			return nil, err
		}
		out.assessments = append(out.assessments, results...)
		for range results {
			out.assessmentTargets = append(out.assessmentTargets, target.ID)
		}
		out.errors = append(out.errors, opErrors...)
	}

	return out, nil
}

// scanOutput holds the combined results of scanning all targets, separating
// evaluation results (assessments) from operational failures (errors).
type scanOutput struct {
	assessments       []provider.AssessmentLog
	assessmentTargets []string
	errors            []string
}

func scanSingleTarget(ctx context.Context, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, target complytime.TargetConfig) ([]provider.AssessmentLog, []string, error) {
	providerTargets := []provider.Target{{
		TargetID:  target.ID,
		Variables: target.Variables,
	}}

	var results []provider.AssessmentLog
	var operationalErrors []string
	for evalID := range groups {
		scanResult, routeErr := mgr.RouteScanResult(ctx, evalID, providerTargets)
		if routeErr != nil {
			return nil, nil, routeErr
		}
		results = append(results, scanResult.Assessments...)
		operationalErrors = append(operationalErrors, scanResult.Errors...)
	}
	return results, operationalErrors, nil
}

func writeScanReports(format string, eval *output.Evaluator, outDir, reportDir, repository string) error {
	logPath, err := eval.Write(outDir)
	if err != nil {
		return fmt.Errorf("failed to write evaluation log: %w", err)
	}
	fmt.Printf("Evaluation log written: %s [target: %s]\n", logPath, eval.TargetID())

	if err := writeFormatReport(format, eval, logPath, reportDir, repository); err != nil {
		return err
	}

	return nil
}

func writeFormatReport(format string, eval *output.Evaluator, logPath, reportDir, repository string) error {
	switch format {
	case complytime.OutputFormatPretty:
		return writePrettyReport(eval, logPath, reportDir, repository)
	case complytime.OutputFormatSARIF:
		return writeSARIFReport(eval, reportDir)
	case complytime.OutputFormatOSCAL:
		return writeOSCALReport(eval, reportDir)
	}
	return nil
}

func writePrettyReport(eval *output.Evaluator, logPath, reportDir, repository string) error {
	md := output.NewMarkdown(repository, eval.GemaraLog())
	md.SetEmbedEvaluationLog(logPath)
	mdPath, err := md.Write(reportDir)
	if err != nil {
		return fmt.Errorf("failed to write markdown report: %w", err)
	}
	fmt.Printf("Markdown report written: %s\n\n", mdPath)
	return nil
}

func writeSARIFReport(eval *output.Evaluator, reportDir string) error {
	sarifPath, err := output.ToSARIF(eval.GemaraLog(), "file:///scan", reportDir)
	if err != nil {
		return fmt.Errorf("failed to export SARIF: %w", err)
	}
	fmt.Printf("SARIF report written: %s\n\n", sarifPath)
	return nil
}

func writeOSCALReport(eval *output.Evaluator, reportDir string) error {
	oscalPath, err := output.ToOSCAL(eval.GemaraLog(), reportDir)
	if err != nil {
		return fmt.Errorf("failed to export OSCAL: %w", err)
	}
	fmt.Printf("OSCAL report written: %s\n\n", oscalPath)
	return nil
}

// runExport orchestrates evidence export to the configured Beacon collector.
// Called when COMPLYTIME_EXPORT_ENABLED is set to a truthy value, after the scan phase completes.
func (o *scanOptions) runExport(ctx context.Context, cfg *complytime.WorkspaceConfig, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup) error {
	if cfg.Collector == nil || cfg.Collector.Endpoint == "" {
		return fmt.Errorf("export requires a collector section in complytime.yaml (see docs for configuration)")
	}
	collector := cfg.Collector

	authToken, err := resolveCollectorAuth(ctx, collector.Auth)
	if err != nil {
		return err
	}

	exportReq := &provider.ExportRequest{
		Collector: provider.CollectorConfig{
			Endpoint:  collector.Endpoint,
			AuthToken: authToken,
		},
	}

	results := exportToProviders(ctx, mgr, groups, exportReq)
	fmt.Println(formatExportSummary(results))
	if failed := countExportFailures(results); failed > 0 {
		return fmt.Errorf("export failed for %d provider(s)", failed)
	}
	return nil
}

func authRequired(auth *complytime.AuthConfig) bool {
	return auth != nil && auth.TokenEndpoint != ""
}

func validateAuthCredentials(auth *complytime.AuthConfig) error {
	if auth.ClientID == "" || auth.ClientSecret == "" {
		return fmt.Errorf("collector auth requires client-id and client-secret when token-endpoint is set")
	}
	return nil
}

func resolveCollectorAuth(ctx context.Context, auth *complytime.AuthConfig) (string, error) {
	if !authRequired(auth) {
		return "", nil
	}
	if err := validateAuthCredentials(auth); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "Resolving OIDC token from %s\n", auth.TokenEndpoint)
	token, err := resolveOIDCToken(ctx, auth)
	if err != nil {
		return "", fmt.Errorf("OIDC token exchange failed: %w", err)
	}
	return token, nil
}

func exportToProviders(ctx context.Context, mgr *provider.Manager, groups map[string]policy.EvaluatorGroup, req *provider.ExportRequest) []exportResult {
	var results []exportResult
	for evalID := range groups {
		results = append(results, exportSingleProvider(ctx, mgr, evalID, req))
	}
	return results
}

func exportSingleProvider(ctx context.Context, mgr *provider.Manager, evalID string, req *provider.ExportRequest) exportResult {
	p, err := mgr.GetProvider(evalID)
	if err != nil {
		return exportResult{providerID: evalID, evalID: evalID, err: err}
	}
	if !p.SupportsExport {
		return exportResult{providerID: p.Info.ProviderID, evalID: evalID, skipped: true}
	}
	resp, exportErr := mgr.RouteExport(ctx, evalID, req)
	return exportResult{
		providerID: p.Info.ProviderID,
		evalID:     evalID,
		response:   resp,
		err:        exportErr,
	}
}

func resolveOIDCToken(ctx context.Context, auth *complytime.AuthConfig) (string, error) {
	cfg := clientcredentials.Config{
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
		TokenURL:     auth.TokenEndpoint,
	}
	token, err := cfg.Token(ctx)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func formatExportSummary(results []exportResult) string {
	var sb strings.Builder
	sb.WriteString("\nExport Summary\n")
	fmt.Fprintf(&sb, "%-20s %-10s %-10s %s\n", "PROVIDER", "EXPORTED", "FAILED", "STATUS")

	var errorMessages []string
	for _, r := range results {
		errorMessages = appendExportRow(&sb, r, errorMessages)
	}

	if len(errorMessages) > 0 {
		sb.WriteString("\n")
		for _, msg := range errorMessages {
			sb.WriteString(msg + "\n")
		}
	}

	return sb.String()
}

func appendExportRow(sb *strings.Builder, r exportResult, errors []string) []string {
	if r.skipped {
		fmt.Fprintf(sb, "%-20s %-10s %-10s %s (no export support)\n",
			r.providerID, "-", "-", complytime.StatusSkipped)
		return errors
	}
	if r.err != nil {
		fmt.Fprintf(sb, "%-20s %-10s %-10s %s\n",
			r.providerID, "-", "-", complytime.StatusError)
		return append(errors, fmt.Sprintf("%s: %v", r.providerID, r.err))
	}
	return appendResponseRow(sb, r, errors)
}

func appendResponseRow(sb *strings.Builder, r exportResult, errors []string) []string {
	if r.response == nil {
		return errors
	}
	status, errMsg := exportResponseStatus(r)
	fmt.Fprintf(sb, "%-20s %-10d %-10d %s\n",
		r.providerID, r.response.ExportedCount, r.response.FailedCount, status)
	if errMsg != "" {
		errors = append(errors, errMsg)
	}
	return errors
}

func exportResponseStatus(r exportResult) (string, string) {
	if r.response.FailedCount > 0 || !r.response.Success {
		var errMsg string
		if r.response.ErrorMessage != "" {
			errMsg = fmt.Sprintf("%s: %s", r.providerID, r.response.ErrorMessage)
		}
		return complytime.StatusFailed, errMsg
	}
	return complytime.StatusPassed, ""
}

type exportResult struct {
	providerID string
	evalID     string
	response   *provider.ExportResponse
	skipped    bool
	err        error
}

// countExportFailures returns the number of export results that represent
// a failure: a transport error or a response where Success is false or
// FailedCount is non-zero. Skipped providers are not counted as failures.
func countExportFailures(results []exportResult) int {
	count := 0
	for _, r := range results {
		if r.skipped {
			continue
		}
		if r.err != nil {
			count++
			continue
		}
		if r.response != nil && (!r.response.Success || r.response.FailedCount > 0) {
			count++
		}
	}
	return count
}

// injectWorkspaceIntoTargets returns a copy of targets with the resolved
// workspace directory injected into each target's Variables map. This ensures
// providers receive the absolute workspace path during scan without mutating
// the original config.
func injectWorkspaceIntoTargets(targets []complytime.TargetConfig, baseDir string) []complytime.TargetConfig {
	result := make([]complytime.TargetConfig, len(targets))
	for i, t := range targets {
		result[i] = t
		result[i].Variables = complytime.WithWorkspaceVar(t.Variables, baseDir)
	}
	return result
}

// extractReqToControlMap builds a requirement-ID → control-ID mapping
// from the parsed control catalogs in the dependency graph.
func extractReqToControlMap(graph *policy.DependencyGraph) map[string]string {
	m := make(map[string]string)
	if graph == nil {
		return m
	}
	for _, ctrl := range graph.Controls {
		if ctrl.Parsed == nil {
			continue
		}
		for _, c := range ctrl.Parsed.Controls {
			for _, ar := range c.AssessmentRequirements {
				m[ar.Id] = c.Id
			}
		}
	}
	return m
}

// extractPlanToReqMap builds a plan-ID → requirement-ID mapping from the
// assessment plans in the dependency graph. Providers return plan IDs in
// their results; this map resolves them to actual requirement IDs.
func extractPlanToReqMap(graph *policy.DependencyGraph) map[string]string {
	m := make(map[string]string)
	if graph == nil {
		return m
	}
	for _, a := range graph.Assessments {
		if a.RequirementID != "" {
			m[a.ID] = a.RequirementID
		}
	}
	return m
}

// resolveAssessmentIDs replaces plan IDs in assessment results with actual
// requirement IDs using the plan-to-requirement mapping from the policy graph.
func resolveAssessmentIDs(assessments []provider.AssessmentLog, planToReq map[string]string) {
	for i := range assessments {
		if reqID, ok := planToReq[assessments[i].RequirementID]; ok {
			assessments[i].RequirementID = reqID
		}
	}
}

// reverseMap inverts a string-to-string map. If multiple keys map to the same
// value, only one mapping is preserved (last-write-wins) and a warning is logged.
func reverseMap(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k, v := range m {
		if existing, ok := r[v]; ok {
			logger.Warn("multiple keys map to same value in reverse map",
				"value", v, "kept", k, "dropped", existing)
		}
		r[v] = k
	}
	return r
}

// buildReqToComplypackRef produces a pre-resolved requirement-ID →
// OCI reference (repository@digest) map by composing two lookups:
//
//  1. state.Complypacks: evaluator-ID → repository@digest
//  2. evaluator groups: requirement-ID → evaluator-ID
//
// Resolving the chain here keeps the Evaluator free of evaluator-ID
// routing concerns and makes it easier to change selection logic later
// (e.g., per-requirement complypack selection).
func buildReqToComplypackRef(cacheDir string, groups map[string]policy.EvaluatorGroup) map[string]string {
	m := make(map[string]string)
	if len(groups) == 0 {
		return m
	}
	state, err := cache.LoadState(cacheDir)
	if err != nil {
		logger.Debug("failed to load cache state for complypack ref resolution", "error", err)
		return m
	}

	evalToRef := make(map[string]string)
	for repo, ps := range state.Complypacks {
		if ps.Digest == "" || ps.EvaluatorID == "" {
			continue
		}
		if existing, ok := evalToRef[ps.EvaluatorID]; ok {
			logger.Warn("multiple complypacks cached for same evaluator",
				"evaluator", ps.EvaluatorID, "kept", existing, "dropped", repo+"@"+ps.Digest)
		}
		evalToRef[ps.EvaluatorID] = repo + "@" + ps.Digest
	}
	if len(evalToRef) == 0 {
		return m
	}

	for evalID, group := range groups {
		ref, ok := evalToRef[evalID]
		if !ok {
			continue
		}
		for _, cfg := range group.Configs {
			m[cfg.RequirementID] = ref
		}
	}
	return m
}
