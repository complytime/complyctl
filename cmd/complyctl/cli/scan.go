// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/plugin"
)

type scanOptions struct {
	*Common
	policyID  string
	format    string
	timeout   time.Duration
	cacheDir  string
	pluginDir string
}

func scanCmd(common *Common) *cobra.Command {
	o := &scanOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:   "scan [flags]",
		Short: "Scan targets and produce compliance reports",
		Example: `complyctl scan --policy-id nist-800-53-r5
  complyctl scan --policy-id nist-800-53-r5 --format pretty
  complyctl scan --policy-id nist-800-53-r5 --format oscal
  complyctl scan --policy-id nist-800-53-r5 --format sarif`,
		SilenceUsage:      true,
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
	cmd.Flags().StringVarP(&o.policyID, "policy-id", "p", "", "Policy ID to scan (see complyctl list)")
	cmd.Flags().StringVarP(&o.format, "format", "f", "", "Output format: oscal, pretty, sarif")
	cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", complytime.DefaultCommandTimeout, "Maximum time for the scan operation (e.g. 5m, 10m, 1h)")
	if err := cmd.MarkFlagRequired("policy-id"); err != nil {
		logger.Error("Failed to mark policy-id as required", "error", err)
	}
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
	o.pluginDir, err = complytime.ResolvePluginDir()
	if err != nil {
		return fmt.Errorf("failed to resolve plugin directory: %w", err)
	}
	return nil
}

