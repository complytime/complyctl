// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/pkg/provider"
)

// CheckStatus is the result state of a single diagnostic check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusFail CheckStatus = "fail"
	StatusWarn CheckStatus = "warn"
)

// CheckResult holds the outcome of a single diagnostic check.
type CheckResult struct {
	Name     string
	Status   CheckStatus
	Message  string
	Blocking bool
}

// ProviderHealth holds Describe-declared variable requirements for a
// single scanning provider, collected during provider discovery (R51).
type ProviderHealth struct {
	EvaluatorID             string
	RequiredGlobalVariables []string
	RequiredTargetVariables []string
}

// PolicyGraphResolver resolves a policy's dependency graph from cached content.
// Satisfied by *policy.Resolver — defined as interface for testability (Constitution II).
type PolicyGraphResolver interface {
	ResolveVersion(policyID, configVersion string) (string, error)
	ResolvePolicyGraph(policyID, version string) (*policy.DependencyGraph, error)
}

// VersionResolver queries an OCI registry for the latest version of a policy.
// Satisfied by the adapter in cmd/complyctl/cli/doctor.go — defined as
// interface for testability (Constitution II).
// See R55: specs/001-gemara-native-workflow/spec.md
type VersionResolver interface {
	ResolveLatestVersion(registry, repository string) (version string, err error)
}

const registryTimeout = 5 * time.Second

// Run orchestrates all diagnostic checks and returns a slice of results.
// The resolver parameter enables policy → evaluator → target mapping for
// variable validation (R51, R52). Pass nil if the policy cache is not
// available — CheckCache will report the failure. providerLogger is the
// hclog.Logger used for provider manager and go-plugin client logging.
// When verbose is true, CheckVariables expands per-provider variable detail
// to show individual key status (R55).
// cacheBaseDir is the root cache directory (~/.complytime) where state.json
// resides. policiesCacheDir is the policies subdirectory used by CheckCache.
// See FR-039, R44, R51, R52, R55: specs/001-gemara-native-workflow/spec.md
func Run(cfg *complytime.WorkspaceConfig, configPath, providerDir, cacheDir string, resolver PolicyGraphResolver, versionResolver VersionResolver, verbose bool, providerLogger hclog.Logger) []CheckResult {
	policiesCacheDir := filepath.Join(cacheDir, complytime.PoliciesSubdir)
	var results []CheckResult
	results = append(results, CheckConfig(configPath))
	providerResults, healthData := CheckProviders(providerDir, providerLogger)
	results = append(results, providerResults...)
	results = append(results, CheckCache(policiesCacheDir))
	results = append(results, CheckPolicyVersions(cfg, cacheDir, versionResolver)...)
	results = append(results, CheckPolicyActivePeriod(cfg, resolver, verbose)...)
	results = append(results, CheckVariables(cfg, healthData, resolver, verbose)...)
	results = append(results, CheckCollector(cfg)...)
	return results
}

// CheckConfig validates that the workspace config file exists, is parseable,
// and passes structural validation including target-policy cross-references.
func CheckConfig(configPath string) CheckResult {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return CheckResult{
			Name:     "config",
			Status:   StatusFail,
			Message:  fmt.Sprintf("%s not found", configPath),
			Blocking: true,
		}
	}

	cfg, err := complytime.LoadFrom(configPath)
	if err != nil {
		return CheckResult{
			Name:     "config",
			Status:   StatusFail,
			Message:  fmt.Sprintf("config load failed: %v", err),
			Blocking: true,
		}
	}

	if err := complytime.Validate(cfg); err != nil {
		return CheckResult{
			Name:     "config",
			Status:   StatusFail,
			Message:  fmt.Sprintf("config validation failed: %v", err),
			Blocking: true,
		}
	}

	return CheckResult{
		Name:     "config",
		Status:   StatusPass,
		Message:  fmt.Sprintf("%s valid", configPath),
		Blocking: true,
	}
}