func (o *scanOptions) run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	ws := complytime.NewWorkspace()
	if err := ws.LoadAndValidate(); err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	cfg := ws.Config()

	targets := cfg.Targets
	if len(targets) == 0 {
		return fmt.Errorf("no targets in complytime.yaml (add targets with policies)")
	}

	cacheMgr := cache.NewCache(o.cacheDir)
	loader := policy.NewLoader(cacheMgr)
	resolver := policy.NewResolver(loader)

	entry, found := complytime.FindPolicy(cfg.Policies, o.policyID)
	if !found {
		return fmt.Errorf("policy %q not found in config — run complyctl list to see available policy IDs", o.policyID)
	}
	ref := complytime.ParsePolicyRef(entry.URL)
	eid := entry.EffectiveID()

	version, err := loader.ResolveVersion(ref.Repository, ref.Version)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Resolved %s version: %s\n", ref.Repository, version)

	graph, err := resolver.ResolvePolicyGraph(ref.Repository, version)
	if err != nil {
		return fmt.Errorf("failed to resolve policy graph: %w", err)
	}

	assessmentConfigs := policy.ExtractAssessmentConfigs(ref.Repository, graph)
	groups := policy.GroupByEvaluator(assessmentConfigs, graph)
	globalVars := cfg.Variables

	mgr, err := plugin.NewManager(o.pluginDir, logger)
	if err != nil {
		return fmt.Errorf("plugin manager init failed: %w", err)
	}
	defer mgr.Cleanup()

	if err := mgr.LoadPlugins(); err != nil {
		return fmt.Errorf("plugin discovery failed: %w", err)
	}

	plugins := mgr.ListPlugins()
	if len(plugins) == 0 {
		return fmt.Errorf("no plugins found in %s (Describe may have failed)", o.pluginDir)
	}

	// Freshness check: skip Generate RPC when artifacts are current.
	// See R37: specs/001-gemara-native-workflow/research.md
	needsGenerate := false
	cacheState, err := cache.LoadState(o.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to load cache state: %w", err)
	}
	policyState, _ := cacheState.GetPolicyState(ref.Repository)

	genState, err := policy.LoadGenerationState(".", ref.Repository)
	if err != nil {
		return fmt.Errorf("failed to load generation state: %w", err)
	}

	switch {
	case genState == nil:
		fmt.Fprintf(os.Stderr, "No prior generation found — generating artifacts for %s\n", eid)
		needsGenerate = true
	case !genState.IsFresh(policyState.Digest):
		fmt.Fprintf(os.Stderr, "Policy %s updated since last generate — regenerating\n", eid)
		needsGenerate = true
	default:
		fmt.Fprintf(os.Stderr, "Reusing generated artifacts for %s (policy unchanged)\n", eid)
	}

	var evaluatorIDs []string
	for evalID := range groups {
		evaluatorIDs = append(evaluatorIDs, evalID)
	}

	var policyTargets []complytime.TargetConfig
	for _, t := range targets {
		if slices.Contains(t.Policies, eid) {
			policyTargets = append(policyTargets, t)
		}
	}

	if needsGenerate {
		genSpin := terminal.NewSpinner("Generating policy artifacts...")
		genSpin.Start()

		for evalID, group := range groups {
			for _, target := range policyTargets {
				if err := mgr.RouteGenerate(ctx, evalID, globalVars, target.Variables, group.Configs); err != nil {
					genSpin.Stop()
					return err
				}
			}
		}

		genSpin.Stop()

		newGenState := policy.NewGenerationState(ref.Repository, policyState.Digest, evaluatorIDs)
		if err := policy.SaveGenerationState(".", ref.Repository, newGenState); err != nil {
			return fmt.Errorf("failed to save generation state: %w", err)
		}
	}

	// Pre-scan summary (FR-034)
	var targetIDs []string
	for _, t := range targets {
		if slices.Contains(t.Policies, eid) {
			targetIDs = append(targetIDs, t.ID)
		}
	}
	fmt.Println(output.FormatPreScanSummary(len(assessmentConfigs), evaluatorIDs, targetIDs))

	reqToControl := extractReqToControlMap(graph)
	eval := output.NewEvaluator(ref.Repository, reqToControl)
	outDir := filepath.Join(".", complytime.WorkspaceDir, complytime.ScanOutputDir)
	reportDir := "."

	scanSpin := terminal.NewSpinner("Scanning targets...")
	scanSpin.Start()

	var allAssessments []plugin.AssessmentLog
	var assessmentTargets []string

	for _, target := range targets {
		if !slices.Contains(target.Policies, eid) {
			continue
		}

		pluginTargets := []plugin.Target{{
			TargetID:  target.ID,
			Variables: target.Variables,
		}}

		var assessments []plugin.AssessmentLog

		for evalID := range groups {
			results, routeErr := mgr.RouteScan(ctx, evalID, pluginTargets)
			if routeErr != nil {
				scanSpin.Stop()
				return routeErr
			}
			assessments = append(assessments, results...)
		}

		eval.AddTarget(assessments)
		allAssessments = append(allAssessments, assessments...)
		for range assessments {
			assessmentTargets = append(assessmentTargets, target.ID)
		}
	}

	scanSpin.Stop()

	logPath, err := eval.Write(outDir)
	if err != nil {
		return fmt.Errorf("failed to write evaluation log: %w", err)
	}
	fmt.Printf("Evaluation log written: %s\n", logPath)

	gemaraLog := eval.GemaraLog()

	switch o.format {
	case complytime.OutputFormatPretty:
		md := output.NewMarkdown(ref.Repository, gemaraLog)
		md.SetEmbedEvaluationLog(logPath)
		mdPath, err := md.Write(reportDir)
		if err != nil {
			return fmt.Errorf("failed to write markdown report: %w", err)
		}
		fmt.Printf("Markdown report written: %s\n\n", mdPath)
	case complytime.OutputFormatSARIF:
		sarifPath, err := output.ToSARIF(gemaraLog, "file:///scan", reportDir)
		if err != nil {
			return fmt.Errorf("failed to export SARIF: %w", err)
		}
		fmt.Printf("SARIF report written: %s\n\n", sarifPath)
	case complytime.OutputFormatOSCAL:
		oscalPath, err := output.ToOSCAL(gemaraLog, reportDir)
		if err != nil {
			return fmt.Errorf("failed to export OSCAL: %w", err)
		}
		fmt.Printf("OSCAL report written: %s\n\n", oscalPath)
	}

	fmt.Println(output.FormatScanSummary(allAssessments, assessmentTargets, reqToControl, eid, targetIDs))
	return nil
}

// extractReqToControlMap builds a requirement-ID → control-ID mapping
// from the parsed control catalogs in the dependency graph.
func extractReqToControlMap(graph *policy.DependencyGraph) map[string]string {
	m := make(map[string]string)
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