// CheckProviders discovers providers and runs Describe on each.
// Returns both diagnostic results and Describe data for variable validation (R51).
// providerLogger is passed to the provider Manager for go-plugin client logging.
func CheckProviders(providerDir string, providerLogger hclog.Logger) ([]CheckResult, []ProviderHealth) {
	if _, err := os.Stat(providerDir); os.IsNotExist(err) {
		return []CheckResult{{
			Name:     "providers",
			Status:   StatusFail,
			Message:  fmt.Sprintf("provider directory %s not found", providerDir),
			Blocking: true,
		}}, nil
	}

	mgr, err := provider.NewManager(providerDir, providerLogger)
	if err != nil {
		return []CheckResult{{
			Name:     "providers",
			Status:   StatusFail,
			Message:  fmt.Sprintf("provider manager init failed: %v", err),
			Blocking: true,
		}}, nil
	}
	defer mgr.Cleanup()

	if err := mgr.LoadProviders(); err != nil {
		return []CheckResult{{
			Name:     "providers",
			Status:   StatusFail,
			Message:  fmt.Sprintf("provider discovery failed: %v", err),
			Blocking: true,
		}}, nil
	}

	providers := mgr.ListProviders()
	if len(providers) == 0 {
		return []CheckResult{{
			Name:     "providers",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("no providers found in %s", providerDir),
			Blocking: false,
		}}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), registryTimeout)
	defer cancel()

	var results []CheckResult
	var healthData []ProviderHealth
	for _, lp := range providers {
		resp, descErr := lp.Client.Describe(ctx, &provider.DescribeRequest{})
		if descErr != nil {
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("provider/%s", lp.Info.EvaluatorID),
				Status:   StatusFail,
				Message:  fmt.Sprintf("Describe failed: %v", descErr),
				Blocking: true,
			})
			continue
		}
		if !resp.Healthy {
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("provider/%s", lp.Info.EvaluatorID),
				Status:   StatusFail,
				Message:  fmt.Sprintf("unhealthy: %s", resp.ErrorMessage),
				Blocking: true,
			})
			continue
		}
		results = append(results, CheckResult{
			Name:     fmt.Sprintf("provider/%s", lp.Info.EvaluatorID),
			Status:   StatusPass,
			Message:  fmt.Sprintf("healthy (v%s)", resp.Version),
			Blocking: true,
		})
		healthData = append(healthData, ProviderHealth{
			EvaluatorID:             lp.Info.EvaluatorID,
			RequiredGlobalVariables: resp.RequiredGlobalVariables,
			RequiredTargetVariables: resp.RequiredTargetVariables,
		})
	}
	return results, healthData
}

// CheckPolicyVersions compares cached policy versions against the latest
// available remotely. Per-policy pass/warn results. Non-blocking warning
// per unreachable registry — policies from that registry get no staleness
// line. Supersedes CheckRegistries (R55).
// See FR-039, R55: specs/001-gemara-native-workflow/spec.md
func CheckPolicyVersions(cfg *complytime.WorkspaceConfig, cacheDir string, versionResolver VersionResolver) []CheckResult {
	if cfg == nil || len(cfg.Policies) == 0 {
		return nil
	}

	if versionResolver == nil {
		return nil
	}

	state, err := cache.LoadState(cacheDir)
	if err != nil {
		return []CheckResult{{
			Name:     "policy",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("cannot load cache state for version comparison: %v", err),
			Blocking: false,
		}}
	}

	unreachable := make(map[string]bool)
	var results []CheckResult

	for _, p := range cfg.Policies {
		ref := complytime.ParsePolicyRef(p.URL)
		eid := p.EffectiveID()

		if unreachable[ref.Registry] {
			continue
		}

		cachedState, exists := state.GetPolicyState(ref.Repository)
		if !exists {
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("policy/%s", eid),
				Status:   StatusWarn,
				Message:  "not cached — run complyctl get first",
				Blocking: false,
			})
			continue
		}

		latestVersion, err := versionResolver.ResolveLatestVersion(ref.Registry, ref.Repository)
		if err != nil {
			unreachable[ref.Registry] = true
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("registry/%s", ref.Registry),
				Status:   StatusWarn,
				Message:  "unreachable — version check skipped",
				Blocking: false,
			})
			continue
		}

		cachedVersion := cachedState.Version
		if cachedVersion == latestVersion {
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("policy/%s", eid),
				Status:   StatusPass,
				Message:  fmt.Sprintf("%s (latest)", cachedVersion),
				Blocking: false,
			})
		} else {
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("policy/%s", eid),
				Status:   StatusWarn,
				Message:  fmt.Sprintf("cached %s, available %s — run complyctl get to update", cachedVersion, latestVersion),
				Blocking: false,
			})
		}
	}

	return results
}

// CheckCache verifies the policy cache directory exists (R52).
// Doctor requires cached policies to resolve provider-to-target mapping
// for target variable validation.
func CheckCache(cacheDir string) CheckResult {
	policiesDir := cacheDir
	if policiesDir == "" {
		return CheckResult{
			Name:     "cache",
			Status:   StatusFail,
			Message:  "policy cache path not resolved",
			Blocking: true,
		}
	}

	if _, err := os.Stat(policiesDir); os.IsNotExist(err) {
		return CheckResult{
			Name:     "cache",
			Status:   StatusFail,
			Message:  "policy cache not found — run complyctl get first",
			Blocking: true,
		}
	}

	entries, err := os.ReadDir(policiesDir)
	if err != nil {
		return CheckResult{
			Name:     "cache",
			Status:   StatusFail,
			Message:  fmt.Sprintf("cannot read cache directory: %v", err),
			Blocking: true,
		}
	}

	if len(entries) == 0 {
		return CheckResult{
			Name:     "cache",
			Status:   StatusFail,
			Message:  "policy cache is empty — run complyctl get first",
			Blocking: true,
		}
	}

	return CheckResult{
		Name:     "cache",
		Status:   StatusPass,
		Message:  fmt.Sprintf("%d cached policy store(s)", len(entries)),
		Blocking: true,
	}
}

// CheckVariables validates Describe-declared required variables against
// workspace config. Global variables are checked against config.variables;
// target variables are checked against relevant config.targets[].variables
// using policy → evaluator → target mapping (R51, R52).
//
// Default mode: per-provider summary line with resolved/missing counts.
// Verbose mode: appends per-key status lines below each provider (R55).
// See FR-039, R51, R55: specs/001-gemara-native-workflow/spec.md
func CheckVariables(cfg *complytime.WorkspaceConfig, healthData []ProviderHealth, resolver PolicyGraphResolver, verbose bool) []CheckResult {
	if len(healthData) == 0 {
		return nil
	}

	if cfg == nil {
		return []CheckResult{{
			Name:     "variables",
			Status:   StatusFail,
			Message:  "cannot validate variables — config not loaded",
			Blocking: true,
		}}
	}

	evaluatorTargets := make(map[string][]complytime.TargetConfig)
	resolveFailures := 0
	if resolver != nil {
		for _, target := range cfg.Targets {
			for _, pid := range target.Policies {
				entry, found := complytime.FindPolicy(cfg.Policies, pid)
				if !found {
					resolveFailures++
					continue
				}
				ref := complytime.ParsePolicyRef(entry.URL)
				version, err := resolver.ResolveVersion(ref.Repository, ref.Version)
				if err != nil {
					resolveFailures++
					continue
				}
				graph, err := resolver.ResolvePolicyGraph(ref.Repository, version)
				if err != nil {
					resolveFailures++
					continue
				}
				configs := policy.ExtractAssessmentConfigs(ref.Repository, graph)
				groups := policy.GroupByEvaluator(configs, graph)
				for evalID := range groups {
					evaluatorTargets[evalID] = append(evaluatorTargets[evalID], target)
				}
			}
		}
	}

	var results []CheckResult

	for _, ph := range healthData {
		globalResolved, globalTotal := countResolved(ph.RequiredGlobalVariables, cfg.Variables)
		var missingGlobals []string
		for _, v := range ph.RequiredGlobalVariables {
			if _, ok := cfg.Variables[v]; !ok {
				missingGlobals = append(missingGlobals, v)
			}
		}

		targets := evaluatorTargets[ph.EvaluatorID]
		unmappedTargetVars := len(ph.RequiredTargetVariables) > 0 && len(targets) == 0

		targetTotal := 0
		targetResolved := 0
		var missingTargetVars []string
		for _, target := range targets {
			for _, reqVar := range ph.RequiredTargetVariables {
				targetTotal++
				if _, ok := target.Variables[reqVar]; ok {
					targetResolved++
				} else {
					missingTargetVars = append(missingTargetVars,
						fmt.Sprintf("%s for target %q", reqVar, target.ID))
				}
			}
		}

		allGlobalPresent := globalResolved == globalTotal
		allTargetPresent := targetResolved == targetTotal
		if unmappedTargetVars && (resolver == nil || resolveFailures > 0) {
			allTargetPresent = false
		}
		name := fmt.Sprintf("variables/%s", ph.EvaluatorID)

		if allGlobalPresent && allTargetPresent {
			var msg string
			if unmappedTargetVars {
				msg = fmt.Sprintf("%d/%d global vars, no target mapping for this evaluator",
					globalResolved, globalTotal)
			} else {
				msg = fmt.Sprintf("%d/%d global vars, %d/%d target vars",
					globalResolved, globalTotal, targetResolved, targetTotal)
			}
			results = append(results, CheckResult{
				Name: name, Status: StatusPass, Message: msg, Blocking: true,
			})
		} else {
			var globalPart, targetPart string
			if allGlobalPresent {
				globalPart = fmt.Sprintf("%d/%d global vars", globalResolved, globalTotal)
			} else {
				globalPart = fmt.Sprintf("%d/%d global vars — missing %s",
					globalResolved, globalTotal, joinNames(missingGlobals))
			}
			if unmappedTargetVars {
				targetPart = fmt.Sprintf("target vars not validated — %s",
					unmappedReason(resolver, resolveFailures))
			} else if allTargetPresent {
				targetPart = fmt.Sprintf("%d/%d target vars", targetResolved, targetTotal)
			} else {
				targetPart = fmt.Sprintf("%d/%d target vars — missing %s",
					targetResolved, targetTotal, joinNames(missingTargetVars))
			}
			results = append(results, CheckResult{
				Name: name, Status: StatusFail,
				Message:  globalPart + ", " + targetPart,
				Blocking: true,
			})
		}

		if verbose {
			for _, v := range ph.RequiredGlobalVariables {
				status := complytime.StatusPassed
				if _, ok := cfg.Variables[v]; !ok {
					status = complytime.StatusFailed
				}
				results = append(results, CheckResult{
					Name:     fmt.Sprintf("variables/%s/detail", ph.EvaluatorID),
					Status:   StatusPass,
					Message:  fmt.Sprintf("   global: %s %s", v, status),
					Blocking: false,
				})
			}
			if unmappedTargetVars {
				for _, reqVar := range ph.RequiredTargetVariables {
					results = append(results, CheckResult{
						Name:     fmt.Sprintf("variables/%s/detail", ph.EvaluatorID),
						Status:   StatusPass,
						Message:  fmt.Sprintf("   target: %s (not validated)", reqVar),
						Blocking: false,
					})
				}
			} else {
				for _, target := range targets {
					for _, reqVar := range ph.RequiredTargetVariables {
						status := complytime.StatusPassed
						if _, ok := target.Variables[reqVar]; !ok {
							status = complytime.StatusFailed
						}
						results = append(results, CheckResult{
							Name:     fmt.Sprintf("variables/%s/detail", ph.EvaluatorID),
							Status:   StatusPass,
							Message:  fmt.Sprintf("   target[%s]: %s %s", target.ID, reqVar, status),
							Blocking: false,
						})
					}
				}
			}
		}
	}

	return results
}

// CheckPolicyActivePeriod resolves each policy's implementation-plan from the
// cached dependency graph and reports whether the evaluation timeline is
// currently active. Non-blocking: a policy outside its active period is a
// concern but does not prevent other checks from running.
// When verbose is true, enforcement timeline detail is appended.
// See specs/001-gemara-native-workflow/spec.md
func CheckPolicyActivePeriod(cfg *complytime.WorkspaceConfig, resolver PolicyGraphResolver, verbose bool) []CheckResult {
	if cfg == nil || len(cfg.Policies) == 0 || resolver == nil {
		return nil
	}

	now := time.Now()
	var results []CheckResult

	for _, p := range cfg.Policies {
		ref := complytime.ParsePolicyRef(p.URL)
		eid := p.EffectiveID()

		version, err := resolver.ResolveVersion(ref.Repository, ref.Version)
		if err != nil {
			continue
		}
		graph, err := resolver.ResolvePolicyGraph(ref.Repository, version)
		if err != nil {
			continue
		}

		name := fmt.Sprintf("policy/%s/active-period", eid)

		if graph.Timeline == nil {
			results = append(results, CheckResult{
				Name: name, Status: StatusPass,
				Message: "no evaluation timeline defined", Blocking: false,
			})
			continue
		}

		tl := graph.Timeline
		evalStatus, evalMsg := evaluateTimeline(
			tl.EvaluationStart, tl.EvaluationEnd, "evaluation", now)

		results = append(results, CheckResult{
			Name: name, Status: evalStatus, Message: evalMsg, Blocking: false,
		})

		if verbose {
			if tl.EvaluationNotes != "" {
				results = append(results, CheckResult{
					Name: name + "/detail", Status: StatusPass,
					Message:  fmt.Sprintf("   evaluation notes: %s", tl.EvaluationNotes),
					Blocking: false,
				})
			}
			enfStatus, enfMsg := evaluateTimeline(
				tl.EnforcementStart, tl.EnforcementEnd, "enforcement", now)
			results = append(results, CheckResult{
				Name: name + "/detail", Status: enfStatus,
				Message:  fmt.Sprintf("   %s", enfMsg),
				Blocking: false,
			})
			if tl.EnforcementNotes != "" {
				results = append(results, CheckResult{
					Name: name + "/detail", Status: StatusPass,
					Message:  fmt.Sprintf("   enforcement notes: %s", tl.EnforcementNotes),
					Blocking: false,
				})
			}
		}
	}

	return results
}

var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parseDatetime(s string) (time.Time, error) {
	for _, layout := range datetimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", s)
}

func evaluateTimeline(startStr, endStr, label string, now time.Time) (CheckStatus, string) {
	if startStr == "" {
		return StatusPass, fmt.Sprintf("no %s timeline defined", label)
	}

	start, err := parseDatetime(startStr)
	if err != nil {
		return StatusWarn, fmt.Sprintf("%s start date unparseable: %s", label, startStr)
	}

	if now.Before(start) {
		return StatusWarn, fmt.Sprintf("%s begins %s", label, startStr)
	}

	if endStr == "" {
		return StatusPass, fmt.Sprintf("%s active since %s (open-ended)", label, startStr)
	}

	end, err := parseDatetime(endStr)
	if err != nil {
		return StatusWarn, fmt.Sprintf("%s end date unparseable: %s", label, endStr)
	}

	if now.After(end) {
		return StatusWarn, fmt.Sprintf("%s ended %s", label, endStr)
	}

	return StatusPass, fmt.Sprintf("%s active (%s to %s)", label, startStr, endStr)
}

func countResolved(required []string, vars map[string]string) (resolved, total int) {
	total = len(required)
	for _, v := range required {
		if _, ok := vars[v]; ok {
			resolved++
		}
	}
	return resolved, total
}

func unmappedReason(resolver PolicyGraphResolver, resolveFailures int) string {
	if resolver == nil {
		return "no policy resolver available"
	}
	if resolveFailures > 0 {
		return fmt.Sprintf("policy graph unresolved (%d error(s)) — run complyctl get", resolveFailures)
	}
	return "evaluator not referenced by any cached policy"
}

// CheckCollector validates collector configuration when present.
// Non-blocking — the collector is optional. When configured, checks that the
// endpoint format looks valid and auth fields are complete.
func CheckCollector(cfg *complytime.WorkspaceConfig) []CheckResult {
	if cfg.Collector == nil {
		return []CheckResult{{
			Name:    "collector",
			Status:  StatusPass,
			Message: "no collector configured (optional — needed for --format otel)",
		}}
	}
	if cfg.Collector.Endpoint == "" {
		return []CheckResult{{
			Name:    "collector",
			Status:  StatusFail,
			Message: "collector.endpoint is empty",
		}}
	}
	results := []CheckResult{{
		Name:    "collector",
		Status:  StatusPass,
		Message: fmt.Sprintf("collector endpoint: %s", cfg.Collector.Endpoint),
	}}
	if cfg.Collector.Auth != nil {
		results = append(results, checkCollectorAuth(cfg.Collector.Auth))
	}
	return results
}

func checkCollectorAuth(auth *complytime.AuthConfig) CheckResult {
	if auth.TokenEndpoint == "" {
		return CheckResult{
			Name:    "collector-auth",
			Status:  StatusWarn,
			Message: "collector.auth.token-endpoint is empty — OIDC auth will not work",
		}
	}
	if auth.ClientID == "" || auth.ClientSecret == "" {
		return CheckResult{
			Name:    "collector-auth",
			Status:  StatusWarn,
			Message: "collector.auth client-id or client-secret missing — OIDC auth will fail",
		}
	}
	return CheckResult{
		Name:    "collector-auth",
		Status:  StatusPass,
		Message: fmt.Sprintf("OIDC client credentials configured (token-endpoint: %s)", auth.TokenEndpoint),
	}
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	result := names[0]
	for _, n := range names[1:] {
		result += ", " + n
	}
	return result
}
